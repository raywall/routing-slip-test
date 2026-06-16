#!/bin/sh
set -eu

if [ "${ACL_REQUIRE_TOKEN_STS:-true}" = "false" ]; then
  mkdir -p /tmp/config
  cp /app/config/*.json /tmp/config/
  sed -i 's/"require_token_sts"[[:space:]]*:[[:space:]]*true/"require_token_sts": false/' /tmp/config/service.json
  export GRAPHQL_CONFIG_REFERENCE="local:/tmp/config/service.json"
fi

exec /app/acl
