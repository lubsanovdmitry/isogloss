package render

import (
	"slices"
	"strings"

	"isogloss/engine/lex"
	"isogloss/engine/phon"
	"isogloss/engine/pop"
	"isogloss/engine/sim"
)

// Correspondence is one regular sound correspondence: a proto-segment and
// its reflex in each living language, found in Count cognate sets.
type Correspondence struct {
	Proto    string
	Reflexes []string // per living language; "∅" for loss
	Count    int
	Examples []string // concept labels
}

// segKey renders a segment without stress (stress is a word property here).
func segKey(s phon.Segment) string {
	s.Stress = 0
	return phon.Render(s)
}

// Correspondences lists recurring sound correspondences between the
// proto-language and every living language (each by its centre variety),
// over core concepts that all of them still express with the inherited
// root. Each proto-segment is aligned with its reflexes; correspondence
// sets seen at least minCount times are returned, grouped by proto-segment
// in inventory order and most frequent first.
func Correspondences(s *sim.Sim, minCount int) (langs []string, out []Correspondence) {
	ls := s.LivingLanguages()
	for _, l := range ls {
		langs = append(langs, l.Name)
	}
	if len(ls) == 0 {
		return langs, nil
	}
	vars := make([]*lex.Variety, len(ls))
	for i, l := range ls {
		vars[i] = s.CentreVariety(l)
	}
	type acc struct {
		c   Correspondence
		key string
		ord int // first-seen order of the proto-segment
	}
	sets := map[string]*acc{}
	protoOrd := map[string]int{}
	for _, c := range lex.Concepts() {
		if !c.Core() {
			continue
		}
		root := lex.ProtoRoot(c.ID)
		pw := s.Proto.Variety.Lex.At(c.ID).Form.Segs
		aligns := make([][]int, len(vars))
		ok := true
		for i, v := range vars {
			l := v.Lex.At(c.ID)
			if l.Etym != root {
				ok = false
				break
			}
			aligns[i] = pop.Align(pw, l.Form.Segs)
		}
		if !ok {
			continue
		}
		for p, ps := range pw {
			pk := segKey(ps)
			if _, seen := protoOrd[pk]; !seen {
				protoOrd[pk] = len(protoOrd)
			}
			refl := make([]string, len(vars))
			for i, v := range vars {
				if j := aligns[i][p]; j >= 0 {
					refl[i] = segKey(v.Lex.At(c.ID).Form.Segs[j])
				} else {
					refl[i] = "∅"
				}
			}
			k := pk + "\x00" + strings.Join(refl, "\x00")
			a := sets[k]
			if a == nil {
				a = &acc{c: Correspondence{Proto: pk, Reflexes: refl}, key: k, ord: protoOrd[pk]}
				sets[k] = a
			}
			a.c.Count++
			if len(a.c.Examples) < 3 && !slices.Contains(a.c.Examples, c.Label) {
				a.c.Examples = append(a.c.Examples, c.Label)
			}
		}
	}
	var all []*acc
	for _, a := range sets { //lint:ordered sorted below
		if a.c.Count >= minCount {
			all = append(all, a)
		}
	}
	slices.SortFunc(all, func(x, y *acc) int {
		if x.ord != y.ord {
			return x.ord - y.ord
		}
		if x.c.Count != y.c.Count {
			return y.c.Count - x.c.Count
		}
		return strings.Compare(x.key, y.key)
	})
	for _, a := range all {
		out = append(out, a.c)
	}
	return langs, out
}

// CorrespondencesJSON encodes Correspondences.
func CorrespondencesJSON(s *sim.Sim, minCount int) []byte {
	langs, cs := Correspondences(s, minCount)
	var j JSON
	j.Obj().KS("proto", s.ProtoName()).K("langs").Arr()
	for _, l := range langs {
		j.S(l)
	}
	j.EndArr().K("sets").Arr()
	for _, c := range cs {
		j.Obj().KS("proto", c.Proto).KI("count", int64(c.Count)).K("reflexes").Arr()
		for _, r := range c.Reflexes {
			j.S(r)
		}
		j.EndArr().K("examples").Arr()
		for _, e := range c.Examples {
			j.S(e)
		}
		j.EndArr().End()
	}
	j.EndArr().End()
	return j.Bytes()
}
