COMPOSE ?= docker compose

EXTERNAL_API_URL ?= https://mock.raysouz.studio
EXTERNAL_API_SERIAL ?= b7af3a9e-6d1a-4b15-9837-3e0f0b47e5b4
AWS_ENDPOINT ?= http://localhost:4566
AWS_REGION ?= us-east-1

LAMBDA_URL ?= http://localhost:8088/2015-03-31/functions/function/invocations
SNS_TOPIC_NAME ?= sample-test-routing-events
KAFKA_TOPIC ?= desconto-realizado
KAFKA_BOOTSTRAP_HOST ?= localhost:29092
PAYLOAD ?= payload.json

.PHONY: help prepare build test start health invoke-lambda publish-sns produce-kafka queues logs stop reload clean explain

help:
	@printf "sample-test\n\n"
	@printf "Infra e runtime Docker:\n"
	@printf "  make prepare        Sobe LocalStack, cria DynamoDB/SNS/SQS/ECS cluster e cria topico Kafka\n"
	@printf "  make start          Sobe webview, metrics, ACL, product worker, agent e routing-slip como Lambda\n"
	@printf "  make health         Valida endpoints principais\n"
	@printf "  make logs           Acompanha logs dos containers\n"
	@printf "  make stop           Para toda a stack\n"
	@printf "  make reload         Recria a stack\n\n"
	@printf "Testes:\n"
	@printf "  make invoke-lambda  Invoca a Lambda local com PAYLOAD=payload.json\n"
	@printf "  make publish-sns    Publica PAYLOAD no SNS usando CONVENIO=133341 por padrao\n"
	@printf "  make produce-kafka  Produz PAYLOAD no topico Kafka consumido pelo ACL\n"
	@printf "  make queues         Lista mensagens nas duas filas SQS filtradas\n"
	@printf "  make explain        Consulta o agent de explicabilidade com CORRELATION_ID=...\n"
	@printf "  make test           Executa testes Go e valida o compose\n"

prepare:
	@$(COMPOSE) up -d --wait localstack kafka-broker
	@$(COMPOSE) exec -T localstack /bin/sh /scripts/localstack-init.sh
	@$(COMPOSE) exec -T kafka-broker /bin/sh /scripts/kafka-init.sh

build:
	@$(COMPOSE) build metrics-service acl-graphql-service service-recepcao-conciliacao service-produto-clt agent-service metrics-webview mock-service

test:
	@cd metrics && GOWORK=off go test ./... && GOWORK=off go vet ./...
	@cd acl && GOWORK=off go test ./... && GOWORK=off go vet ./...
	@cd service && GOWORK=off go test ./... && GOWORK=off go vet ./...
	@cd product && GOWORK=off go test ./... && GOWORK=off go vet ./...
	@cd agent && GOWORK=off go test ./... && GOWORK=off go vet ./...
	@cd mock-service && GOWORK=off go test ./... && GOWORK=off go vet ./...
	@$(COMPOSE) config >/dev/null

start: prepare
	@EXTERNAL_API_URL="$(EXTERNAL_API_URL)" EXTERNAL_API_SERIAL="$(EXTERNAL_API_SERIAL)" \
		$(COMPOSE) up -d --build --wait kafka-broker metrics-service acl-graphql-service metrics-webview mock-service service-recepcao-conciliacao service-produto-clt agent-service
	@$(MAKE) health

health:
	@curl --fail --silent http://localhost:8080/health >/dev/null && echo "metrics ECS: ok"
	@curl --fail --silent http://localhost:8090/graphql >/dev/null && echo "acl ECS:     ok"
	@curl --fail --silent http://localhost:4200 >/dev/null && echo "webview ECS: ok"
	@curl --fail --silent http://localhost:8079/health >/dev/null && echo "mock ECS:    ok"
	@curl --fail --silent http://localhost:8087/health >/dev/null && echo "product ECS: ok"
	@curl --fail --silent http://localhost:8095/health >/dev/null && echo "agent ECS:   ok"
	@code=$$(curl --silent --output /dev/null --write-out '%{http_code}' "$(LAMBDA_URL)" -H 'content-type: application/json' --data '{}' || true); \
		[ "$$code" != "000" ] && echo "lambda:      ok" || (echo "lambda:      unavailable"; exit 1)

invoke-lambda:
	@test -f "$(PAYLOAD)" || (echo "Payload nao encontrado: $(PAYLOAD)"; exit 1)
	@curl --fail --silent "$(LAMBDA_URL)" \
		-H 'content-type: application/json' \
		--data-binary "@$(PAYLOAD)"

publish-sns:
	@test -f "$(PAYLOAD)" || (echo "Payload nao encontrado: $(PAYLOAD)"; exit 1)
	@topic_arn=$$(aws --endpoint-url "$(AWS_ENDPOINT)" --region "$(AWS_REGION)" sns create-topic --name "$(SNS_TOPIC_NAME)" --query TopicArn --output text); \
	[ "$$topic_arn" != "None" ] && [ -n "$$topic_arn" ] || (echo "Topico SNS nao encontrado. Execute make prepare."; exit 1); \
	aws --endpoint-url "$(AWS_ENDPOINT)" --region "$(AWS_REGION)" sns publish \
		--topic-arn "$$topic_arn" \
		--message "$$(cat "$(PAYLOAD)")" \
		--message-attributes '{"data.codigo_identificacao_convenio":{"DataType":"String","StringValue":"'"$${CONVENIO:-133341}"'"}}' \
		--output json

produce-kafka:
	@test -f "$(PAYLOAD)" || (echo "Payload nao encontrado: $(PAYLOAD)"; exit 1)
	@cat "$(PAYLOAD)" | $(COMPOSE) exec -T kafka-broker /opt/kafka/bin/kafka-console-producer.sh \
		--bootstrap-server kafka-broker:9092 \
		--topic "$(KAFKA_TOPIC)"

queues:
	@for queue in sample-test-convenio-133341 sample-test-convenio-outros; do \
		url=$$(aws --endpoint-url "$(AWS_ENDPOINT)" --region "$(AWS_REGION)" sqs get-queue-url --queue-name "$$queue" --query QueueUrl --output text); \
		echo "== $$queue =="; \
		aws --endpoint-url "$(AWS_ENDPOINT)" --region "$(AWS_REGION)" sqs receive-message --queue-url "$$url" --max-number-of-messages 10 --wait-time-seconds 1 --output json; \
	done

explain:
	@test -n "$(CORRELATION_ID)" || (echo "Informe CORRELATION_ID=..."; exit 1)
	@curl --fail --silent http://localhost:8095/v1/explain \
		-H 'content-type: application/json' \
		--data '{"target":"product","correlation_id":"$(CORRELATION_ID)","question":"O que aconteceu com este processamento e em que etapa ele terminou?"}'

logs:
	@$(COMPOSE) logs -f localstack kafka-broker metrics-service acl-graphql-service metrics-webview mock-service service-recepcao-conciliacao service-produto-clt agent-service

stop:
	@$(COMPOSE) down --remove-orphans

reload: stop start

clean:
	@$(COMPOSE) down --remove-orphans --volumes
