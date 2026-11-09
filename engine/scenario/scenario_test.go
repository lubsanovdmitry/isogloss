package scenario

import "testing"

func TestParseSubset(t *testing.T) {
	src := `# comment
name: Test # trailing comment
summary: >-
  folded
  text
list:
  - a
  - b: 1
    c: "x # not a comment"
  -
    d: 2
nested:
  k: v
seq-at-key:
- 1
- 2
`
	n, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if n.Get("summary").Str != "folded text" {
		t.Errorf("folded scalar: %q", n.Get("summary").Str)
	}
	l := n.Get("list")
	if l.Kind != Seq || len(l.Items) != 3 || l.Items[0].Str != "a" || l.Items[1].Get("c").Str != "x # not a comment" || l.Items[2].Get("d").Str != "2" {
		t.Errorf("list parsed wrong: %+v", l)
	}
	if n.Get("nested").Get("k").Str != "v" || len(n.Get("seq-at-key").Items) != 2 {
		t.Error("nesting parsed wrong")
	}
	for _, bad := range []string{"a: 1\n\tb: 2", "a: 1\na: 2", "a: 1\n   b: 2", "- x\ny: 1"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestStarters(t *testing.T) {
	ss := Starters()
	if len(ss) < 5 {
		t.Fatalf("%d starters", len(ss))
	}
	for _, s := range ss {
		sc, err := Read(s.Src)
		if err != nil {
			t.Fatalf("%s: %v", s.ID, err)
		}
		if sc.Name == "" || sc.Summary == "" || sc.Watch == "" {
			t.Errorf("%s: missing name, summary or watch", s.ID)
		}
	}
	if _, err := Read("name: x\nevents:\n  - year: 100\n    do: explode\n"); err == nil {
		t.Error("unknown action accepted")
	}
	if _, err := Read("name: x\nworld:\n  size: 3\n"); err == nil {
		t.Error("unknown key accepted")
	}
}

func TestWithLanguage(t *testing.T) {
	sc, err := Read(WithLanguage("", "latin", "Latin", "Spoken Latin: cases."))
	if err != nil || sc.Proto.Language != "latin" || sc.Name != "Latin" {
		t.Fatalf("minimal: %+v %v", sc, err)
	}
	for _, st := range Starters() {
		sc, err := Read(WithLanguage(st.Src, "pie", "", ""))
		if err != nil {
			t.Fatalf("%s: %v", st.ID, err)
		}
		if sc.Proto.Language != "pie" {
			t.Errorf("%s: language %q", st.ID, sc.Proto.Language)
		}
		// Applying twice replaces, never duplicates.
		if _, err := Read(WithLanguage(WithLanguage(st.Src, "latin", "", ""), "pie", "", "")); err != nil {
			t.Errorf("%s twice: %v", st.ID, err)
		}
	}
}
