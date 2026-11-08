GO ?= go
WASM_OUT := web/public/isogloss.wasm
WASM_EXEC := $(shell $(GO) env GOROOT)/lib/wasm/go_js_wasm_exec
# All engine packages except the root lint test (it shells out to `go list`).
WASM_PKGS := ./engine/fx/ ./engine/rng/ ./engine/phon/ ./engine/rules/ ./engine/morph/ ./engine/lex/ ./engine/tmpl/ \
	./engine/world/ ./engine/history/ ./engine/pop/ ./engine/sim/ ./engine/render/

.PHONY: build test test-wasm wasm web web-dev data golden-update

build:
	$(GO) build -o bin/isogloss ./cmd/isogloss

test:
	$(GO) vet ./...
	$(GO) test ./...

test-wasm:
	GOOS=js GOARCH=wasm $(GO) test -exec="$(WASM_EXEC)" $(WASM_PKGS)

wasm:
	GOOS=js GOARCH=wasm $(GO) build -trimpath -ldflags="-s -w" -o $(WASM_OUT) ./wasm
	@if command -v wasm-opt >/dev/null; then wasm-opt -Oz --enable-bulk-memory $(WASM_OUT) -o $(WASM_OUT); else echo "wasm-opt not found; skipping (install binaryen)"; fi
	@gzip -9c $(WASM_OUT) | wc -c | xargs echo "gzipped bytes:"

web/public/wasm_exec.js:
	cp "$(shell $(GO) env GOROOT)/lib/wasm/wasm_exec.js" $@

# Build the browser app into web/dist (needs npm install in web/ once).
web: wasm web/public/wasm_exec.js
	cd web && npm run check && npm run build

# Development server with hot reload (rebuild the wasm with `make wasm`).
web-dev: wasm web/public/wasm_exec.js
	cd web && npm run dev

data:
	$(GO) run ./cmd/datagen

golden-update:
	$(GO) test ./engine/sim/ -run Golden -update
