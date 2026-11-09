package phon

import (
	"strings"
	"testing"
)

func TestCanonKeepsTableSymbols(t *testing.T) {
	for _, s := range Symbols() {
		if c := Canon(s.F); c != s.F {
			t.Errorf("%s: Canon changed a table bundle: %v → %v", s.IPA, s.F, c)
		}
	}
}

func TestCanonFixesIncoherentBundles(t *testing.T) {
	lookup := func(ipa string) FeatureSet {
		f, ok := Lookup(ipa)
		if !ok {
			t.Fatalf("no %s", ipa)
		}
		return f
	}
	for _, c := range []struct {
		name string
		in   FeatureSet
		want string
	}{
		{"unrounded y is i", lookup("y").Set(Round, false), "i"},
		{"raised a is tense e", lookup("a").Set(Low, false).Set(Front, true), "e"},
		{"voiced glottal stop", lookup("ʔ").Set(Voice, true), "ʔ"},
		{"h without spread glottis", lookup("h").Set(SpreadGlottis, false), "h"},
		{"voiced ejective fricative", lookup("xʼ").Set(Voice, true), "xʼ"},
	} {
		if got := RenderFeatures(Canon(c.in)); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestRenderFindsExactSpelling(t *testing.T) {
	// A raised schwa: no table symbol, but a base plus diacritics says it.
	f := Canon(mustSeg(t, "ə").Set(High, true))
	if s := Spell(f); strings.HasPrefix(s, "[") {
		t.Errorf("raised schwa spelled as a matrix: %s", s)
	}
}

func TestLabelsDistinguishCollisions(t *testing.T) {
	x := mustSeg(t, "x")
	odd := x.Set(Labial, true).Set(Round, false).Set(Labiodental, false)
	labs := Labels([]FeatureSet{x, odd, odd.Set(Round, true)})
	seen := map[string]bool{}
	for _, l := range labs {
		if seen[l] {
			t.Fatalf("duplicate label %q in %v", l, labs)
		}
		seen[l] = true
	}
}

func mustSeg(t *testing.T, ipa string) FeatureSet {
	t.Helper()
	s, err := ParseSegments(ipa)
	if err != nil || len(s) != 1 {
		t.Fatalf("%s: %v", ipa, err)
	}
	return s[0].F
}

func TestDoublyArticulatedFricative(t *testing.T) {
	g := mustSeg(t, "ɡ͡b")
	f := g.Set(Continuant, true).Set(DelayedRelease, true) // spirantized ɡ͡b
	if got := RenderFeatures(f); got != "ɣ͡β" {
		t.Errorf("spirantized ɡ͡b renders as %s, want ɣ͡β", got)
	}
}

func TestCombiningMarksSitOnTheLetter(t *testing.T) {
	f := mustSeg(t, "ɨ̈̃ː")
	got := RenderFeatures(f)
	rs := []rune(got)
	if rs[len(rs)-1] != 'ː' {
		t.Errorf("%s: length mark should come last", got)
	}
	if g := mustSeg(t, got); g != f {
		t.Errorf("%s does not parse back", got)
	}
}
