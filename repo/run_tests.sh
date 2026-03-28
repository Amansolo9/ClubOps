#!/usr/bin/env bash
set -euo pipefail

go test ./fullstack/unit_tests/... -v
go test ./fullstack/API_tests/... -v
