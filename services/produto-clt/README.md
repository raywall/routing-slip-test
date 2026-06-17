# Product Service

Consumidor SQS do `sample-test` que usa o `routing-slip-pattern/app/framework` para processar o workflow `product-processing-sqs`.

## O que faz

- consome a fila `sample-test-convenio-133341`;
- processa o payload com o workflow do routing slip;
- publica métricas no `custom-business-metrics`;
- expõe `GET /health` na porta `8087`;
- expõe `POST /mcp` na porta `9094` para consultas de workflow e execuções.

## MCP disponível

- `tools/list`
- `tools/call` com:
  - `explain_workflow`
  - `list_business_rules`
  - `get_execution`
  - `find_executions`

## Exemplo

```bash
curl --request POST \
  --url http://localhost:9094/mcp \
  --header 'content-type: application/json' \
  --data '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "find_executions",
      "arguments": {
        "correlation_id": "corr-baixa-parcelas-001"
      }
    }
  }'
```
