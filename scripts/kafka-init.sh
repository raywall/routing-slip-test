#!/bin/sh
set -eu

BOOTSTRAP="${KAFKA_BOOTSTRAP:-localhost:9092}"
TOPIC="${ACL_KAFKA_TOPIC:-sample-test-acl-events}"

until /opt/kafka/bin/kafka-topics.sh --bootstrap-server "${BOOTSTRAP}" --list >/dev/null 2>&1; do
  sleep 1
done

/opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server "${BOOTSTRAP}" \
  --create \
  --if-not-exists \
  --topic "${TOPIC}" \
  --partitions 3 \
  --replication-factor 1

/opt/kafka/bin/kafka-topics.sh --bootstrap-server "${BOOTSTRAP}" --describe --topic "${TOPIC}"
