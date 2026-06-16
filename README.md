# Sample Test

Laboratorio Docker para validar o uso integrado de:

- `custom-business-metrics/service` como service ECS simulado no Docker, porta `8080`;
- `go-graphql-connector` como ACL GraphQL em service ECS simulado no Docker, porta `8090`;
- `custom-business-metrics/webview` como service ECS simulado no Docker, porta `4200`;
- `sample-test/product` como worker ECS simulado consumindo SQS, portas `8087` e `9094` para health e MCP;
- `sample-test/agent` como agent ECS simulado para explicabilidade usando MCP, porta `8095`;
- `routing-slip-pattern` como Lambda em container Docker, porta `8088`;
- LocalStack para DynamoDB, SNS, SQS, ECS metadata e STS, porta `4566`;
- Kafka para producao de eventos consumidos pelo ACL, porta host `29092`.

Os tres services long-running ficam na mesma rede/cluster Docker `sample-test-cluster` e tambem recebem labels indicando o cluster ECS simulado. O LocalStack cria um cluster ECS com o mesmo nome para representar o ambiente alvo.

## Topologia

```text
Browser -> metrics-webview:4200 -> metrics-service:8080 -> LocalStack DynamoDB

Lambda local:8088 -> routing-slip-pattern
  -> acl-service:8090/graphql
  -> metrics-service:8080/v1/metrics
  -> LocalStack DynamoDB routing-slip-state

product-service:8087
  -> consome SQS sample-test-convenio-133341
  -> acl-service:8090/graphql
  -> metrics-service:8080/v1/metrics
  -> LocalStack DynamoDB routing-slip-state
  -> expõe MCP em :9094/mcp

agent-service:8095
  -> consulta MCP do product-service
  -> responde perguntas sobre o processamento

SNS sample-test-routing-events
  -> SQS sample-test-convenio-133341  filtro data.codigo_identificacao_convenio = 133341
  -> SQS sample-test-convenio-outros  filtro anything-but 133341

Kafka sample-test-acl-events -> acl-service consumer group sample-test-acl
```

## Executar

```bash
make start
```

O comando provisiona LocalStack, DynamoDB, SNS, SQS, cluster ECS simulado, Kafka, topico Kafka e os containers de aplicacao do laboratorio.

## Imagens Docker

Para reduzir consumo de disco, a stack usa poucas familias de imagem:

- `golang:1.25-alpine` para build dos binarios Go;
- `alpine:3.22` para os runtimes pequenos de `metrics-service`, `acl-service` e `metrics-webview`;
- `localstack/localstack:4.9.2` para LocalStack e tambem para o bootstrap AWS, evitando uma imagem extra de AWS CLI;
- `apache/kafka:3.8.1` para Kafka e tambem para o bootstrap do topico;
- `public.ecr.aws/lambda/provided:al2023` apenas para a Lambda local, por causa do contrato de runtime Lambda.

URLs principais:

```text
Metrics API: http://localhost:8080
GraphQL ACL: http://localhost:8090/graphql
Webview:     http://localhost:4200
Product:     http://localhost:8087/health
Product MCP: http://localhost:9094/mcp
Agent:       http://localhost:8095
Lambda RIE:  http://localhost:8088/2015-03-31/functions/function/invocations
LocalStack:  http://localhost:4566
Kafka host:  localhost:29092
```

## Invocar a Lambda

```bash
make invoke-lambda
```

Por padrao o comando usa `payload.json`. Para informar outro payload:

```bash
make invoke-lambda PAYLOAD=meu-evento.json
```

## Publicar no SNS

```bash
make publish-sns
```

O comando publica o `payload.json` no topico `sample-test-routing-events` com o atributo SNS
`data.codigo_identificacao_convenio=133341`. Esse evento vai para a fila
`sample-test-convenio-133341`.

Para testar a fila dos demais convenios:

```bash
make publish-sns CONVENIO=999999
make queues
```

## Produzir evento Kafka

```bash
make produce-kafka
```

O ACL consome o topico `sample-test-acl-events` com o group id `sample-test-acl` e registra os eventos em log. O consumo é propositalmente simples neste laboratorio: ele valida conectividade e consumo do canal de eventos sem alterar o comportamento GraphQL.

## Explicabilidade com MCP

Depois de publicar um evento no SNS e deixar o `product-service` processar a fila filtrada, use:

```bash
make explain CORRELATION_ID=corr-baixa-parcelas-001
```

O agent consulta o MCP do `product-service`, recupera workflow, regras de negócio e snapshots de execução e devolve uma resposta pronta para leitura humana.

Por padrão o `agent-service` usa um modo determinístico local. Se quiser testar com um modelo pequeno em um servidor compatível com Ollama:

```bash
make start \
  AGENT_LLM_PROVIDER=ollama \
  AGENT_OLLAMA_URL=http://host.docker.internal:11434 \
  AGENT_OLLAMA_MODEL=smollm2:135m
```

## Configuracao da API externa

O ACL usa por padrao os mocks publicados em `https://mock.raysouz.studio`.

```bash
make start \
  EXTERNAL_API_URL=https://mock.raysouz.studio \
  EXTERNAL_API_SERIAL=b7af3a9e-6d1a-4b15-9837-3e0f0b47e5b4
```

Sem `EXTERNAL_API_SERIAL`, o servico de token STS pode responder `401` e o GraphQL nao inicia corretamente.

No compose, o ACL sobe com `ACL_REQUIRE_TOKEN_STS=false` por padrao para evitar que uma oscilacao do mock externo de token impeça o ambiente local de iniciar. Para validar o fluxo com token STS real, execute:

```bash
make start ACL_REQUIRE_TOKEN_STS=true
```

## Observabilidade

O agent de metricas embutido usa lote unitario e intervalo curto para que cada etapa apareca no webview imediatamente durante os testes locais. O webview identifica processamentos pelos eventos `routing_slip.*`, independentemente do nome configurado como `source` pelo servico.

O modulo `service` requer `routing-slip-pattern/app` a partir da versao `v1.0.1`. Essa versao preserva variaveis GraphQL como `$codigoCliente` durante o carregamento do workflow. Versoes anteriores podem remover o nome das variaveis e produzir erro de sintaxe na consulta.

## Comandos uteis

```bash
make prepare
make health
make logs
make test
make stop
make clean
```
# routing-slip-test
