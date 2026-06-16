#!/bin/sh
set -eu

AWS_ENDPOINT="${AWS_ENDPOINT:-http://localstack:4566}"
REGION="${AWS_REGION:-us-east-1}"
if command -v awslocal >/dev/null 2>&1; then
  AWS="awslocal --region ${REGION}"
else
  AWS="aws --endpoint-url ${AWS_ENDPOINT} --region ${REGION}"
fi

until ${AWS} sts get-caller-identity >/dev/null 2>&1; do
  sleep 1
done

${AWS} dynamodb create-table \
  --table-name custom-business-metrics-events \
  --attribute-definitions AttributeName=pk,AttributeType=S AttributeName=sk,AttributeType=S AttributeName=correlation_id,AttributeType=S AttributeName=trace_id,AttributeType=S AttributeName=timestamp,AttributeType=S \
  --key-schema AttributeName=pk,KeyType=HASH AttributeName=sk,KeyType=RANGE \
  --global-secondary-indexes '[{"IndexName":"correlation-index","KeySchema":[{"AttributeName":"correlation_id","KeyType":"HASH"},{"AttributeName":"timestamp","KeyType":"RANGE"}],"Projection":{"ProjectionType":"ALL"}},{"IndexName":"trace-index","KeySchema":[{"AttributeName":"trace_id","KeyType":"HASH"},{"AttributeName":"timestamp","KeyType":"RANGE"}],"Projection":{"ProjectionType":"ALL"}}]' \
  --billing-mode PAY_PER_REQUEST >/dev/null 2>&1 || true

${AWS} dynamodb update-time-to-live \
  --table-name custom-business-metrics-events \
  --time-to-live-specification Enabled=true,AttributeName=expires_at >/dev/null 2>&1 || true

${AWS} dynamodb create-table \
  --table-name routing-slip-state \
  --attribute-definitions AttributeName=pk,AttributeType=S AttributeName=sk,AttributeType=S \
  --key-schema AttributeName=pk,KeyType=HASH AttributeName=sk,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST >/dev/null 2>&1 || true

${AWS} dynamodb update-time-to-live \
  --table-name routing-slip-state \
  --time-to-live-specification Enabled=true,AttributeName=expires_at >/dev/null 2>&1 || true

TOPIC_ARN="$(${AWS} sns create-topic --name sample-test-routing-events --query TopicArn --output text)"
QUEUE_133341_URL="$(${AWS} sqs create-queue --queue-name sample-test-convenio-133341 --query QueueUrl --output text)"
QUEUE_OTHERS_URL="$(${AWS} sqs create-queue --queue-name sample-test-convenio-outros --query QueueUrl --output text)"

QUEUE_133341_ARN="$(${AWS} sqs get-queue-attributes --queue-url "${QUEUE_133341_URL}" --attribute-names QueueArn --query 'Attributes.QueueArn' --output text)"
QUEUE_OTHERS_ARN="$(${AWS} sqs get-queue-attributes --queue-url "${QUEUE_OTHERS_URL}" --attribute-names QueueArn --query 'Attributes.QueueArn' --output text)"

write_queue_attributes() {
  queue_arn="$1"
  target_file="$2"
  python3 - "${TOPIC_ARN}" "${queue_arn}" "${target_file}" <<'PY'
import json
import sys

topic_arn, queue_arn, target_file = sys.argv[1:]
policy = {
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": "*",
            "Action": "sqs:SendMessage",
            "Resource": queue_arn,
            "Condition": {"ArnEquals": {"aws:SourceArn": topic_arn}},
        }
    ],
}
with open(target_file, "w", encoding="utf-8") as output:
    json.dump({"Policy": json.dumps(policy, separators=(",", ":"))}, output)
PY
}

write_queue_attributes "${QUEUE_133341_ARN}" /tmp/queue-133341-attributes.json
write_queue_attributes "${QUEUE_OTHERS_ARN}" /tmp/queue-others-attributes.json

${AWS} sqs set-queue-attributes --queue-url "${QUEUE_133341_URL}" --attributes file:///tmp/queue-133341-attributes.json
${AWS} sqs set-queue-attributes --queue-url "${QUEUE_OTHERS_URL}" --attributes file:///tmp/queue-others-attributes.json

SUB_133341="$(${AWS} sns subscribe --topic-arn "${TOPIC_ARN}" --protocol sqs --notification-endpoint "${QUEUE_133341_ARN}" --query SubscriptionArn --output text)"
SUB_OTHERS="$(${AWS} sns subscribe --topic-arn "${TOPIC_ARN}" --protocol sqs --notification-endpoint "${QUEUE_OTHERS_ARN}" --query SubscriptionArn --output text)"

${AWS} sns set-subscription-attributes --subscription-arn "${SUB_133341}" --attribute-name RawMessageDelivery --attribute-value true
${AWS} sns set-subscription-attributes --subscription-arn "${SUB_133341}" --attribute-name FilterPolicy --attribute-value '{"data.codigo_identificacao_convenio":["133341"]}'
${AWS} sns set-subscription-attributes --subscription-arn "${SUB_OTHERS}" --attribute-name RawMessageDelivery --attribute-value true
${AWS} sns set-subscription-attributes --subscription-arn "${SUB_OTHERS}" --attribute-name FilterPolicy --attribute-value '{"data.codigo_identificacao_convenio":[{"anything-but":"133341"}]}'

cat >/tmp/sample-test-outputs.env <<EOF
SNS_TOPIC_ARN=${TOPIC_ARN}
SQS_CONVENIO_133341_URL=${QUEUE_133341_URL}
SQS_CONVENIO_OUTROS_URL=${QUEUE_OTHERS_URL}
ECS_CLUSTER=sample-test-cluster
EOF

cat /tmp/sample-test-outputs.env
