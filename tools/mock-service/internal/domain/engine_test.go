package domain

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMatchMockPathAndBody(t *testing.T) {
	t.Parallel()

	mock := MockDefinition{
		Enabled:         true,
		Method:          "POST",
		EndpointPattern: "/clientes/{codigoCliente}/contratos/{contratoId}",
		ExpectedBody: map[string]any{
			"data": map[string]any{
				"status": "ATIVA",
			},
		},
	}
	mock.Normalize()

	req := httptest.NewRequest("POST", "/clientes/123/contratos/999", strings.NewReader(`{"data":{"status":"ATIVA","extra":true}}`))
	match, err := MatchMock(mock, req)
	if err != nil {
		t.Fatalf("MatchMock() error = %v", err)
	}
	if !match.Matched {
		t.Fatalf("MatchMock() matched = false, want true, reason=%s", match.Reason)
	}
	if got := match.PathVars["codigoCliente"]; got != "123" {
		t.Fatalf("codigoCliente = %q, want 123", got)
	}
	if got := match.PathVars["contratoId"]; got != "999" {
		t.Fatalf("contratoId = %q, want 999", got)
	}
}

func TestRenderTemplateValuePreservesTypedPlaceholder(t *testing.T) {
	t.Parallel()

	scope := map[string]any{
		"path": map[string]any{
			"contratoId": "2699999999",
		},
		"generated": map[string]any{
			"parcelas": []any{1, 2, 3},
		},
	}
	value := renderTemplateValue(map[string]any{
		"contrato": "{{path.contratoId}}",
		"parcelas": "{{generated.parcelas}}",
	}, scope).(map[string]any)

	if value["contrato"] != "2699999999" {
		t.Fatalf("contrato = %v, want 2699999999", value["contrato"])
	}
	parcelas, ok := value["parcelas"].([]any)
	if !ok || len(parcelas) != 3 {
		t.Fatalf("parcelas = %#v, want typed array with len 3", value["parcelas"])
	}
}

func TestGenerateConsignadoOperacaoProducesConsistentInstallments(t *testing.T) {
	t.Parallel()

	body := generateConsignadoOperacao(&ConsignadoOperacaoConfig{
		CustomerPath:        "codigoCliente",
		ContractPath:        "identificadorOperacaoCredito",
		InstallmentCountMin: 6,
		InstallmentCountMax: 6,
		MaxOverdueOpen:      2,
	}, RequestContext{
		PathVar: map[string]string{},
		Body: map[string]any{
			"codigoCliente":                "12345678901",
			"identificadorOperacaoCredito": "2699999999",
		},
	}).([]any)

	operacao := body[0].(map[string]any)
	if operacao["operacaoId"] != "2699999999" {
		t.Fatalf("operacaoId = %v, want preserved contract id", operacao["operacaoId"])
	}
	parcelas := operacao["parcelas"].([]any)
	foundOpen := false
	for index, raw := range parcelas {
		parcela := raw.(map[string]any)
		status := parcela["situacaoParcela"].(string)
		if status != "paga" {
			foundOpen = true
		}
		if foundOpen && status == "paga" {
			t.Fatalf("installment %d is paid after an open installment, violating ordering", index+1)
		}
	}
}
