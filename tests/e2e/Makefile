# E2E test runner for codex-go SDK.
# Requires `codex app-server` to be reachable (local binary or Docker image).
#
# Variables:
#   E2E_TARGET          docker (default) | local
#   CODEX_BIN           path to codex binary for local mode   (default: codex)
#   CODEX_E2E_IMAGE     Docker image                          (default: ghcr.io/openai/codex:latest)
#   OPENAI_API_KEY      forwarded to the Docker container     (optional)
#   E2E_TIMEOUT         go test -timeout value                (default: 300s)
#   CODEX_SERVER_BASE   deployed codex-server base URL        (default: http://codex-server.localdev.com)

E2E_TARGET      ?= docker
CODEX_BIN       ?= codex
CODEX_E2E_IMAGE ?= ghcr.io/openai/codex:latest
E2E_TIMEOUT     ?= 300s
CODEX_SERVER_BASE ?= http://codex-server.localdev.com

.PHONY: e2e e2e-local e2e-docker deployed-smoke docker-build help

## e2e: Run e2e tests against the configured target (default: docker).
e2e:
	E2E_TARGET=$(E2E_TARGET) \
	CODEX_BIN=$(CODEX_BIN) \
	CODEX_E2E_IMAGE=$(CODEX_E2E_IMAGE) \
	go test -v -count=1 -timeout $(E2E_TIMEOUT) -tags e2e \
		$(if $(RUN),-run $(RUN)) \
		../../...

## e2e-local: Run e2e tests against a locally installed codex binary.
e2e-local:
	$(MAKE) e2e E2E_TARGET=local

## e2e-docker: Run e2e tests against the Docker image.
e2e-docker:
	$(MAKE) e2e E2E_TARGET=docker

## deployed-smoke: Check the deployed codex-server health/readiness endpoints.
deployed-smoke:
	CODEX_SERVER_BASE=$(CODEX_SERVER_BASE) \
	./deployed_smoke.sh

## docker-build: Build a local Docker image for e2e testing.
docker-build:
	docker build -t codex-e2e:local docker/

## help: Print available targets.
help:
	@grep -E '^## ' Makefile | sed 's/## //'
