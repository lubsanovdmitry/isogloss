package render

import (
	"strings"
	"testing"

	"isogloss/engine/sim"
)

func TestStoryMaterial(t *testing.T) {
	s := sim.New(42, sim.DefaultConfig())
	s.Run(30)
	l := s.LivingLanguages()[0]
	cs, err := LangNameCandidates(s, l.ID)
	if err != nil || len(cs) < 4 {
		t.Fatalf("candidates %v %v", cs, err)
	}
	for _, c := range cs {
		t.Logf("%-14s %-14s %s", c.Name, c.Form, c.Gloss)
		if c.Name == "" || c.Gloss == "" {
			t.Error("empty candidate")
		}
	}
	if len(s.Polities) > 0 {
		pc, err := PolityNameCandidates(s, 0)
		if err != nil || len(pc) == 0 {
			t.Errorf("polity candidates %v %v", pc, err)
		}
	}
	valid(t, "era", EraJSON(s, 0, 30))
	valid(t, "mythspec", MythSpecJSON())
	clauses, err := BuildClauses([]ClauseSpec{
		{Subj: NPSpec{Noun: "woman", Def: true, Adj: "old"}, Verb: "give", Tense: "past", Obj: &NPSpec{Noun: "fish"},
			Dat: &NPSpec{Noun: "child", PL: true, Def: true}, English: "The old woman gave the children a fish."},
		{Subj: NPSpec{Pron: "we"}, Verb: "see", Tense: "future", Obj: &NPSpec{Noun: "sun", Def: true}, English: "We will see the sun."},
	})
	if err != nil {
		t.Fatal(err)
	}
	b := TextJSON(s, clauses)
	valid(t, "text", b)
	if !strings.Contains(string(b), "We will see the sun.") {
		t.Error("english missing")
	}
	for _, bad := range [][]ClauseSpec{
		{{Subj: NPSpec{Noun: "run"}, Verb: "see", Tense: "past"}},
		{{Subj: NPSpec{Noun: "woman"}, Verb: "woman", Tense: "past"}},
		{{Subj: NPSpec{Pron: "she"}, Verb: "see", Tense: "past"}},
		{{Subj: NPSpec{Noun: "woman"}, Verb: "see", Tense: "pluperfect"}},
	} {
		if _, err := BuildClauses(bad); err == nil {
			t.Errorf("accepted %+v", bad[0])
		}
	}
	s.Annotate("42/name/lang/0/v1", `{"name":"X"}`)
	r := sim.Replay(s.SaveState())
	if len(r.Annotations) != 1 || r.Digest() != s.Digest() {
		t.Error("annotations not carried through replay, or they changed the digest")
	}
}
