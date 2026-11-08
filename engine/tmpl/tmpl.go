// Package tmpl holds the library of sound-change templates and instantiates
// them over a language's inventory. Every instantiated template is ordinary
// rule-syntax text, so each change the simulation makes can be printed and
// replayed with `isogloss sca` (prepend Prelude).
package tmpl

import (
	_ "embed"
	"strconv"
	"strings"
	"sync"

	"isogloss/engine/phon"
	"isogloss/engine/rng"
	"isogloss/engine/rules"
)

//go:embed library.isg
var librarySrc string

// Prelude defines the classes template bodies may use.
const Prelude = `class stop = [-son -cont -delrel -syl]
class affr = [-son -cont +delrel -syl]
class fric = [-son +cont -syl]
class obs = [-son -syl]
class nasal = [+nas -syl]
class liquid = [+son +cons -nas -syl]
class glide = [+son -cons -syl]
class son = [+son -syl]
class sib = [+strid -syl]
`

// Template is one parametrised sound change.
type Template struct {
	Name    string
	Family  string
	Weight  uint32
	Diffuse string // "" regular, else freq-first / rare-first
	Stress  int8   // -1, or a phon.StressRule the change installs
	Slots   []Slot
	Near    []near
	Body    string
	Index   int
}

// Slot is a named placeholder with alternatives.
type Slot struct {
	Name string
	Alts []string // literal alternatives; "@seg[...]" expands over the inventory
}

type near struct {
	A, B string
	Max  int
}

var (
	once    sync.Once
	lib     []*Template
	prelude *rules.File
	loadErr error
)

func load() {
	prelude, loadErr = rules.Parse(Prelude)
	if loadErr != nil {
		return
	}
	lib, loadErr = parseLibrary(librarySrc)
}

// Library returns all templates.
func Library() []*Template {
	once.Do(load)
	if loadErr != nil {
		panic(loadErr)
	}
	return lib
}

// PreludeFile returns the parsed prelude classes.
func PreludeFile() *rules.File {
	Library()
	return prelude
}

func parseLibrary(src string) ([]*Template, error) {
	var out []*Template
	var cur *Template
	for n, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		indented := line[0] == ' ' || line[0] == '\t'
		switch {
		case indented:
			if cur == nil {
				return nil, errAt(n, "body outside template")
			}
			cur.Body += "    " + t + "\n"
		case strings.HasPrefix(t, "template "):
			f := strings.Fields(t[len("template "):])
			cur = &Template{Name: f[0], Weight: 10, Stress: -1, Index: len(out)}
			for _, kv := range f[1:] {
				k, v, _ := strings.Cut(kv, "=")
				switch k {
				case "weight":
					w, err := strconv.Atoi(v)
					if err != nil {
						return nil, errAt(n, "bad weight")
					}
					cur.Weight = uint32(w)
				case "family":
					cur.Family = v
				case "diffuse":
					cur.Diffuse = v
				case "stress":
					r, ok := phon.StressRuleByName(v)
					if !ok {
						return nil, errAt(n, "bad stress rule "+v)
					}
					cur.Stress = int8(r)
				default:
					return nil, errAt(n, "unknown template attribute "+k)
				}
			}
			out = append(out, cur)
		case strings.HasPrefix(t, "slot "):
			name, alts, ok := strings.Cut(t[len("slot "):], "=")
			if !ok || cur == nil {
				return nil, errAt(n, "bad slot")
			}
			s := Slot{Name: strings.TrimSpace(name)}
			for _, a := range strings.Split(alts, "|") {
				s.Alts = append(s.Alts, strings.TrimSpace(a))
			}
			cur.Slots = append(cur.Slots, s)
		case strings.HasPrefix(t, "near "):
			f := strings.Fields(t)
			if len(f) != 4 || cur == nil {
				return nil, errAt(n, "near A B N")
			}
			m, _ := strconv.Atoi(f[3])
			cur.Near = append(cur.Near, near{f[1], f[2], m})
		default:
			return nil, errAt(n, "unexpected line: "+t)
		}
	}
	return out, nil
}

