package sim

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"testing"

	"isogloss/engine/lex"
	"isogloss/engine/phon"
)

var update = flag.Bool("update", false, "rewrite golden digests")

const goldenPath = "../../testdata/golden/digests.txt"

var goldenSeeds = []uint64{1, 42, 1337}

// TestGolden pins the state digest of fixed-seed 40-tick runs. CI runs it on
// amd64, arm64 and wasm; any platform difference or unintended behaviour
// change fails here. Regenerate deliberately with `make golden-update`.
func TestGolden(t *testing.T) {
	got := map[uint64]string{}
	for _, seed := range goldenSeeds {
		s := New(seed, DefaultConfig())
		s.Run(40)
		got[seed] = strconv.FormatUint(s.Digest(), 16)
	}
	if *update {
		var b strings.Builder
		b.WriteString("# seed\tdigest after 40 ticks (DefaultConfig). Regenerate: make golden-update\n")
		for _, seed := range goldenSeeds {
			b.WriteString(strconv.FormatUint(seed, 10) + "\t" + got[seed] + "\n")
		}
		if err := os.WriteFile(goldenPath, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("%v (run make golden-update)", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || line[0] == '#' {
			continue
		}
		f := strings.Split(line, "\t")
		seed, _ := strconv.ParseUint(f[0], 10, 64)
		if got[seed] != f[1] {
			t.Errorf("seed %d: digest %s, golden %s", seed, got[seed], f[1])
		}
	}
}

func TestReplayAndFork(t *testing.T) {
	a := New(5, DefaultConfig())
	a.Run(10)
	if err := a.Intervene(Intervention{Kind: "rule", Hex: -1, Rule: "rule all-a-to-o:\n    a => o\n"}); err != nil {
		t.Fatal(err)
	}
	a.Run(10)
	sv := a.SaveState()
	b := Replay(sv)
	if a.Digest() != b.Digest() {
		t.Fatal("replay differs from original run")
	}
	// Forking before the intervention and replaying must differ from the
	// run with it, and match a fresh run without it.
	f := Replay(a.Fork(10))
	f.Run(10)
	fresh := New(5, DefaultConfig())
	fresh.Run(20)
	if f.Digest() != fresh.Digest() {
		t.Error("fork at 10 should equal a run without later interventions")
	}
	if f.Digest() == a.Digest() {
		t.Error("intervention had no effect")
	}
}

func TestInterventionApplies(t *testing.T) {
	s := New(9, DefaultConfig())
	s.Run(2)
	if err := s.Intervene(Intervention{Kind: "rule", Hex: -1, Rule: "rule p-to-f:\n    p => f\n"}); err != nil {
		t.Fatal(err)
	}
	s.Step()
	p, _ := phon.Lookup("p")
	for _, c := range s.Communities() {
		for i := 0; i < c.Variety.Lex.Len(); i++ {
			l := c.Variety.Lex.At(lex.ConceptID(i))
			if l.Etym >= lex.FirstCoinedRoot || l.Stratum.Loan() {
				continue // new words and loans proposed earlier may bring p back
			}
			for _, sg := range l.Form.Segs {
				if sg.F == p {
					t.Fatalf("hex %d still has p", c.Hex)
				}
			}
		}
	}
	if err := s.Intervene(Intervention{Kind: "rule", Hex: -1, Rule: "rule bad:\n    a => [+bogus]\n"}); err == nil {
		t.Error("invalid rule accepted")
	}
}
