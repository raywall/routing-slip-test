package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type MatchResult struct {
	Matched  bool
	PathVars map[string]string
	Body     any
	Reason   string
}

type RequestContext struct {
	Method  string
	Path    string
	PathVar map[string]string
	Query   url.Values
	Headers http.Header
	Body    any
}

func MatchMock(mock MockDefinition, req *http.Request) (MatchResult, error) {
	if !mock.Enabled {
		return MatchResult{Reason: "mock disabled"}, nil
	}
	if strings.ToUpper(req.Method) != mock.Method {
		return MatchResult{Reason: "method mismatch"}, nil
	}
	pathVars, ok := matchPath(mock.EndpointPattern, req.URL.Path)
	if !ok {
		return MatchResult{Reason: "path mismatch"}, nil
	}
	for key, expected := range mock.ExpectedHeaders {
		if req.Header.Get(key) != expected {
			return MatchResult{Reason: fmt.Sprintf("header mismatch: %s", key)}, nil
		}
	}
	for key, expected := range mock.ExpectedQuery {
		if req.URL.Query().Get(key) != expected {
			return MatchResult{Reason: fmt.Sprintf("query mismatch: %s", key)}, nil
		}
	}
	var body any
	if mock.ExpectedBody != nil {
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			return MatchResult{}, err
		}
		req.Body = io.NopCloser(bytes.NewReader(payload))
		if len(bytes.TrimSpace(payload)) == 0 {
			return MatchResult{Reason: "body mismatch: empty body"}, nil
		}
		if err := json.Unmarshal(payload, &body); err != nil {
			return MatchResult{Reason: "body mismatch: invalid json"}, nil
		}
		if !matchPartial(mock.ExpectedBody, body) {
			return MatchResult{Reason: "body mismatch"}, nil
		}
	} else if req.Body != nil {
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			return MatchResult{}, err
		}
		req.Body = io.NopCloser(bytes.NewReader(payload))
		if len(bytes.TrimSpace(payload)) > 0 {
			_ = json.Unmarshal(payload, &body)
		}
	}
	return MatchResult{
		Matched:  true,
		PathVars: pathVars,
		Body:     body,
	}, nil
}

func RenderResponse(mock MockDefinition, ctx RequestContext) (status int, headers map[string]string, body any, latency time.Duration, err error) {
	status = mock.StatusCode
	headers = cloneStringMap(mock.ResponseHeaders)
	latency = randomLatency(mock.LatencyMinMS, mock.LatencyMaxMS)
	switch mock.ResponseMode {
	case ResponseModeStatic:
		body = mock.ResponseBody
	case ResponseModeTemplate:
		body = renderTemplateValue(mock.ResponseTemplate, buildTemplateScope(ctx, nil, mock.AdditionalVariables))
	case ResponseModeGenerated:
		var generated any
		generated, err = generateBody(mock, ctx)
		if err != nil {
			return 0, nil, nil, 0, err
		}
		body = generated
	default:
		err = fmt.Errorf("%w: unsupported response mode %s", ErrInvalidConfig, mock.ResponseMode)
		return
	}
	return
}

func matchPath(pattern, actual string) (map[string]string, bool) {
	pattern = normalizePath(pattern)
	actual = normalizePath(actual)
	if pattern == actual {
		return map[string]string{}, true
	}
	patternParts := splitPath(pattern)
	actualParts := splitPath(actual)
	if len(patternParts) != len(actualParts) {
		return nil, false
	}
	out := map[string]string{}
	for i := range patternParts {
		pp := patternParts[i]
		ap := actualParts[i]
		if strings.HasPrefix(pp, "{") && strings.HasSuffix(pp, "}") {
			out[strings.TrimSuffix(strings.TrimPrefix(pp, "{"), "}")] = ap
			continue
		}
		if pp != ap {
			return nil, false
		}
	}
	return out, true
}

func splitPath(path string) []string {
	if path == "/" {
		return []string{}
	}
	return strings.Split(strings.Trim(path, "/"), "/")
}

