# Agent Service

Serviço de exemplo para explicabilidade usando:

- regras de negócio do workflow via MCP;
- definição estrutural do workflow via MCP;
- snapshots de execução via MCP;
- um modo determinístico para fallback local;
- suporte opcional a um modelo pequeno via endpoint compatível com Ollama.

## Endpoints

- `GET /health`
- `GET /v1/targets`
- `POST /v1/explain`

## Exemplo de uso

```bash
curl --request POST \
  --url http://localhost:8095/v1/explain \
  --header 'content-type: application/json' \
  --data '{
    "target": "product",
    "correlation_id": "corr-baixa-parcelas-001",
    "question": "O que aconteceu com este processamento e em que etapa ele terminou?"
  }'
```

## Modo LLM opcional

Por padrão o serviço usa `heuristic`, que sempre funciona localmente.

Para usar um modelo pequeno hospedado em um servidor compatível com Ollama:

```bash
AGENT_LLM_PROVIDER=ollama
AGENT_OLLAMA_URL=http://localhost:11434
AGENT_OLLAMA_MODEL=smollm2:135m
```
