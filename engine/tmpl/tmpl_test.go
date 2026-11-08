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