func matchPartial(expected, actual any) bool {
	switch e := expected.(type) {
	case map[string]any:
		a, ok := actual.(map[string]any)
		if !ok {
			return false
		}
		for key, value := range e {
			if !matchPartial(value, a[key]) {
				return false
			}
		}
		return true
	case []any:
		a, ok := actual.([]any)
		if !ok || len(a) < len(e) {
			return false
		}
		for i, value := range e {
			if !matchPartial(value, a[i]) {
				return false
			}
		}
		return true
	case float64:
		return toFloat(actual) == e
	default:
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
	}
}

func cloneStringMap(input map[string]string) map[string]string {
	if input == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func randomLatency(minMS, maxMS int) time.Duration {
	switch {
	case minMS <= 0 && maxMS <= 0:
		return 0
	case maxMS <= minMS:
		return time.Duration(minMS) * time.Millisecond
	default:
		delta := maxMS - minMS
		return time.Duration(minMS+rand.Intn(delta+1)) * time.Millisecond
	}
}

func buildTemplateScope(ctx RequestContext, generated any, extra map[string]any) map[string]any {
	scope := map[string]any{
		"request": map[string]any{
			"method": ctx.Method,
			"path":   ctx.Path,
		},
		"path":      mapStringAny(ctx.PathVar),
		"query":     firstValues(ctx.Query),
		"headers":   firstValues(ctx.Headers),
		"body":      ctx.Body,
		"generated": generated,
		"extra":     extra,
	}
	return scope
}

func mapStringAny(input map[string]string) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func firstValues(values map[string][]string) map[string]any {
	out := map[string]any{}
	for key, list := range values {
		if len(list) > 0 {
			out[key] = list[0]
		}
	}
	return out
}

func renderTemplateValue(value any, scope map[string]any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, nested := range typed {
			out[key] = renderTemplateValue(nested, scope)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, nested := range typed {
			out = append(out, renderTemplateValue(nested, scope))
		}
		return out
	case string:
		return renderTemplateString(typed, scope)
	default:
		return typed
	}
}

func renderTemplateString(template string, scope map[string]any) any {
	trimmed := strings.TrimSpace(template)
	if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") && strings.Count(trimmed, "{{") == 1 && strings.Count(trimmed, "}}") == 1 {
		expr := strings.TrimSpace(trimmed[2 : len(trimmed)-2])
		if resolved, ok := resolveExpression(scope, expr); ok {
			return resolved
		}
	}
	result := template
	for {
		start := strings.Index(result, "{{")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}}")
		if end == -1 {
			break
		}
		end += start
		expr := strings.TrimSpace(result[start+2 : end])
		replacement := ""
		if resolved, ok := resolveExpression(scope, expr); ok {
			replacement = fmt.Sprintf("%v", resolved)
		}
		result = result[:start] + replacement + result[end+2:]
	}
	return result
}

