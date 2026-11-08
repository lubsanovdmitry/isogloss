package sim

import (
	"os"
	"runtime"
	"runtime/debug"
	"testing"
)

// TestMain caps the heap under wasm: the tests run dozens of 1000-year
// simulations in one process, and a wasm process never returns memory, so
// without a soft limit its address space fills up (fatal "out of memory").
func TestMain(m *testing.M) {
	if runtime.GOARCH == "wasm" {
		debug.SetMemoryLimit(900 << 20)
	}
	os.Exit(m.Run())
}

// heavy skips a long statistical test under wasm and with -short. These
// tests run many 1000-year simulations; on wasm, whose memory never
// shrinks, fragmentation eventually exhausts the 4 GB address space. Their
// results are platform-independent once the golden digests (which do run
// under wasm) match.
func heavy(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("slow")
	}
	if runtime.GOARCH == "wasm" {
		t.Skip("statistical; covered by the golden digests on wasm")
	}
}
