package render

import (
	"encoding/json"
	"strings"
	"testing"

	"isogloss/engine/lex"
	"isogloss/engine/sim"
)

func valid(t *testing.T, name string, b []byte) {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("%s: invalid JSON: %v\n%.300s", name, err, b)
	}
}

func TestJSONWriter(t *testing.T) {
	var j JSON
	j.Obj().KS("a", "x\"y\n\x01").K("b").Arr().I(1).I(-2).Obj().End().EndArr().KB("c", true).K("d").Null().End()
	valid(t, "writer", j.Bytes())
	if j.String() != `{"a":"x\"y\n\u0001","b":[1,-2,{}],"c":true,"d":null}` {
		t.Error(j.String())
	}
}

func TestViews(t *testing.T) {
	s := sim.New(42, sim.DefaultConfig())
	s.Run(20)
	valid(t, "world", WorldJSON(s))
	valid(t, "snapshot", SnapshotJSON(s, 0))
	valid(t, "frames", FramesJSON(s))
	valid(t, "concepts", ConceptsJSON())
	tb := Compare(s, true)
	valid(t, "compare", tb.JSON(s))
	if len(tb.Rows) != 207 || len(tb.Langs) < 2 {
		t.Errorf("table %d rows %d langs", len(tb.Rows), len(tb.Langs))
	}
	valid(t, "lang", LangJSON(s, s.LivingLanguages()[0].ID))
	valid(t, "innovs", InnovListJSON(s, 3))
	valid(t, "innov", InnovJSON(s, 1))
	valid(t, "intel", IntelJSON(s))
	valid(t, "place", PlaceJSON(s, s.Communities()[0].Hex))
	valid(t, "place-sea", PlaceJSON(s, 0))
	valid(t, "compare-all", Compare(s, false).JSON(s))
	valid(t, "preview", PreviewJSON(s, s.Communities()[0].Hex, "rule x:\n    a => e\n"))
	valid(t, "preview-error", PreviewJSON(s, 0, "rule x:\n    a => [+nope]\n"))
	w, _ := lex.ConceptByLabel("water")
	e, ok := EtymOf(s, s.LivingLanguages()[0].Centre, w)
	if !ok {
		t.Fatal("no etymology")
	}
	valid(t, "etym", e.JSON())
	if !strings.Contains(e.Text(), "< *") {
		t.Error(e.Text())
	}
	t.Log("\n" + e.Text())
	files := CLDF(s)
	valid(t, "cldf-metadata", []byte(files["Wordlist-metadata.json"]))
	if !strings.HasSuffix(strings.TrimSpace(files["tree.nwk"]), ";") {
		t.Error("newick")
	}
	t.Log(files["tree.nwk"])
}

func TestCorrespondences(t *testing.T) {
	s := sim.New(42, sim.DefaultConfig())
	s.Run(40)
	langs, cs := Correspondences(s, 2)
	if len(cs) < 10 {
		t.Fatalf("only %d correspondence sets", len(cs))
	}
	// Some correspondences must show change (otherwise nothing happened).
	changed := 0
	for _, c := range cs {
		for _, r := range c.Reflexes {
			if r != c.Proto {
				changed++
				break
			}
		}
	}
	if changed == 0 {
		t.Error("no correspondence shows a sound change")
	}
	valid(t, "corresp", CorrespondencesJSON(s, 2))
	for _, c := range cs[:min(len(cs), 12)] {
		t.Logf("*%s : %v  ×%d  %v", c.Proto, c.Reflexes, c.Count, c.Examples)
	}
	t.Log(langs)
}

func TestGrammarViews(t *testing.T) {
	s := sim.New(1001, sim.DefaultConfig())
	s.Run(40)
	l := s.LivingLanguages()[0]
	valid(t, "sketch", SketchJSON(s, l.ID))
	fire, _ := lex.ConceptByLabel("fire")
	valid(t, "paradigm", ParadigmJSON(s, l.ID, fire))
	valid(t, "celletym", CellEtymJSON(s, l.Centre, fire, 1))
	b := FableJSON(s)
	valid(t, "fable", b)
	var f struct {
		English []string
		Langs   []struct {
			Name  string
			Order string
			Lines [][]struct{ F, G string }
		}
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	for _, lang := range f.Langs[:min(3, len(f.Langs))] {
		t.Logf("%s (%s)", lang.Name, lang.Order)
		for i, line := range lang.Lines[:4] {
			var a, g []string
			for _, w := range line {
				n := max(len([]rune(w.F)), len([]rune(w.G)))
				a = append(a, w.F+strings.Repeat(" ", n-len([]rune(w.F))))
				g = append(g, w.G+strings.Repeat(" ", n-len([]rune(w.G))))
			}
			t.Logf("  %s\n    %s\n    '%s'", strings.Join(a, "  "), strings.Join(g, "  "), f.English[i])
		}
	}
	t.Logf("%s", CellEtymJSON(s, l.Centre, fire, 1))
}
