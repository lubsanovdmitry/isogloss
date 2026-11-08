package sim

import (
	"slices"
	"strconv"

	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/phon"
)

// nameSources are concepts whose word may name a language, in order of preference.
var nameSources = []string{"person", "male-person", "earth", "speak", "we", "mouth", "tongue"}

func (s *Sim) settledNbrs(h int32) []int32 {
	var out []int32
	for _, c := range s.contacts[h] {
		out = append(out, c.To)
	}
	return out
}

// cluster recomputes languages, matches them to the previous ones, logs
// splits and extinctions, and rebuilds the tree.
func (s *Sim) cluster() {
	comms := s.Communities()
	groups := s.cl.Cluster(comms, s.ByHex, s.settledNbrs)

	// Match each group to the previous language it overlaps most.
	type match struct {
		overlap, group int
		lang           int32
	}
	var ms []match
	prevCount := make([]map[int32]int, len(groups))
	for gi, g := range groups {
		counts := map[int32]int{}
		for _, h := range g.Members {
			if l := s.ByHex[h].Lang; l >= 0 {
				counts[l]++
			}
		}
		prevCount[gi] = counts
		for _, l := range sortedKeys(counts) {
			ms = append(ms, match{counts[l], gi, l})
		}
	}
	slices.SortStableFunc(ms, func(a, b match) int {
		if a.overlap != b.overlap {
			return b.overlap - a.overlap
		}
		if a.group != b.group {
			return a.group - b.group
		}
		return int(a.lang - b.lang)
	})
	groupLang := make([]int32, len(groups))
	for i := range groupLang {
		groupLang[i] = -1
	}
	taken := map[int32]bool{}
	for _, m := range ms {
		if groupLang[m.group] >= 0 || taken[m.lang] || !s.Langs[m.lang].Alive() {
			continue
		}
		groupLang[m.group] = m.lang
		taken[m.lang] = true
	}
	for gi, g := range groups {
		if groupLang[gi] >= 0 {
			continue
		}
		parent := int32(-1)
		best := 0
		for _, l := range sortedKeys(prevCount[gi]) {
			if prevCount[gi][l] > best {
				parent, best = l, prevCount[gi][l]
			}
		}
		l := &Language{ID: int32(len(s.Langs)), Parent: parent, Born: s.Tick, Died: -1}
		s.Langs = append(s.Langs, l)
		groupLang[gi] = l.ID
		l.Name = s.nameFor(s.ByHex[g.Centre].Variety, parent < 0)
		if parent >= 0 {
			s.log(history.Event{Kind: "split", Hex: g.Centre, Lang: l.ID, Text: l.Name + " splits from " + s.Langs[parent].Name})
		}
	}
	alive := map[int32]bool{}
	for gi, g := range groups {
		l := s.Langs[groupLang[gi]]
		l.Centre, l.Members, l.Continuum = g.Centre, g.Members, g.Continuum
		alive[l.ID] = true
		for _, h := range g.Members {
			s.ByHex[h].Lang = l.ID
		}
	}
	for _, l := range s.Langs {
		if l.Alive() && !alive[l.ID] {
			l.Died = s.Tick
			s.log(history.Event{Kind: "extinct", Lang: l.ID, Hex: l.Centre, Text: l.Name + " is no longer spoken as a separate language"})
		}
	}
	frame := LangFrame{Tick: s.Tick, Lang: make([]int32, len(s.ByHex))}
	for h, c := range s.ByHex {
		frame.Lang[h] = -1
		if c != nil {
			frame.Lang[h] = c.Lang
		}
	}
	s.Hist = append(s.Hist, frame)
	s.buildTree()

	// Drop varieties nobody speaks and their cached intelligibility.
	live := map[uint64]bool{s.Proto.Variety.Key: true}
	for _, c := range comms {
		live[c.Variety.Key] = true
	}
	for k := range s.vars { //lint:ordered deleting entries is order-independent
		if !live[k] {
			delete(s.vars, k)
		}
	}
	s.cl.Prune(live)
}

func sortedKeys(m map[int32]int) []int32 {
	ks := make([]int32, 0, len(m))
	for k := range m { //lint:ordered sorted below
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return ks
}

func (s *Sim) buildTree() {
	var leaves []history.Leaf
	for _, l := range s.Langs {
		if l.Alive() {
			leaves = append(leaves, history.Leaf{Lang: l.ID, Innovs: history.InnovSet(s.ByHex[l.Centre].Variety)})
		}
	}
	s.Tree = history.BuildTree(leaves)
}

// nameFor names a language from its own word for "person" (or a fallback).
func (s *Sim) nameFor(v *lex.Variety, first bool) string {
	used := map[string]bool{}
	for _, l := range s.Langs {
		used[l.Name] = true
	}
	for _, src := range nameSources {
		c, ok := lex.ConceptByLabel(src)
		if !ok {
			continue
		}
		n := phon.Capitalize(phon.Romanize(v.Lex.At(c).Form))
		if len(n) < 2 {
			continue
		}
		if first {
			n = "Old " + n
		}
		if !used[n] {
			return n
		}
	}
	base := phon.Capitalize(phon.Romanize(v.Lex.At(0).Form))
	for i := 2; ; i++ {
		n := base + " " + strconv.Itoa(i)
		if !used[n] {
			return n
		}
	}
}

// ProtoName names the proto-language.
func (s *Sim) ProtoName() string {
	c, _ := lex.ConceptByLabel("person")
	return "Proto-" + phon.Capitalize(phon.Romanize(s.Proto.Variety.Lex.At(c).Form))
}

// LivingLanguages returns spoken languages in ID order.
func (s *Sim) LivingLanguages() []*Language {
	var out []*Language
	for _, l := range s.Langs {
		if l.Alive() {
			out = append(out, l)
		}
	}
	return out
}

// CentreVariety returns the variety at a language's centre.
func (s *Sim) CentreVariety(l *Language) *lex.Variety { return s.ByHex[l.Centre].Variety }
