package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"
)

type explainRequest struct {
	Target        string `json:"target"`
	MCPURL        string `json:"mcp_url"`
	MessageID     string `json:"message_id"`
	CorrelationID string `json:"correlation_id"`
	TraceID       string `json:"trace_id"`
	Workflow      string `json:"workflow"`
	Question      string `json:"question"`
}

type explainResponse struct {
	Target         string         `json:"target"`
	MCPURL         string         `json:"mcp_url"`
	Workflow       any            `json:"workflow"`
	BusinessRules  any            `json:"business_rules"`
	Execution      any            `json:"execution,omitempty"`
	Executions     any            `json:"executions,omitempty"`
	Explanation    string         `json:"explanation"`
	ReasoningStyle string         `json:"reasoning_style"`
	Question       string         `json:"question"`
	Prompt         string         `json:"prompt"`
	Model          string         `json:"model"`
	Provider       string         `json:"provider"`
	GeneratedAt    string         `json:"generated_at"`
}

type mcpClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

type ollamaResponse struct {
	Response string `json:"response"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := &http.Server{
		Addr:              env("AGENT_HTTP_ADDR", ":8095"),
		Handler:           routes(logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()

	logger.Info("agent service listening",
		slog.String("addr", server.Addr),
		slog.String("default_target", env("AGENT_DEFAULT_TARGET", "product")),
		slog.String("provider", env("AGENT_LLM_PROVIDER", "heuristic")),
	)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func routes(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":   "ok",
			"service":  "agent-service",
			"provider": env("AGENT_LLM_PROVIDER", "heuristic"),
		})
	})
	mux.HandleFunc("/v1/targets", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"targets": targetMap()})
	})
	mux.HandleFunc("/v1/explain", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var input explainRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		response, err := explain(r.Context(), input, logger)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		writeJSON(w, http.StatusOK, response)
	})
	return mux
}

func explain(ctx context.Context, input explainRequest, logger *slog.Logger) (*explainResponse, error) {
	baseURL, target := resolveTarget(input)
	if baseURL == "" {
		return nil, fmt.Errorf("mcp target not configured")
	}
	client := &mcpClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  env("AGENT_MCP_API_KEY", ""),
		client: &http.Client{
			Timeout: parseDurationEnv("AGENT_REQUEST_TIMEOUT", 12*time.Second),
		},
	}

	workflow, err := client.callTool(ctx, "explain_workflow", map[string]any{})
	if err != nil {
		return nil, err
	}
	rules, err := client.callTool(ctx, "list_business_rules", map[string]any{})
	if err != nil {
		return nil, err
	}

	var execution any
	var executions any
	switch {
	case strings.TrimSpace(input.MessageID) != "":
		execution, err = client.callTool(ctx, "get_execution", map[string]any{"message_id": input.MessageID})
	case strings.TrimSpace(input.CorrelationID) != "":
		executions, err = client.callTool(ctx, "find_executions", map[string]any{"correlation_id": input.CorrelationID, "workflow": input.Workflow})
	case strings.TrimSpace(input.TraceID) != "":
		executions, err = client.callTool(ctx, "find_executions", map[string]any{"trace_id": input.TraceID, "workflow": input.Workflow})
	default:
		return nil, fmt.Errorf("message_id, correlation_id or trace_id is required")
	}
	if err != nil {
		return nil, err
	}

	prompt := buildPrompt(input, workflow, rules, execution, executions)
	explanation, provider, model := generateAnswer(ctx, prompt, logger)

	return &explainResponse{
		Target:         target,
		MCPURL:         baseURL,
		Workflow:       workflow,
		BusinessRules:  rules,
		Execution:      execution,
		Executions:     executions,
		Explanation:    explanation,
		ReasoningStyle: provider,
		Question:       input.Question,
		Prompt:         prompt,
		Model:          model,
		Provider:       provider,
		GeneratedAt:    time.Now().Format(time.RFC3339),
	}, nil
}

func resolveTarget(input explainRequest) (string, string) {
	if strings.TrimSpace(input.MCPURL) != "" {
		return input.MCPURL, "custom"
	}
	target := strings.TrimSpace(input.Target)
	if target == "" {
		target = env("AGENT_DEFAULT_TARGET", "product")
	}
	return targetMap()[target], target
}

func targetMap() map[string]string {
	targets := map[string]string{}
	raw := strings.TrimSpace(env("AGENT_MCP_TARGETS", "product=http://localhost:9094/mcp"))
	for _, item := range strings.Split(raw, ",") {
		parts := strings.SplitN(strings.TrimSpace(item), "=", 2)
		if len(parts) != 2 {
			continue
		}
		targets[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return targets
}

func (c *mcpClient) callTool(ctx context.Context, name string, args map[string]any) (any, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      time.Now().UnixNano(),
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": args,
		},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Result any `json:"result"`
		Error  struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	if parsed.Error.Message != "" {
		return nil, fmt.Errorf("%s", parsed.Error.Message)
	}
	return parsed.Result, nil
}

func buildPrompt(input explainRequest, workflow, rules, execution, executions any) string {
	var sections []string
	sections = append(sections,
		"Você é um agente de explicabilidade de workflows.",
		"Responda em português claro, com foco em causa, impacto e próximos passos.",
	)
	if question := strings.TrimSpace(input.Question); question != "" {
		sections = append(sections, "Pergunta do usuário:\n"+question)
	}
	sections = append(sections, "Workflow:\n"+pretty(workflow))
	sections = append(sections, "Regras de negócio:\n"+pretty(rules))
	if execution != nil {
		sections = append(sections, "Execução:\n"+pretty(execution))
	}
	if executions != nil {
		sections = append(sections, "Execuções relacionadas:\n"+pretty(executions))
	}
	return strings.Join(sections, "\n\n")
}

func generateAnswer(ctx context.Context, prompt string, logger *slog.Logger) (string, string, string) {
	provider := strings.ToLower(strings.TrimSpace(env("AGENT_LLM_PROVIDER", "heuristic")))
	if provider == "ollama" {
		answer, model, err := generateWithOllama(ctx, prompt)
		if err == nil && strings.TrimSpace(answer) != "" {
			return answer, "ollama", model
		}
		logger.Warn("ollama generation failed, falling back to heuristic", slog.String("error", err.Error()))
	}
	return heuristicSummary(prompt), "heuristic", "deterministic-explainer"
}

func generateWithOllama(ctx context.Context, prompt string) (string, string, error) {
	model := env("AGENT_OLLAMA_MODEL", "smollm2:135m")
	payload := map[string]any{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(env("AGENT_OLLAMA_URL", "http://localhost:11434"), "/")+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", model, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: parseDurationEnv("AGENT_REQUEST_TIMEOUT", 12*time.Second)}).Do(req)
	if err != nil {
		return "", model, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return "", model, fmt.Errorf("ollama returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var parsed ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", model, err
	}
	return strings.TrimSpace(parsed.Response), model, nil
}

func heuristicSummary(prompt string) string {
	lines := []string{
		"Resumo automático baseado nas informações do workflow e das execuções consultadas via MCP.",
		"",
		"O foco desta resposta é apontar:",
		"- o que foi processado;",
		"- em que estado o processo terminou;",
		"- quais etapas relevantes aparecem no histórico;",
		"- quais regras de negócio ajudam a explicar o comportamento observado.",
		"",
		"Contexto consolidado:",
		trimPrompt(prompt, 3200),
	}
	return strings.Join(lines, "\n")
}

func trimPrompt(value string, limit int) string {
	text := strings.TrimSpace(value)
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "\n...[resumido]"
}

func pretty(value any) string {
	if value == nil {
		return "null"
	}
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(body)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func parseDurationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
