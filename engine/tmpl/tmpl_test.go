package tmpl

import (
	"strings"
	"testing"

	"isogloss/engine/lex"
	"isogloss/engine/phon"
	"isogloss/engine/rng"
	"isogloss/engine/rules"
)

// Every template instantiates into a valid rule for a range of generated
// inventories (Instantiate panics on invalid rule text).
func TestAllTemplatesInstantiate(t *testing.T) {
	lib := Library()
	if len(lib) < 60 {
		t.Errorf("only %d templates", len(lib))
	}
	families := map[string]int{}
	for _, tp := range lib {
		families[tp.Family]++
	}
	t.Logf("%d templates in %d families", len(lib), len(families))
	for seed := uint64(1); seed <= 15; seed++ {
		p := lex.GenerateProto(rng.New(seed, rng.Key("proto")))
		inv := p.Variety.Inventory()
		r := rng.New(seed, rng.Key("tmpl"))
		for _, tp := range lib {
			for k := 0; k < 4; k++ {
				tp.Instantiate(r, inv)
			}
		}
	}
}

func TestDiffuseAndStressAttrs(t *testing.T) {
	var diffuse, stress int
	for _, tp := range Library() {
		if tp.Diffuse != "" {
			diffuse++
		}
		if tp.Stress >= 0 {
			stress++
		}
	}
	if diffuse < 4 || stress != 3 {
		t.Errorf("diffuse=%d stress=%d", diffuse, stress)
	}
}

func byName(name string) *Template {
	for _, t := range Library() {
		if t.Name == name {
			return t
		}
	}
	panic(name)
}

func TestSpirantizationOutputs(t *testing.T) {
	tp := byName("intervocalic-spirantization")
	r, err := Compile("rule x:\n" + strings.ReplaceAll(tp.Body, "$S", "stop"))
	if err != nil {
		t.Fatal(err)
	}
	for in, want := range map[string]string{"ada": "aða", "aba": "aβa", "aɡa": "aɣa"} {
		w, _ := r.Apply(phon.MustParseWord(in), nil)
		if got := phon.RenderWord(w); got != want {
			t.Errorf("%s → %s, want %s", in, got, want)
		}
	}
}

// filled returns a template's body with each slot set to the given
// alternative (by index; slots not named take their first).
func filled(t *testing.T, name string, alts map[string]int) *Candidate {
	t.Helper()
	tp := byName(name)
	body := tp.Body
	for _, s := range tp.Slots {
		body = strings.ReplaceAll(body, "$"+s.Name, s.Alts[alts[s.Name]])
	}
	r, err := Compile("rule " + name + ":\n" + body)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return &Candidate{T: tp, Rule: r}
}

// The templates that retire rare sounds give the plain sounds they should.
func TestRareSoundsGoSomewhere(t *testing.T) {
	for _, c := range []struct {
		name    string
		alts    map[string]int
		in, out string
	}{
		{"labial-fricative-shift", nil, "aβa", "ava"},
		{"labial-fricative-shift", nil, "ɸa", "fa"},
		{"labial-fricative-shift", map[string]int{"P": 2}, "ɸa", "ha"},
		{"labial-fricative-shift", map[string]int{"P": 3}, "aβa", "awa"},
		{"dental-fricative-shift", nil, "θaða", "tada"},
		{"dental-fricative-shift", map[string]int{"P": 1}, "θaða", "saza"},
		{"dental-fricative-shift", map[string]int{"P": 2}, "θa", "fa"},
		{"dental-fricative-shift", map[string]int{"P": 3}, "aða", "aa"},
		{"pharyngeal-shift", nil, "ʕa", "ʔa"},
		{"pharyngeal-shift", map[string]int{"P": 1}, "ħa", "ha"},
		{"guttural-loss", map[string]int{"G": 2}, "taʕm", "taːm"},
		{"guttural-loss", nil, "peχs", "peːs"},
		{"guttural-loss", map[string]int{"G": 1}, "aɣa", "aa"},
		{"guttural-loss", nil, "χeljo", "eljo"},
		{"guttural-loss", nil, "peːχ", "peː"},
		{"guttural-loss", nil, "χwese", "χwese"},
		{"guttural-vocalization", map[string]int{"V": 1}, "pχteːr", "pateːr"},
		{"guttural-vocalization", map[string]int{"V": 1}, "χneːr", "aneːr"},
		{"guttural-vocalization", map[string]int{"V": 2}, "d̤ʕto", "d̤ito"},
		{"ʔ-loss", map[string]int{"E": 2}, "taʔ", "ta"},
		{"delabialization", nil, "kʷa", "ka"},
		{"delabialization", nil, "ɡʷa", "ɡa"},
		{"labiovelar-to-labial", nil, "kʷa", "pa"},
		{"labiovelar-to-labial", nil, "ɡʷa", "ba"},
		{"depalatalization", nil, "kʲa", "ka"},
		{"depalatalization", map[string]int{"P": 1}, "kʲa", "sa"},
		{"depalatalization", map[string]int{"P": 3}, "mʲa", "ma"},
		{"depalatalization", map[string]int{"P": 4}, "tʲa", "ta"},
		{"emphatic-loss", nil, "tˤaːb", "taːb"},
		{"emphatic-loss", nil, "sˤa", "sa"},
		{"ejective-loss", nil, "kʼa", "ka"},
		{"ejective-loss", map[string]int{"P": 1}, "tʼa", "da"},
		{"ejective-loss", map[string]int{"P": 2}, "tʼa", "tˤa"},
		{"guttural-lowering", nil, "qeχ", "qaχ"},
		{"guttural-lowering", map[string]int{"T": 1}, "qi", "qe"},
		{"guttural-rounding", nil, "ɣʷe", "ɣʷo"},
		{"retroflex-loss", nil, "ɖaʈa", "data"},
		{"lateral-fricative-shift", nil, "ɬa", "sa"},
		{"lateral-fricative-shift", map[string]int{"P": 1}, "ɬa", "ʃa"},
		{"lateral-fricative-shift", map[string]int{"P": 3}, "ɬa", "la"},
		{"palatal-shift", nil, "ʎa", "la"},
		{"palatal-shift", map[string]int{"P": 1}, "ʎa", "ja"},
		{"palatal-shift", map[string]int{"P": 2}, "ɟa", "d͡ʒa"},
		{"palatal-shift", map[string]int{"P": 4}, "ɟa", "ja"},
		{"palatal-shift", map[string]int{"P": 5}, "ɕa", "sa"},
		{"palatal-shift", map[string]int{"P": 6}, "ɕa", "ʃa"},
		{"uvular-fronting", nil, "qaχaʁ", "kaxaɣ"},
		{"uvular-fronting", map[string]int{"P": 1}, "qa", "ʔa"},
		{"uvular-fronting", map[string]int{"P": 2}, "aʁa", "ara"},
	} {
		cand := filled(t, c.name, c.alts)
		w, _ := cand.Rule.Apply(phon.MustParseWord(c.in), nil)
		if got := phon.RenderWord(w); got != c.out {
			t.Errorf("%s %v: %s → %s, want %s", c.name, c.alts, c.in, got, c.out)
		}
	}
}

