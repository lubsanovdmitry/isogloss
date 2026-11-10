package tmpl

import (
	"strings"
	"testing"

	"isogloss/engine/lex"
	"isogloss/engine/phon"
	"isogloss/engine/rng"
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
		{"ʔ-loss", map[string]int{"E": 2}, "taʔ", "ta"},
		{"delabialization", nil, "kʷa", "ka"},
		{"delabialization", nil, "ɡʷa", "ɡa"},
		{"labiovelar-to-labial", nil, "kʷa", "pa"},
		{"labiovelar-to-labial", nil, "ɡʷa", "ba"},
		{"depalatalization", nil, "kʲa", "ka"},
		{"depalatalization", map[string]int{"P": 1}, "kʲa", "sa"},
		{"emphatic-loss", nil, "tˤaːb", "taːb"},
		{"emphatic-loss", nil, "sˤa", "sa"},
		{"ejective-loss", nil, "kʼa", "ka"},
		{"ejective-loss", map[string]int{"P": 1}, "tʼa", "da"},
		{"ejective-loss", map[string]int{"P": 2}, "tʼa", "tˤa"},
		{"guttural-lowering", nil, "qeχ", "qaχ"},
		{"guttural-lowering", map[string]int{"T": 1}, "qi", "qe"},
		{"guttural-rounding", nil, "ɣʷe", "ɣʷo"},
	} {
		cand := filled(t, c.name, c.alts)
		w, _ := cand.Rule.Apply(phon.MustParseWord(c.in), nil)
		if got := phon.RenderWord(w); got != c.out {
			t.Errorf("%s %v: %s → %s, want %s", c.name, c.alts, c.in, got, c.out)
		}
	}
}
