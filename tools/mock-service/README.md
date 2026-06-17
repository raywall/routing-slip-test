# Mock Service

Serviço Go para cadastrar e servir mocks de APIs via browser, com suporte a:

- CRUD de mocks pela interface web;
- definição de método, status code, headers esperados e query string;
- endpoints com variáveis de path, como `/clientes/{codigoCliente}/contratos/{contratoId}`;
- validação parcial do body de entrada em JSON;
- headers de resposta e body de resposta;
- templates com placeholders baseados na requisição;
- latência mínima e máxima com seleção randômica;
- geração dinâmica de payload para cenários de crédito consignado.

## Executar localmente

```bash
go run .
```

Variáveis:

```text
MOCK_SERVICE_ADDR=:8079
MOCK_SERVICE_DATA=/data/mocks.json
```

## Interface web

Abra:

```text
http://localhost:8079
```

## Placeholders suportados

No modo `template`, use expressões como:

```json
{
  "cliente": "{{path.codigoCliente}}",
  "contrato": "{{path.contratoId}}",
  "origem": "{{query.origem}}",
  "matricula": "{{body.data.codigo_matricula}}"
}
```

## Geradores dinâmicos

Atualmente o serviço inclui:

- `consignado_operacao_v1`
- `consignado_saldos_v1`

Esses geradores permitem produzir respostas randômicas, preservando cliente e contrato recebidos na requisição e respeitando regras de coerência das parcelas.
