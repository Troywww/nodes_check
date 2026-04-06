#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")/.."

if [ ! -f ./configs/config.example.yaml ]; then
  echo "config file not found: ./configs/config.example.yaml" >&2
  exit 1
fi

go build -o ./nodes-check ./cmd/server
./nodes-check -config ./configs/config.example.yaml
