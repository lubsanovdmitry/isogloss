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
	valid(t, "compare", tb.JSON())
	if len(tb.Rows) != 207 || len(tb.Langs) < 2 {
		t.Errorf("table %d rows %d langs", len(tb.Rows), len(tb.Langs))
	}
	valid(t, "lang", LangJSON(s, s.LivingLanguages()[0].ID))
	valid(t, "innovs", InnovListJSON(s, 3))
	valid(t, "innov", InnovJSON(s, 1))
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