func resolveExpression(scope map[string]any, expr string) (any, bool) {
	parts := strings.Split(expr, ".")
	var current any = scope
	for _, part := range parts {
		switch typed := current.(type) {
		case map[string]any:
			value, ok := typed[part]
			if !ok {
				return nil, false
			}
			current = value
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, false
			}
			current = typed[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func generateBody(mock MockDefinition, ctx RequestContext) (any, error) {
	switch mock.Generator.Kind {
	case GeneratorConsignadoOperacao:
		return generateConsignadoOperacao(mock.Generator.ConsignadoOperacao, ctx), nil
	case GeneratorConsignadoSaldos:
		return generateConsignadoSaldos(mock.Generator.ConsignadoSaldos, ctx), nil
	default:
		return nil, fmt.Errorf("%w: unsupported generator kind %q", ErrInvalidConfig, mock.Generator.Kind)
	}
}

func generateConsignadoOperacao(cfg *ConsignadoOperacaoConfig, ctx RequestContext) any {
	if cfg == nil {
		cfg = &ConsignadoOperacaoConfig{}
	}
	value := randomFloat(normalizeRange(cfg.ContractValueMin, cfg.ContractValueMax, 15000, 75000))
	count := randomInt(normalizeIntRange(cfg.InstallmentCountMin, cfg.InstallmentCountMax, 12, 72))
	instRangeMin, instRangeMax := normalizeRange(cfg.InstallmentValueMin, cfg.InstallmentValueMax, 250, 1800)
	maxOverdue := cfg.MaxOverdueOpen
	if maxOverdue < 0 {
		maxOverdue = 0
	}
	custody := pickString(cfg.CustodyOptions, "SF")
	company := pickString(cfg.CompanyCodes, "0341")
	contractID := lookupRequestValue(ctx, cfg.ContractPath, "identificadorOperacaoCredito")
	if contractID == "" {
		contractID = "2699999999"
	}
	customerID := lookupRequestValue(ctx, cfg.CustomerPath, "codigoCliente")
	if customerID == "" {
		customerID = "12345678901"
	}
	convenio := lookupRequestValue(ctx, cfg.ConvenioPath, "codigoConvenio")
	if convenio == "" {
		convenio = "133341"
	}
	paidCount := randomInt(0, max(0, count-maxOverdue-1))
	overdueCount := randomInt(0, min(maxOverdue, count-paidCount))
	now := time.Now()
	parcelas := make([]any, 0, count)
	totalPrincipal := 0.0
	for i := 1; i <= count; i++ {
		principal := randomFloat(instRangeMin, instRangeMax)
		interest := round2(principal * randomFloat(0.05, 0.22))
		original := round2(principal + interest)
		totalPrincipal += original
		status := "aberta"
		statusCode := 1
		dueDate := now.AddDate(0, 0, (i-paidCount-overdueCount)*30)
		switch {
		case i <= paidCount:
			status = "paga"
			statusCode = 2
		case i <= paidCount+overdueCount:
			status = "aberta"
			statusCode = 3
			dueDate = now.AddDate(0, 0, -(paidCount+overdueCount-i+1)*30)
		default:
			status = "aberta"
			statusCode = 1
		}
		parcelas = append(parcelas, map[string]any{
			"numeroParcela":             i,
			"situacaoParcela":           status,
			"codigoSituacaoParcela":     statusCode,
			"dataVencimento":            dueDate.Format("2006-01-02"),
			"numeroPlanoAmortizacao":    1,
			"valorParcelaPrincipal":     principal,
			"valorJuroParcelaPrincipal": interest,
			"valorParcelaOriginal":      original,
			"valorSaldoPrincipal":       round2(math.Max(original-randomFloat(0, original*0.8), 0)),
			"valorSaldoPrincipalTotal":  original,
		})
	}
	return []any{
		map[string]any{
			"operacaoId":                           contractID,
			"situacaoOperacao":                     defaultString(cfg.OperationStatus, "ATIVA"),
			"codigoSituacaoOperacao":               defaultInt(cfg.OperationStatusCode, 1),
			"siglaCustodia":                        custody,
			"codigoEmpresaContabil":                company,
			"saldoDevedor":                         round2(totalPrincipal),
			"meioRecebimentoOperacaoCredito":       defaultString(cfg.ReceivingMethod, "CONTA_CORRENTE"),
			"codigoMeioRecebimentoOperacaoCredito": defaultInt(cfg.ReceivingMethodCode, 1),
			"convenio": map[string]any{
				"codigoConvenio": convenio,
			},
			"cliente": map[string]any{
				"numeroBeneficio": customerID,
			},
			"produto": map[string]any{
				"codigoProdutoOperacional":   defaultInt(cfg.ProductOperational, 1234),
				"codigoProdutoFinanceiro":    defaultInt(cfg.ProductFinancial, 5678),
				"codigoProdutoCreditoLimite": defaultInt(cfg.ProductCreditLimit, 0),
			},
			"parcelas":      parcelas,
			"valorContrato": round2(value),
		},
	}
}

func generateConsignadoSaldos(cfg *ConsignadoSaldosConfig, ctx RequestContext) any {
	if cfg == nil {
		cfg = &ConsignadoSaldosConfig{}
	}
	count := randomInt(normalizeIntRange(cfg.InstallmentCountMin, cfg.InstallmentCountMax, 12, 72))
	operationBalance := randomFloat(normalizeRange(cfg.OperationBalanceMin, cfg.OperationBalanceMax, 12000, 55000))
	delay := randomInt(0, defaultInt(cfg.MaxDelayDays, 45))
	liqMin, liqMax := normalizeRange(cfg.InstallmentLiquidationMin, cfg.InstallmentLiquidationMax, 300, 2200)
	_ = lookupRequestValue(ctx, cfg.CustomerPath, "codigoCliente")
	_ = lookupRequestValue(ctx, cfg.ContractPath, "identificadorOperacaoCredito")
	_ = lookupRequestValue(ctx, cfg.BaseDatePath, "dataPosicaoCalculo")

	parcelas := make([]any, 0, count)
	totalLiquid := 0.0
	for i := 1; i <= count; i++ {
		liquidation := randomFloat(liqMin, liqMax)
		discount := round2(liquidation * randomFloat(0, 0.08))
		parcela := map[string]any{
			"numero_plano_amortizacao_contratacao": 1,
			"numero_parcela_amortizacao":           i,
			"valor_parcela_amortizacao":            round2(liquidation + randomFloat(10, 150)),
			"valor_liquidacao_antecipada":          liquidation,
			"valor_desconto_abatimento":            discount,
		}
		parcelas = append(parcelas, parcela)
		totalLiquid += liquidation
	}
	return []any{
		map[string]any{
			"saldo": map[string]any{
				"saldo_liquido_operacao":              round2(totalLiquid),
				"quantidade_dia_atraso":               delay,
				"saldo_devedor_operacao_sem_desconto": round2(operationBalance),
				"saldo_parcelas":                      parcelas,
			},
		},
	}
}

func lookupRequestValue(ctx RequestContext, explicitPath, fallback string) string {
	for _, candidate := range []string{explicitPath, fallback} {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if value, ok := ctx.PathVar[candidate]; ok {
			return fmt.Sprintf("%v", value)
		}
		if queryValue := ctx.Query.Get(candidate); queryValue != "" {
			return queryValue
		}
		if value, ok := resolveExpression(buildTemplateScope(ctx, nil, nil), "body."+candidate); ok {
			return fmt.Sprintf("%v", value)
		}
	}
	return ""
}

func normalizeRange(minValue, maxValue, defaultMin, defaultMax float64) (float64, float64) {
	if minValue <= 0 {
		minValue = defaultMin
	}
	if maxValue <= 0 {
		maxValue = defaultMax
	}
	if maxValue < minValue {
		maxValue = minValue
	}
	return minValue, maxValue
}

func normalizeIntRange(minValue, maxValue, defaultMin, defaultMax int) (int, int) {
	if minValue <= 0 {
		minValue = defaultMin
	}
	if maxValue <= 0 {
		maxValue = defaultMax
	}
	if maxValue < minValue {
		maxValue = minValue
	}
	return minValue, maxValue
}

func randomFloat(minValue, maxValue float64) float64 {
	if maxValue <= minValue {
		return round2(minValue)
	}
	return round2(minValue + rand.Float64()*(maxValue-minValue))
}

func randomInt(minValue, maxValue int) int {
	if maxValue <= minValue {
		return minValue
	}
	return minValue + rand.Intn(maxValue-minValue+1)
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func pickString(options []string, fallback string) string {
	if len(options) == 0 {
		return fallback
	}
	return options[rand.Intn(len(options))]
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func defaultInt(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func toFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		number, _ := typed.Float64()
		return number
	default:
		number, _ := strconv.ParseFloat(fmt.Sprintf("%v", value), 64)
		return number
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
