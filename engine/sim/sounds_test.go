package sim

import (
	"runtime"
	"sync"
	"testing"

	"isogloss/engine/lex"
	"isogloss/engine/phon"
	"isogloss/engine/scenario"
)

// soundRun is what TestRealProtosShedRareSounds measures in one run.
type soundRun struct {
	marked, kept int      // (community, rare proto sound) pairs, and those still in use
	gained, odd  int      // (community, new sound) pairs, and those no PHOIBLE inventory has
	suggested    []string // the six best sound changes suggested at the homeland
}

// TestRealProtosShedRareSounds guards against sinks and immortal sounds:
// started from each real proto-language, rare sounds (in under 10% of
// PHOIBLE inventories: kʷ, θ, ʕ, emphatics, ejectives, laryngeals) must
// fade within 1000 years, sound changes must not mint sounds no language
// has (a lax ɨ̈, an extra-short ă), and suggestions must not be the same
// few changes everywhere. Before 2026-11-10, 95% of rare proto sounds were
// still in use after 1000 years and β ɸ θ ð ɨ̈ piled up.
func TestRealProtosShedRareSounds(t *testing.T) {
	heavy(t)
	langs := []string{"latin", "pie", "arabic", "proto-austronesian", "proto-germanic", "proto-semitic"}
	const seeds = 2
	runs := make([]soundRun, len(langs)*seeds)
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	for li, lang := range langs {
		info, ok := lex.RealByID(lang)
		if !ok {
			t.Fatalf("no proto-language %s", lang)
		}
		sc, err := scenario.Read(scenario.WithLanguage("", lang, info.Name, info.Summary))
		if err != nil {
			t.Fatal(err)
		}
		for k := range seeds {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int, seed uint64) {
				defer func() { <-sem; wg.Done() }()
				s, err := NewScenario(seed, DefaultConfig(), sc)
				if err != nil {
					t.Error(err)
					return
				}
				runs[i] = measureSounds(s)
			}(li*seeds+k, uint64(k+1))
		}
	}
	wg.Wait()

	var marked, kept, gained, odd int
	inLists := map[string]int{}
	for i, r := range runs {
		marked, kept, gained, odd = marked+r.marked, kept+r.kept, gained+r.gained, odd+r.odd
		for _, n := range r.suggested {
			inLists[n]++
		}
		t.Logf("%-18s seed %d: rare sounds kept %3d%%, new sounds unattested %2d%%, suggested %v",
			langs[i/seeds], i%seeds+1, pct(r.kept, r.marked), pct(r.odd, r.gained), r.suggested)
	}
	common, most := 0, ""
	for n, k := range inLists { //lint:ordered max with a name tie-break
		if k > common || k == common && n < most {
			common, most = k, n
		}
	}
	t.Logf("all: rare sounds kept %d%%, new sounds unattested %d%%; %d templates suggested, the commonest (%s) in %d of %d lists",
		pct(kept, marked), pct(odd, gained), len(inLists), most, common, len(runs))
	if pct(kept, marked) > 80 {
		t.Errorf("%d%% of rare proto sounds still in use after 1000 years, want ≤ 80%%", pct(kept, marked))
	}
	if pct(odd, gained) > 3 {
		t.Errorf("%d%% of new sounds are in no PHOIBLE inventory, want ≤ 3%%", pct(odd, gained))
	}
	// The six-best lists drew on 16 templates before the jitter, recency
	// and family cap, 24 after.
	if len(inLists) < 20 {
		t.Errorf("suggestions drew on only %d templates, want ≥ 20", len(inLists))
	}
}

func pct(a, b int) int {
	if b == 0 {
		return 0
	}
	return a * 100 / b
}

// measureSounds runs s for 1000 years and compares every community's
// inventory with the proto-language's.
func measureSounds(s *Sim) soundRun {
	var r soundRun
	var proto map[phon.FeatureSet]bool
	home := int32(-1)
	for _, c := range s.ByHex {
		if c != nil {
			proto = map[phon.FeatureSet]bool{}
			for _, f := range c.Variety.Inventory() {
				proto[f] = true
			}
			home = c.Hex
			break
		}
	}
	s.Run(40)
	for _, c := range s.ByHex {
		if c == nil {
			continue
		}
		now := map[phon.FeatureSet]bool{}
		for _, f := range c.Variety.Inventory() {
			now[f] = true
			if !proto[f] {
				r.gained++
				if phon.Frequency(f) == 0 {
					r.odd++
				}
			}
		}
		for f := range proto { //lint:ordered counting
			if phon.Frequency(f) < 1000 {
				r.marked++
				if now[f] {
					r.kept++
				}
			}
		}
	}
	if s.ByHex[home] != nil {
		for _, sg := range s.SuggestSounds(home, 6, 0) {
			r.suggested = append(r.suggested, sg.Name)
		}
	}
	return r
}
