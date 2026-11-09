package sim

import (
	"strings"
	"testing"

	"isogloss/engine/lex"
	"isogloss/engine/phon"
	"isogloss/engine/scenario"
)

func starter(t testing.TB, id string) *scenario.Scenario {
	st, ok := scenario.StarterByID(id)
	if !ok {
		t.Fatalf("no starter %s", id)
	}
	sc, err := scenario.Read(st.Src)
	if err != nil {
		t.Fatal(err)
	}
	return sc
}

// langShape returns the number of living languages, the share (‰) of
// communities in the largest, and the size of the largest dialect
// continuum (languages that share a chain of intelligible neighbours).
func langShape(s *Sim) (n, largest, continuum int) {
	comms := len(s.Communities())
	cont := map[int32]int{}
	for _, l := range s.LivingLanguages() {
		n++
		largest = max(largest, len(l.Members)*1000/max(comms, 1))
		if l.Continuum >= 0 {
			cont[l.Continuum]++
		}
	}
	for _, k := range cont { //lint:ordered max is order-independent
		continuum = max(continuum, k)
	}
	return n, largest, continuum
}

// understood is the share (‰) of communities whose speech is intelligible
// with the speech at hex h.
func understood(s *Sim, h int32) int {
	n, ok := 0, 0
	for _, c := range s.Communities() {
		n++
		if s.Intel(s.ByHex[h].Variety, c.Variety) >= s.Threshold() {
			ok++
		}
	}
	return ok * 1000 / max(n, 1)
}

// TestEmpireCollapseContinuum is the M4 gate: under the empire most of the
// continent understands the capital's speech; after the empire falls in
// year 1000 that unity goes and the family breaks up into a dialect
// continuum: a chain of languages whose neighbours understand each other
// but whose ends do not.
func TestEmpireCollapseContinuum(t *testing.T) {
	heavy(t)
	sc := starter(t, "empire-collapse")
	for _, seed := range []uint64{7, 8, 9} {
		s, err := NewScenario(seed, DefaultConfig(), sc)
		if err != nil {
			t.Fatal(err)
		}
		s.Run(int(sc.Events[0].Tick) - 1)
		capital := s.Polities[0].Capital
		u0 := understood(s, capital)
		n0, _, _ := langShape(s)
		s.Run(int(sc.Ticks - s.Tick))
		u1 := understood(s, capital)
		n1, _, cont := langShape(s)
		t.Logf("seed %d: before the fall %d languages, %d‰ understand the capital; %d years later %d languages, %d‰, largest continuum %d languages",
			seed, n0, u0, (sc.Ticks-sc.Events[0].Tick)*25, n1, u1, cont)
		if u0 < 500 {
			t.Errorf("seed %d: under the empire only %d‰ understand the capital", seed, u0)
		}
		if u1 >= u0 || n1 <= n0 || cont < 3 {
			t.Errorf("seed %d: no continuum after the fall (%d → %d languages, continuum of %d, %d‰ → %d‰)", seed, n0, n1, cont, u0, u1)
		}
	}
}

// Every starter scenario sets up and runs a few generations.
func TestStartersRun(t *testing.T) {
	for _, st := range scenario.Starters() {
		sc := starter(t, st.ID)
		s, err := NewScenario(3, DefaultConfig(), sc)
		if err != nil {
			t.Fatalf("%s: %v", st.ID, err)
		}
		s.Run(6)
		if len(s.Protos) != max(1, len(sc.Families)) {
			t.Errorf("%s: %d families", st.ID, len(s.Protos))
		}
		// Scenario runs replay exactly.
		if r := Replay(s.SaveState()); r.Digest() != s.Digest() {
			t.Errorf("%s: replay differs", st.ID)
		}
	}
}