// Pressure singles out changes aimed at the rare sounds of an inventory.
func TestPressureAimsAtRareSounds(t *testing.T) {
	var inv []phon.FeatureSet
	for _, s := range strings.Fields("p t k kʷ ɡʷ kʲ b d ɡ χ h s m n r l w j a e i o u eː oː") {
		f, ok := phon.Lookup(s)
		if !ok {
			t.Fatalf("no %s", s)
		}
		inv = append(inv, f)
	}
	rar := make([]int, len(inv))
	for i, f := range inv {
		rar[i] = phon.Rarity(f)
	}
	p := map[string]int{}
	for _, tp := range Library() {
		p[tp.Name] = tp.Pressure(inv, rar)
		insertion := tp.Name == "anaptyxis" || tp.Name == "prothesis"
		if len(tp.fills) == 0 && !insertion && tp.Name != "diphthongization" {
			t.Errorf("%s: no targets found", tp.Name)
		}
	}
	t.Logf("guttural-loss %d, delabialization %d, depalatalization %d, intervocalic-voicing %d, apocope %d, consonant-merger %d",
		p["guttural-loss"], p["delabialization"], p["depalatalization"], p["intervocalic-voicing"], p["apocope"], p["consonant-merger"])
	for _, aimed := range []string{"guttural-loss", "delabialization", "depalatalization"} {
		if p[aimed] < 300 {
			t.Errorf("%s: pressure %d, want ≥ 300", aimed, p[aimed])
		}
	}
	for _, broad := range []string{"intervocalic-voicing", "apocope", "nasal-place-assimilation"} {
		if p[broad] > 150 {
			t.Errorf("%s: pressure %d, want ≤ 150", broad, p[broad])
		}
	}
}

// A change applied to any sound in at least 1% of inventories gives a
// bundle that renders unambiguously: its rendering parses back to it, so no
// two sounds of an inventory look alike (ɻ fronted would have rendered as
// l). Known exceptions, older than this test: doubly articulated stops
// spirantized (ɡ͡b > ɣ͡β, k͡p > x͡ɸ) and the placeless or glide-like ʍ, hʲ,
// hʷ, whose changed bundles render as the originals.
func TestChangesOfCommonSoundsRender(t *testing.T) {
	known := map[string]bool{"ɡ͡b": true, "ɡ͡br": true, "ɡ͡bɾ": true, "k͡p": true, "k͡pr": true, "k͡pɾ": true, "ʍ": true, "hʲ": true, "hʷ": true}
	for _, tp := range Library() {
		for _, a := range tp.aims {
			if a.out.Kind != rules.OMatrix {
				continue
			}
			for _, s := range phon.Symbols() {
				if phon.Frequency(s.F) < 100 || !a.pat.MatchFeatures(s.F) || known[s.IPA] {
					continue
				}
				out := phon.Canon(s.F.Apply(a.out.Delta))
				r := phon.RenderFeatures(out)
				if seg, err := phon.ParseSegments(r); err != nil || len(seg) != 1 || seg[0].F != out {
					t.Errorf("%s: %s > %s, which is another sound", tp.Name, s.IPA, r)
				}
			}
		}
	}
}
