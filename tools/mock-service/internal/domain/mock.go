package domain

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const (
	ResponseModeStatic    = "static"
	ResponseModeTemplate  = "template"
	ResponseModeGenerated = "generated"

	GeneratorConsignadoOperacao = "consignado_operacao_v1"
	GeneratorConsignadoSaldos   = "consignado_saldos_v1"
)

var (
	ErrNotFound      = errors.New("mock not found")
	ErrInvalidConfig = errors.New("invalid mock configuration")
)

type MockDefinition struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	Description         string            `json:"description,omitempty"`
	Enabled             bool              `json:"enabled"`
	Method              string            `json:"method"`
	EndpointPattern     string            `json:"endpoint_pattern"`
	StatusCode          int               `json:"status_code"`
	ExpectedHeaders     map[string]string `json:"expected_headers,omitempty"`
	ExpectedQuery       map[string]string `json:"expected_query,omitempty"`
	ExpectedBody        any               `json:"expected_body,omitempty"`
	ResponseHeaders     map[string]string `json:"response_headers,omitempty"`
	ResponseMode        string            `json:"response_mode"`
	ResponseBody        any               `json:"response_body,omitempty"`
	ResponseTemplate    any               `json:"response_template,omitempty"`
	LatencyMinMS        int               `json:"latency_min_ms,omitempty"`
	LatencyMaxMS        int               `json:"latency_max_ms,omitempty"`
	Generator           *GeneratorConfig  `json:"generator,omitempty"`
	AdditionalVariables map[string]any    `json:"additional_variables,omitempty"`
	Tags                []string          `json:"tags,omitempty"`
}

type GeneratorConfig struct {
	Kind               string                    `json:"kind"`
	ConsignadoOperacao *ConsignadoOperacaoConfig `json:"consignado_operacao,omitempty"`
	ConsignadoSaldos   *ConsignadoSaldosConfig   `json:"consignado_saldos,omitempty"`
}

type ConsignadoOperacaoConfig struct {
	CustomerPath        string   `json:"customer_path,omitempty"`
	ContractPath        string   `json:"contract_path,omitempty"`
	ConvenioPath        string   `json:"convenio_path,omitempty"`
	ContractValueMin    float64  `json:"contract_value_min"`
	ContractValueMax    float64  `json:"contract_value_max"`
	InstallmentValueMin float64  `json:"installment_value_min"`
	InstallmentValueMax float64  `json:"installment_value_max"`
	InstallmentCountMin int      `json:"installment_count_min"`
	InstallmentCountMax int      `json:"installment_count_max"`
	MaxOverdueOpen      int      `json:"max_overdue_open"`
	CustodyOptions      []string `json:"custody_options,omitempty"`
	CompanyCodes        []string `json:"company_codes,omitempty"`
	OperationStatus     string   `json:"operation_status,omitempty"`
	OperationStatusCode int      `json:"operation_status_code,omitempty"`
	ReceivingMethod     string   `json:"receiving_method,omitempty"`
	ReceivingMethodCode int      `json:"receiving_method_code,omitempty"`
	ProductOperational  int      `json:"product_operational,omitempty"`
	ProductFinancial    int      `json:"product_financial,omitempty"`
	ProductCreditLimit  int      `json:"product_credit_limit,omitempty"`
}

type ConsignadoSaldosConfig struct {
	CustomerPath              string  `json:"customer_path,omitempty"`
	ContractPath              string  `json:"contract_path,omitempty"`
	BaseDatePath              string  `json:"base_date_path,omitempty"`
	OperationBalanceMin       float64 `json:"operation_balance_min"`
	OperationBalanceMax       float64 `json:"operation_balance_max"`
	InstallmentLiquidationMin float64 `json:"installment_liquidation_min"`
	InstallmentLiquidationMax float64 `json:"installment_liquidation_max"`
	InstallmentCountMin       int     `json:"installment_count_min"`
	InstallmentCountMax       int     `json:"installment_count_max"`
	MaxDelayDays              int     `json:"max_delay_days,omitempty"`
}

func NewID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (m *MockDefinition) Normalize() {
	m.Method = strings.ToUpper(strings.TrimSpace(m.Method))
	m.EndpointPattern = normalizePath(m.EndpointPattern)
	if m.StatusCode == 0 {
		m.StatusCode = 200
	}
	if m.ResponseMode == "" {
		m.ResponseMode = ResponseModeStatic
	}
	if m.ExpectedHeaders == nil {
		m.ExpectedHeaders = map[string]string{}
	}
	if m.ExpectedQuery == nil {
		m.ExpectedQuery = map[string]string{}
	}
	if m.ResponseHeaders == nil {
		m.ResponseHeaders = map[string]string{}
	}
	if m.AdditionalVariables == nil {
		m.AdditionalVariables = map[string]any{}
	}
}

func (m MockDefinition) Validate() error {
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidConfig)
	}
	if strings.TrimSpace(m.Method) == "" {
		return fmt.Errorf("%w: method is required", ErrInvalidConfig)
	}
	if strings.TrimSpace(m.EndpointPattern) == "" {
		return fmt.Errorf("%w: endpoint_pattern is required", ErrInvalidConfig)
	}
	switch m.ResponseMode {
	case ResponseModeStatic, ResponseModeTemplate, ResponseModeGenerated:
	default:
		return fmt.Errorf("%w: unsupported response_mode %q", ErrInvalidConfig, m.ResponseMode)
	}
	if m.ResponseMode == ResponseModeGenerated {
		if m.Generator == nil || strings.TrimSpace(m.Generator.Kind) == "" {
			return fmt.Errorf("%w: generator.kind is required when response_mode=generated", ErrInvalidConfig)
		}
	}
	if m.StatusCode < 100 || m.StatusCode > 599 {
		return fmt.Errorf("%w: invalid status_code %d", ErrInvalidConfig, m.StatusCode)
	}
	if m.LatencyMinMS < 0 || m.LatencyMaxMS < 0 {
		return fmt.Errorf("%w: latency must be >= 0", ErrInvalidConfig)
	}
	if m.LatencyMaxMS > 0 && m.LatencyMaxMS < m.LatencyMinMS {
		return fmt.Errorf("%w: latency_max_ms must be >= latency_min_ms", ErrInvalidConfig)
	}
	return nil
}

func normalizePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return trimmed
}
