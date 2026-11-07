# engine

Pure Go, no I/O. Runs natively (`cmd/isogloss`) and as WASM (`wasm/`).

Rules (enforced by `lint_test.go`):

- No `fmt`, `reflect` or `encoding/json` imports. Use `strconv` and `strings.Builder`;
  JSON encoding lives in `wasm/` and `cmd/` behind hand-written `AppendJSON` methods.
- Never range over a map where order matters. Use `internal/ord.Keys`, or mark a
  line `//lint:ordered` when order provably does not matter.
- No floats in simulation state. Decisions use `fx.Q16` or per-mille integers;
  floats are allowed only for display in `render/`.
- Randomness comes only from `rng.New(seed, keys...)`, one stream per subsystem/entity.