// Each starter shows what it says to watch.
func TestStartersShowWhatTheyPromise(t *testing.T) {
	heavy(t)
	t.Run("steppe riders: every outcome of foreign rule", func(t *testing.T) {
		sc := starter(t, "steppe-riders")
		s, _ := NewScenario(7, DefaultConfig(), sc)
		s.Run(int(sc.Ticks))
		seen := map[string]bool{}
		for _, e := range s.Log {
			if e.Kind == "rule" {
				for _, o := range []string{"elite dominance", "elite assimilation", "stable bilingualism"} {
					if strings.HasPrefix(e.Text, o) {
						seen[o] = true
					}
				}
			}
		}
		if len(seen) < 2 {
			t.Errorf("outcomes seen: %v", seen)
		}
	})
	t.Run("tone delta: tonogenesis", func(t *testing.T) {
		// Tone spreads as an areal wave; count communities, not only the
		// capitals languages are recorded at.
		sc := starter(t, "tone-delta")
		runs := 0
		for _, seed := range []uint64{7, 8, 9, 10} {
			s, _ := NewScenario(seed, DefaultConfig(), sc)
			s.Run(int(sc.Ticks))
			tonal := 0
			for _, c := range s.Communities() {
				v := c.Variety
			words:
				for i := 0; i < v.Lex.Len(); i++ {
					for _, sg := range v.Lex.At(lex.ConceptID(i)).Form.Segs {
						if sg.Tone != phon.ToneNone {
							tonal++
							break words
						}
					}
				}
			}
			t.Logf("seed %d: %d of %d communities tonal", seed, tonal, len(s.Communities()))
			if tonal*5 >= len(s.Communities()) {
				runs++
			}
		}
		if runs < 2 {
			t.Errorf("tone took hold (a fifth of communities) in only %d of 4 runs", runs)
		}
	})
	t.Run("island hopping: founder losses thin inventories", func(t *testing.T) {
		// Each crossing loses the founders' rarest sounds; the losses
		// should last in their descendants (sounds can come back through
		// loans and later changes, but mostly do not within the run).
		sc := starter(t, "island-hopping")
		kept, total := 0, 0
		for _, seed := range []uint64{7, 8, 9, 10} {
			s, _ := NewScenario(seed, DefaultConfig(), sc)
			s.Run(int(sc.Ticks))
			for _, c := range s.Communities() {
				have := map[string]bool{}
				for _, f := range c.Variety.Inventory() {
					have[phon.RenderFeatures(f)] = true
				}
				var lost []string
				for _, e := range c.Variety.Adopted() {
					if e.Kind != lex.EntryInnov {
						continue
					}
					if name, ok := strings.CutPrefix(s.Reg.Get(e.Innov).Template, "founder effect: "); ok {
						lost = append(lost, strings.Split(strings.TrimSuffix(name, " lost"), ", ")...)
					}
				}
				if len(lost) < 2 {
					continue
				}
				total++
				gone := 0
				for _, x := range lost {
					if !have[x] {
						gone++
					}
				}
				if gone*2 > len(lost) {
					kept++
				}
			}
		}
		t.Logf("%d of %d communities with two or more founder losses still lack most of the lost sounds", kept, total)
		if total == 0 || kept*10 < total*7 {
			t.Errorf("founder losses did not last: %d of %d", kept, total)
		}
	})
	t.Run("silk road: words cross families", func(t *testing.T) {
		sc := starter(t, "silk-road")
		s, _ := NewScenario(7, DefaultConfig(), sc)
		s.Run(int(sc.Ticks))
		for _, l := range s.LivingLanguages() {
			v := s.CentreVariety(l)
			cross := 0
			for i := 0; i < v.Lex.Len(); i++ {
				if f, _, ok := lex.RootOrigin(v.Lex.At(lex.ConceptID(i)).Etym); ok && f != v.Family {
					cross++
				}
			}
			if cross == 0 {
				t.Errorf("%s has no words from other families", l.Name)
			}
		}
	})
}

// TestSettingsRoundTrip: a config written as named settings and applied to
// the defaults comes back unchanged (saves carry configs this way).
func TestSettingsRoundTrip(t *testing.T) {
	c := DefaultConfig()
	c.SoundRate, c.LexRate, c.Threshold, c.Society = 120, 900, 400, false
	c.World.W, c.World.H, c.World.Land, c.World.Shape, c.World.Rivers = 30, 22, 380, 1, 14
	got := DefaultConfig()
	for _, st := range c.Settings() {
		if err := ApplySetting(&got, st.Key, st.Value); err != nil {
			t.Fatal(err)
		}
	}
	if got != c {
		t.Errorf("round trip: %+v, want %+v", got, c)
	}
}

// Every real proto-language starts a run on its own (as the UI's picker
// does), keeps its name and listed words, and replays exactly.
func TestRealLanguagesRun(t *testing.T) {
	water, _ := lex.ConceptByLabel("water")
	for _, info := range lex.RealProtos() {
		sc, err := scenario.Read(scenario.WithLanguage("", info.ID, info.Name, info.Summary))
		if err != nil {
			t.Fatalf("%s: %v", info.ID, err)
		}
		s, err := NewScenario(5, DefaultConfig(), sc)
		if err != nil {
			t.Fatalf("%s: %v", info.ID, err)
		}
		if s.ProtoName() != info.Name {
			t.Errorf("%s: named %q", info.ID, s.ProtoName())
		}
		if s.CoinedRoot(0, water) {
			t.Errorf("%s: no word for water", info.ID)
		}
		t.Logf("%s: water %s%s", info.Name, s.Star(0), phon.RenderWord(s.Proto.Variety.Lex.At(water).Form))
		s.Run(6)
		if r := Replay(s.SaveState()); r.Digest() != s.Digest() {
			t.Errorf("%s: replay differs", info.ID)
		}
	}
	if _, err := NewScenario(1, DefaultConfig(), &scenario.Scenario{Name: "x", Proto: scenario.Proto{Language: "klingon"}}); err == nil {
		t.Error("an unknown language was accepted")
	}
}
