// Package tmpl holds the library of sound-change templates and instantiates
// them over a language's inventory. Every instantiated template is ordinary
// rule-syntax text, so each change the simulation makes can be printed and
// replayed with `isogloss sca` (prepend Prelude).
package tmpl

import (
	_ "embed"
	"slices"
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
	aims    []aim     // every distinct target element (with its replacement) over all slot fillings
	fills   [][][]int // per distinct filling, per clause, the aims it targets
}

// aim is one target element of a template and what replaces it.
type aim struct {
	pat *rules.SegPat
	out rules.Out // ODelete for a deletion
}

// rarityAfter is the rarity of what f becomes.
func (a aim) rarityAfter(f phon.FeatureSet) int {
	switch a.out.Kind {
	case rules.ODelete:
		return 0
	case rules.OMatrix:
		return phon.Rarity(phon.Canon(f.Apply(a.out.Delta)))
	case rules.OLit:
		r := 0
		for _, sg := range a.out.Lit {
			r = max(r, phon.Rarity(sg.F))
		}
		return r
	}
	return phon.Rarity(f) // a correspondence list: unknown, no gain
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
	if loadErr != nil {
		return
	}
	for _, t := range lib { // prelude is parsed; Library would deadlock here
		t.findAims()
	}
}

// maxFillings bounds the slot fillings findAims tries per template.
const maxFillings = 512

// findAims collects the target elements of every filling of the
// template's slots: the distinct elements with their replacements in
// aims, and for each distinct filling the aims of each clause. An @seg slot
// is filled with its matrix, which stands for every segment it would expand
// to; a filling that does not parse that way (an @seg inside a matrix) is
// skipped, as are insertions, which target nothing.
func (t *Template) findAims() {
	index := map[string]int{}
	seenFill := map[string]bool{}
	idx := make([]int, len(t.Slots))
	for n := 0; n < maxFillings; n++ {
		body := t.Body
		for _, name := range t.slotNamesLongestFirst() {
			i := t.slotIndex(name)
			alt := t.Slots[i].Alts[idx[i]]
			if strings.HasPrefix(alt, "@seg[") {
				alt = alt[len("@seg"):]
			}
			body = strings.ReplaceAll(body, "$"+name, alt)
		}
		if f, err := rules.ParseWith("rule "+t.Name+":\n"+body, prelude); err == nil {
			var fill [][]int
			for _, c := range f.Rules[0].Clauses {
				var cl []int
				for k, e := range c.Target {
					if e.Kind != rules.ESeg {
						continue
					}
					a := aim{pat: e.Seg, out: replacement(c, k)}
					key := e.Seg.Src + " => " + a.out.Src + strconv.Itoa(int(a.out.Kind))
					j, ok := index[key]
					if !ok {
						j = len(t.aims)
						index[key] = j
						t.aims = append(t.aims, a)
					}
					cl = append(cl, j)
				}
				if len(cl) > 0 {
					fill = append(fill, cl)
				}
			}
			var kb strings.Builder
			for _, cl := range fill {
				for _, j := range cl {
					kb.WriteString(strconv.Itoa(j))
					kb.WriteByte(' ')
				}
				kb.WriteByte('|')
			}
			if key := kb.String(); len(fill) > 0 && !seenFill[key] {
				seenFill[key] = true
				t.fills = append(t.fills, fill)
			}
		}
		// Next filling (odometer order); stop after the last.
		k := 0
		for ; k < len(idx); k++ {
			idx[k]++
			if idx[k] < len(t.Slots[k].Alts) {
				break
			}
			idx[k] = 0
		}
		if k == len(idx) {
			break
		}
	}
}

// replacement is what replaces target element k of clause c. A clause
// whose replacement differs in length from its target is a single deletion
// or a literal sequence (see rules.Clause); each element then becomes the
// whole sequence.
func replacement(c *rules.Clause, k int) rules.Out {
	if len(c.Repl) == len(c.Target) {
		return c.Repl[k]
	}
	if len(c.Repl) == 1 && c.Repl[0].Kind == rules.ODelete {
		return c.Repl[0]
	}
	lit := rules.Out{Kind: rules.OLit}
	for _, o := range c.Repl {
		lit.Lit = append(lit.Lit, o.Lit...)
		lit.Src += o.Src
	}
	return lit
}

func (t *Template) slotIndex(name string) int {
	for i, s := range t.Slots {
		if s.Name == name {
			return i
		}
	}
	return -1
}

// slotNamesLongestFirst lists slot names so that $AB is substituted before
// $A.
func (t *Template) slotNamesLongestFirst() []string {
	names := make([]string, 0, len(t.Slots))
	for _, s := range t.Slots {
		names = append(names, s.Name)
	}
	slices.SortStableFunc(names, func(a, b string) int { return len(b) - len(a) })
	return names
}

// Pressure (per-mille) is how much the template's change would make an
// inventory less unusual (rarity[i] is inv[i]'s): a target element scores
// the mean, over the inventory sounds it matches, of the rarity it removes
// (the sound's rarity less that of what it becomes; nothing if that is
// rarer); a clause scores its best element if all its elements match
// something; a filling of the slots scores its best clause; and the
// template scores the mean over the fillings with a clause that applies,
// which is what a random instantiation aims at. Deleting χ scores χ's
// rarity, kʷ > k most of kʷ's, and kʷ > ɡʷ or voicing stops nothing.
func (t *Template) Pressure(inv []phon.FeatureSet, rarity []int) int {
	score := make([]int, len(t.aims)) // -1: matches nothing
	for j, a := range t.aims {
		sum, n := 0, 0
		for i, f := range inv {
			if a.pat.MatchFeatures(f) {
				sum += max(0, rarity[i]-a.rarityAfter(f))
				n++
			}
		}
		score[j] = -1
		if n > 0 {
			score[j] = sum / n
		}
	}
	total, n := 0, 0
	for _, fill := range t.fills {
		best := -1
		for _, cl := range fill {
			cs := 0
			for _, j := range cl {
				if score[j] < 0 {
					cs = -1
					break
				}
				cs = max(cs, score[j])
			}
			best = max(best, cs)
		}
		if best >= 0 {
			total += best
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return total / n
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
			out = append(out, phon.Spell(s))
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
		for _, n := range t.slotNamesLongestFirst() {
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
