package phon

import (
	"testing"
)

func TestTableLoads(t *testing.T) {
	if n := len(Symbols()); n < 1500 {
		t.Fatalf("only %d symbols", n)
	}
	p, ok := Lookup("p")
	if !ok || p.Has(Voice) || !p.Has(Labial) || p.Has(Continuant) {
		t.Fatalf("p features wrong: %v", p)
	}
	if _, ok := Lookup("t͡s"); !ok {
		t.Fatal("affricate should be canonical with tie bar")
	}
}

// Every table symbol renders back to itself unless another symbol with the
// same bundle is preferred; either way parse(render(f)) == f.
func TestRoundTrip(t *testing.T) {
	for _, s := range Symbols() {
		out := RenderFeatures(s.F)
		w, err := ParseWord(out)
		if err != nil {
			t.Fatalf("%s rendered as %q: %v", s.IPA, out, err)
		}
		if len(w.Segs) != 1 || w.Segs[0].F != s.F {
			t.Errorf("%s → %q does not round-trip", s.IPA, out)
		}
	}
}

func TestFeatureEdits(t *testing.T) {
	p, _ := Lookup("p")
	b, _ := Lookup("b")
	if got := RenderFeatures(p.Set(Voice, true)); got != "b" || p.Set(Voice, true) != b {
		t.Errorf("p +voice = %q", got)
	}
	k, _ := Lookup("k")
	if got := RenderFeatures(k.Set(SpreadGlottis, true)); got != "kʰ" {
		t.Errorf("k +sg = %q", got)
	}
}

func TestComposedRender(t *testing.T) {
	// A bundle not in the table renders as nearest base + diacritics and
	// still parses back to the same bundle.
	x, _ := Lookup("ɣ")
	f := x.Set(Nasal, true).Set(Long, true).Set(Short, false)
	out := RenderFeatures(f)
	w, err := ParseWord(out)
	if err != nil || w.Segs[0].F != f {
		t.Errorf("composed %q does not round-trip (%v)", out, err)
	}
}

func TestParseWord(t *testing.T) {
	w, err := ParseWord("ˈkap+i")
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Segs) != 4 || w.Segs[1].Stress != 1 || len(w.Bounds) != 1 || w.Bounds[0].At != 3 {
		t.Fatalf("bad parse: %+v", w)
	}
	Resyllabify(&w, nil)
	if got := RenderWord(w); got != "ˈka.p+i" && got != "ˈkap+i" {
		t.Errorf("render %q", got)
	}
	if _, err := ParseWord("ts"); err != nil {
		t.Fatal(err)
	}
	if w := MustParseWord("ts"); len(w.Segs) != 2 {
		t.Error("plain ts should be two segments")
	}
	if w := MustParseWord("t͡sa"); len(w.Segs) != 2 {
		t.Error("t͡s should be one segment")
	}
	if w := MustParseWord("má"); w.Segs[1].Tone != ToneHigh {
		t.Error("tone mark")
	}
	if _, err := ParseWord("aQ"); err == nil {
		t.Error("expected error")
	}
}

func syllables(s string, p *Phonotactics) string {
	w := MustParseWord(s)
	Resyllabify(&w, p)
	out := ""
	for k, sy := range w.Syll {
		if k > 0 {
			out += "."
		}
		for i := sy.Start; i < sy.End; i++ {
			out += Render(w.Segs[i])
		}
	}
	return out
}

func TestSyllabify(t *testing.T) {
	cases := map[string]string{
		"kapitu": "ka.pi.tu",
		"antra":  "an.tra",
		"astra":  "a.stra",
		"alpa":   "al.pa",
		"aŋkta":  "aŋk.ta",
		"ia":     "i.a",
		"strata": "stra.ta",
	}
	for in, want := range cases {
		if got := syllables(in, nil); got != want {
			t.Errorf("%s: got %s want %s", in, got, want)
		}
	}
	strict := &Phonotactics{Template: Template{MaxOnset: 1, MaxCoda: 1}}
	if got := syllables("antra", strict); got != "ant.ra" {
		t.Errorf("strict: %s", got)
	}
}

func TestStress(t *testing.T) {
	stress := func(s string, r StressRule) string {
		w := MustParseWord(s)
		Resyllabify(&w, nil)
		AssignStress(&w, r)
		return RenderWord(w)
	}
	cases := []struct {
		in   string
		r    StressRule
		want string
	}{
		{"kapitu", StressInitial, "ˈkapitu"},
		{"kapitu", StressPenult, "kaˈpitu"},
		{"kapitu", StressFinal, "kapiˈtu"},
		{"dominus", StressLatin, "ˈdominus"},
		{"amiːkus", StressLatin, "aˈmiːkus"},
		{"kaˈpitu", StressLexical, "kaˈpitu"},
	}
	for _, c := range cases {
		if got := stress(c.in, c.r); got != c.want {
			t.Errorf("%s/%v: got %s want %s", c.in, c.r, got, c.want)
		}
	}
}

func TestTemplate(t *testing.T) {
	tp, err := ParseTemplate("(C)(C)V(C)")
	if err != nil || tp.MaxOnset != 2 || tp.MinOnset != 0 || tp.MaxCoda != 1 {
		t.Fatalf("%+v %v", tp, err)
	}
	if tp.String() != "(C)(C)V(C)" {
		t.Error(tp.String())
	}
}
