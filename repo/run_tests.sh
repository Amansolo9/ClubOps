#!/usr/bin/env bash
set -euo pipefail

go test ./unit_tests/... -v
go test ./API_tests/... -v
