#!/bin/sh
set -eu

mkdir -p /app/configs /app/runtime/cache /app/runtime/logs /app/runtime/outputs

if [ ! -f /app/configs/config.yaml ]; then
  cp /app/defaults/config.yaml /app/configs/config.yaml
  echo "initialized /app/configs/config.yaml from template"
fi

if [ ! -f /app/configs/subscription_urls.txt ]; then
  cp /app/defaults/subscription_urls.txt /app/configs/subscription_urls.txt
  echo "initialized /app/configs/subscription_urls.txt from template"
fi

exec /app/nodes-check -config /app/configs/config.yaml