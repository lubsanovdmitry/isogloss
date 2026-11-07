package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"isogloss/engine/phon"
)

// canon renders an IPA string through parse → render, so expected outputs
// may be written in NFC or with optional notation.
func canon(t *testing.T, s string) string {
	t.Helper()
	w, err := phon.ParseWord(s)
	if err != nil {
		t.Fatalf("expected form %q: %v", s, err)
	}
	phon.CleanStress(&w, nil)
	return phon.RenderWord(w)
}

func lines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		out = append(out, strings.Fields(l)[0])
	}
	return out
}

// TestFixtures is the M0 gate: every fixture's rules turn words.txt into
// expected.txt.
func TestFixtures(t *testing.T) {
	dirs, _ := filepath.Glob("../../testdata/sca/*")
	if len(dirs) == 0 {
		t.Fatal("no fixtures found")
	}
	for _, dir := range dirs {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(dir, "rules.isg"))
			if err != nil {
				t.Fatal(err)
			}
			f, err := Parse(string(src))
			if err != nil {
				t.Fatal(err)
			}
			words := lines(t, filepath.Join(dir, "words.txt"))
			want := lines(t, filepath.Join(dir, "expected.txt"))
			if len(words) != len(want) {
				t.Fatalf("%d words but %d expected forms", len(words), len(want))
			}
			for i, in := range words {
				got, steps := f.ApplyAll(phon.MustParseWord(in), nil)
				if g, w := phon.RenderWord(got), canon(t, want[i]); g != w {
					var tr []string
					for _, s := range steps {
						tr = append(tr, s.Rule.Name+": "+phon.RenderWord(s.Before)+" → "+phon.RenderWord(s.After))
					}
					t.Errorf("%s: got %s, want %s\n  %s", in, g, w, strings.Join(tr, "\n  "))
				}
			}
		})
	}
}

func apply(t *testing.T, src, word string) string {
	t.Helper()
	f, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	w, _ := f.ApplyAll(phon.MustParseWord(word), nil)
	return phon.RenderWord(w)
}

func TestSpecExamples(t *testing.T) {
	cases := []struct{ src, in, want string }{
		{"class stop = [-cont -son -syl]\nrule v:\n    [stop -voice] => [+voice] / V _ V\n", "apata", "abada"},
		{"rule g:\n    p t k => f θ x\n", "pataka", "faθaxa"},
		{"rule u:\n    [+syl +back] => [-back +front] / _ C* [+syl +high -back stress:0]\n", "ˈkupi", "ˈkypi"},
		{"rule s:\n    [+syl stress:0 -long] => * / [stress:1] C _ C V\n", "ˈdominus", "ˈdomnus"},
		{"rule t:\n    [+syl] ʔ => [tone:rise] * / _ #\n", "kaʔ", "kǎ"},
		// Except blocks before a vowel-initial suffix; the comment is stripped.
		{"rule d @diffuse(freq-first):\n    [-son] => [-voice] / _ #\n    except / _ + [+syl]   # not before a vowel-initial suffix\n", "kab", "kap"},
		// Insertion (prothesis) at the word edge.
		{"rule p:\n    * => e / # _ s C\n", "skola", "eskola"},
		// Morpheme boundaries are invisible to segment matching but visible when named.
		{"rule v:\n    [-son -voice] => [+voice] / V _ V\n", "kap+i", "kab+i"},
		{"rule v:\n    [-son -voice] => [+voice] / V _ + V\n", "kap+i", "kab+i"},
		{"rule v:\n    [-son -voice] => [+voice] / V _ + V\n", "kapi", "kapi"},
		// A word boundary followed by a comment.
		{"rule f:\n    [-son] => [-voice] / _ #   # final devoicing\n", "kad", "kat"},
		// Optional element.
		{"rule n:\n    a => e / _ (C) i\n", "ani", "eni"},
		{"rule n:\n    a => e / _ (C) i\n", "ai", "ei"},
		// Positional list with deletion.
		{"rule l:\n    p t k => f θ *\n", "pataka", "faθaa"},
	}
	for _, c := range cases {
		if got := apply(t, c.src, c.in); got != canon(t, c.want) {
			t.Errorf("%q on %s: got %s want %s", c.src, c.in, got, c.want)
		}
	}
}

func TestBoundaryKept(t *testing.T) {
	f := MustParse("rule a:\n    a => * / _ #\n")
	w, _ := f.ApplyAll(phon.MustParseWord("kap+a"), nil)
	if len(w.Bounds) != 0 || phon.RenderWord(w) != "kap" {
		t.Errorf("trailing boundary should be dropped: %s %+v", phon.RenderWord(w), w.Bounds)
	}
	f = MustParse("rule a:\n    p => b\n")
	w, _ = f.ApplyAll(phon.MustParseWord("kap+a"), nil)
	if len(w.Bounds) != 1 || w.Bounds[0].At != 3 {
		t.Errorf("boundary moved: %+v", w.Bounds)
	}
}

func TestDiffuseParsed(t *testing.T) {
	f := MustParse("rule d @diffuse(freq-first):\n    a => e\nrule e @diffuse:\n    e => i\n")
	if f.Rules[0].Diffuse != "freq-first" || f.Rules[1].Diffuse != "freq-first" {
		t.Errorf("diffuse: %q %q", f.Rules[0].Diffuse, f.Rules[1].Diffuse)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"rule x:\n    a = b\n":                 "line 2",
		"rule x:\n    [+bogus] => a\n":         "unknown feature",
		"rule x:\n    p t => f θ x\n":          "list lengths",
		"rule x:\n    a => b / V V\n":          "needs '_'",
		"rule x\n    a => b\n":                 "end with ':'",
		"    a => b\n":                         "expected 'class'",
		"rule x:\n    a => b / _ C *\n":        "directly follow",
		"rule x:\n    [+syl] C => [+voice]\n":  "only literal",
		"class s = [-son\n":                    "not a single IPA segment",
		"rule x:\n":                            "no clauses",
		"rule x:\n    except / _ a\n":          "except before",
		"rule x:\n    a => [+high stress:7]\n": "stress must be",
	}
	for src, want := range cases {
		_, err := Parse(src)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want error containing %q", src, err, want)
		}
	}
}

func TestFormatRoundTrip(t *testing.T) {
	src := "class stop = [-cont -son -syl]\nrule final-devoicing @diffuse(freq-first):\n    [stop] => [-voice] / _ #\n    except / _ + [+syl]\n"
	f := MustParse(src)
	again := FormatClasses(f) + Format(f.Rules[0])
	g := MustParse(again)
	for _, w := range []string{"kad", "kad+a", "kada"} {
		a, _ := f.ApplyAll(phon.MustParseWord(w), nil)
		b, _ := g.ApplyAll(phon.MustParseWord(w), nil)
		if !a.Equal(b) {
			t.Errorf("%s: formatted rule differs:\n%s", w, again)
		}
	}
}
