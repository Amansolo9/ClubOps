#!/usr/bin/env bash
set -euo pipefail

docker run --rm \
  -v "$(pwd)":/app \
  -w /app \
  golang:1.26.1-alpine \
  sh -c "go test ./unit_tests/... -v && go test ./API_tests/... -v"
