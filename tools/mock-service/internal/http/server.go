package httpserver

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/raywall/workflows/sample-test/mock-service/internal/domain"
	"github.com/raywall/workflows/sample-test/mock-service/internal/store"
)

//go:embed all:web
var webFS embed.FS

type Server struct {
	repo   store.Repository
	logger *log.Logger
}

func New(repo store.Repository, logger *log.Logger) *Server {
	return &Server{repo: repo, logger: logger}
}

func (s *Server) Handler() http.Handler {
	rand.Seed(time.Now().UnixNano())

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/mocks", s.handleMocks)
	mux.HandleFunc("/api/mocks/", s.handleMockByID)
	mux.HandleFunc("/api/catalog/generators", s.handleGeneratorCatalog)

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	mux.Handle("/assets/", http.StripPrefix("/assets/", fileServer))
	mux.HandleFunc("/", s.handleIndexOrMock(fileServer))
	return loggingMiddleware(corsMiddleware(mux), s.logger)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleMocks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := s.repo.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		mock, err := decodeMock(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		saved, err := s.repo.Save(r.Context(), mock)
		if err != nil {
			writeStatusError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, saved)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleMockByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/mocks/")
	id = path.Clean("/" + id)
	id = strings.TrimPrefix(id, "/")
	if id == "" || id == "." {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := s.repo.Get(r.Context(), id)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, domain.ErrNotFound) {
				status = http.StatusNotFound
			}
			writeStatusError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodPut:
		mock, err := decodeMock(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		mock.ID = id
		saved, err := s.repo.Save(r.Context(), mock)
		if err != nil {
			writeStatusError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, saved)
	case http.MethodDelete:
		if err := s.repo.Delete(r.Context(), id); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, domain.ErrNotFound) {
				status = http.StatusNotFound
			}
			writeStatusError(w, status, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleGeneratorCatalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{
		{
			"kind":        domain.GeneratorConsignadoOperacao,
			"description": "Gera operacoes de credito consignado com parcelas coerentes, preservando cliente e contrato da requisicao.",
		},
		{
			"kind":        domain.GeneratorConsignadoSaldos,
			"description": "Gera saldos e parcelas de liquidacao antecipada com atraso, descontos e quantidade parametrizavel.",
		},
	})
}

func (s *Server) handleIndexOrMock(fileServer http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/index.html":
			fileServer.ServeHTTP(w, r)
			return
		}
		s.serveMock(w, r)
	}
}

func (s *Server) serveMock(w http.ResponseWriter, r *http.Request) {
	items, err := s.repo.List(r.Context())
	if err != nil {
		writeStatusError(w, http.StatusInternalServerError, err)
		return
	}
	for _, item := range items {
		match, err := domain.MatchMock(item, r)
		if err != nil {
			writeStatusError(w, http.StatusInternalServerError, err)
			return
		}
		if !match.Matched {
			continue
		}
		ctx := domain.RequestContext{
			Method:  r.Method,
			Path:    r.URL.Path,
			PathVar: match.PathVars,
			Query:   r.URL.Query(),
			Headers: r.Header.Clone(),
			Body:    match.Body,
		}
		status, headers, body, latency, err := domain.RenderResponse(item, ctx)
		if err != nil {
			writeStatusError(w, http.StatusInternalServerError, err)
			return
		}
		if latency > 0 {
			timer := time.NewTimer(latency)
			defer timer.Stop()
			select {
			case <-r.Context().Done():
				return
			case <-timer.C:
			}
		}
		for key, value := range headers {
			w.Header().Set(key, value)
		}
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		if body == nil {
			return
		}
		if err := json.NewEncoder(w).Encode(body); err != nil {
			s.logger.Printf("write response failed: %v", err)
		}
		return
	}
	writeStatusError(w, http.StatusNotFound, fmt.Errorf("no mock matched %s %s", r.Method, r.URL.Path))
}

func decodeMock(r *http.Request) (domain.MockDefinition, error) {
	defer r.Body.Close()
	var payload domain.MockDefinition
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return domain.MockDefinition{}, err
	}
	payload.Normalize()
	return payload, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeStatusError(w, status, err)
}

func writeStatusError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler, logger *log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		next.ServeHTTP(w, r)
		logger.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(startedAt))
	})
}

func Shutdown(ctx context.Context, server *http.Server) error {
	return server.Shutdown(ctx)
}
