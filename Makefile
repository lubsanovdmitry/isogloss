GO ?= go
WASM_OUT := web/public/isogloss.wasm
WASM_EXEC := $(shell $(GO) env GOROOT)/lib/wasm/go_js_wasm_exec

.PHONY: build test test-wasm wasm data golden-update

build:
	$(GO) build -o bin/isogloss ./cmd/isogloss

test:
	$(GO) vet ./...
	$(GO) test ./...

test-wasm:
	GOOS=js GOARCH=wasm $(GO) test -exec="$(WASM_EXEC)" ./engine/fx/ ./engine/rng/ ./engine/phon/... ./engine/rules/...

wasm:
	GOOS=js GOARCH=wasm $(GO) build -trimpath -ldflags="-s -w" -o $(WASM_OUT) ./wasm
	@if command -v wasm-opt >/dev/null; then wasm-opt -Oz --enable-bulk-memory $(WASM_OUT) -o $(WASM_OUT); else echo "wasm-opt not found; skipping (install binaryen)"; fi
	@gzip -9c $(WASM_OUT) | wc -c | xargs echo "gzipped bytes:"

data:
	$(GO) run ./cmd/datagen

golden-update:
	$(GO) test ./engine/sim/ -run Golden -update