type libError struct {
	line int
	msg  string
}

func (e *libError) Error() string { return "tmpl: line " + strconv.Itoa(e.line) + ": " + e.msg }

func errAt(n int, msg string) error { return &libError{n + 1, msg} }

// Candidate is one instantiation of a template.
type Candidate struct {
	T    *Template
	Src  string // rule source (uses Prelude classes)
	Rule *rules.Rule
}

// segAlternatives expands "@seg[MATRIX]" over the inventory.
func segAlternatives(alt string, inv []phon.FeatureSet) ([]string, []phon.FeatureSet, bool) {
	if !strings.HasPrefix(alt, "@seg[") {
		return nil, nil, false
	}
	f, err := rules.ParseWith("class x = "+alt[len("@seg"):]+"\n", PreludeFile())
	if err != nil {
		panic("tmpl: " + alt + ": " + err.Error())
	}
	pat := f.Classes[0].Pat
	var out []string
	var feats []phon.FeatureSet
	for _, s := range inv {
		if pat.MatchFeatures(s) {
			out = append(out, phon.RenderFeatures(s))
			feats = append(feats, s)
		}
	}
	return out, feats, true
}

// Instantiate fills the template's slots at random from the alternatives
// that the inventory supports. It returns false if no valid filling exists.
func (t *Template) Instantiate(r *rng.Stream, inv []phon.FeatureSet) (*Candidate, bool) {
	for try := 0; try < 6; try++ {
		vals := map[string]string{}
		feats := map[string]phon.FeatureSet{}
		ok := true
		for _, s := range t.Slots {
			alt := s.Alts[r.IntN(len(s.Alts))]
			if names, fs, isSeg := segAlternatives(alt, inv); isSeg {
				// Honour near constraints against already-filled slots.
				var cand []int
				for i := range names {
					good := true
					for _, n := range t.Near {
						other := ""
						if n.B == s.Name {
							other = n.A
						} else if n.A == s.Name {
							other = n.B
						}
						if other == "" {
							continue
						}
						if of, filled := feats[other]; filled {
							if of == fs[i] || phon.Distance(of, fs[i]) > n.Max {
								good = false
							}
						}
					}
					if good {
						cand = append(cand, i)
					}
				}
				if len(cand) == 0 {
					ok = false
					break
				}
				i := cand[r.IntN(len(cand))]
				vals[s.Name] = names[i]
				feats[s.Name] = fs[i]
				continue
			}
			vals[s.Name] = alt
		}
		if !ok {
			continue
		}
		body := t.Body
		// Substitute longer names first so $AB is not read as $A + B.
		names := make([]string, 0, len(t.Slots))
		for _, s := range t.Slots {
			names = append(names, s.Name)
		}
		for i := range names {
			for j := i + 1; j < len(names); j++ {
				if len(names[j]) > len(names[i]) {
					names[i], names[j] = names[j], names[i]
				}
			}
		}
		for _, n := range names {
			body = strings.ReplaceAll(body, "$"+n, vals[n])
		}
		header := "rule " + t.Name
		if t.Diffuse != "" {
			header += " @diffuse(" + t.Diffuse + ")"
		}
		src := header + ":\n" + body
		f, err := rules.ParseWith(src, PreludeFile())
		if err != nil {
			panic("tmpl: template " + t.Name + " produced invalid rule:\n" + src + err.Error())
		}
		return &Candidate{T: t, Src: src, Rule: f.Rules[0]}, true
	}
	return nil, false
}

// Compile parses user or replayed rule text against the prelude.
func Compile(src string) (*rules.Rule, error) {
	f, err := rules.ParseWith(src, PreludeFile())
	if err != nil {
		return nil, err
	}
	if len(f.Rules) != 1 {
		return nil, &libError{0, "expected exactly one rule"}
	}
	return f.Rules[0], nil
}
