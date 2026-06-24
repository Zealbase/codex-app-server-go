SDK_DIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

.PHONY: generate check-schema-drift build test

generate:
	cd $(SDK_DIR) && go run scripts/generate_schema.go

check-schema-drift:
	@# Compare the live codex schema against the pinned one (normalized for key-order stability)
	@TMPDIR=$$(mktemp -d) && \
	codex app-server generate-json-schema --out "$$TMPDIR" 2>/dev/null && \
	python3 $(SDK_DIR)/scripts/compare_schema.py \
	  "$$TMPDIR/codex_app_server_protocol.v2.schemas.json" \
	  "$(SDK_DIR)/internal/protocol/schema/codex_app_server_protocol.v2.schemas.json"

build:
	cd $(SDK_DIR)/.. && go build ./...

test:
	cd $(SDK_DIR)/.. && go test ./...
