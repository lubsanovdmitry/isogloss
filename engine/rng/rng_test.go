package rng

import "testing"

// Golden values pin the stream so any platform or Go-version drift fails CI.
func TestGolden(t *testing.T) {
	s := New(42, Key("test"), 7)
	var got [3]uint64
	for i := range got {
		got[i] = s.Uint64()
	}
	want := goldenStream
	if got != want {
		t.Fatalf("stream drift: got %#v", got)
	}
}

func TestIndependentKeys(t *testing.T) {
	if New(1, Key("a")).Uint64() == New(1, Key("b")).Uint64() {
		t.Fatal("keys collide")
	}
}

func TestWeighted(t *testing.T) {
	s := New(1)
	if s.Weighted([]uint32{0, 0}) != -1 {
		t.Fatal("zero weights")
	}
	for i := 0; i < 100; i++ {
		if s.Weighted([]uint32{0, 5, 0}) != 1 {
			t.Fatal("only index 1 has weight")
		}
	}
}
