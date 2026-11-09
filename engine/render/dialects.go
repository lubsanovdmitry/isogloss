package render

import (
	"slices"

	"isogloss/engine/lex"
	"isogloss/engine/phon"
	"isogloss/engine/sim"
)

// Dialect views: how one word varies from community to community, which
// the comparative table (one variety per language) hides.

// variant is one local form of a word.
type variant struct {
	form  string
	etym  lex.EtymID
	st    lex.Stratum
	hexes []int32
}

// variants groups communities' words for concept c by form (communities in
// hex order; variants by number of communities, then form).
func variants(s *sim.Sim, c lex.ConceptID, members []int32) []*variant {
	byForm := map[string]*variant{}
	var out []*variant
	for _, h := range members {
		com := s.ByHex[h]
		if com == nil {
			continue
		}
		l := com.Variety.Lex.At(c)
		if l.Absent() {
			continue
		}
		f := phon.RenderWord(l.Form)
		v := byForm[f]
		if v == nil {
			v = &variant{form: f, etym: l.Etym, st: l.Stratum}
			byForm[f] = v
			out = append(out, v)
		}
		v.hexes = append(v.hexes, h)
	}
	slices.SortStableFunc(out, func(a, b *variant) int {
		if len(a.hexes) != len(b.hexes) {
			return len(b.hexes) - len(a.hexes)
		}
		if a.form < b.form {
			return -1
		}
		return 1
	})
	return out
}

// rootClasses numbers the roots of vs by how many communities use them.
func rootClasses(vs []*variant) map[lex.EtymID]int {
	n := map[lex.EtymID]int{}
	var order []lex.EtymID
	for _, v := range vs {
		if _, ok := n[v.etym]; !ok {
			order = append(order, v.etym)
		}
		n[v.etym] += len(v.hexes)
	}
	slices.SortStableFunc(order, func(a, b lex.EtymID) int { return n[b] - n[a] })
	cls := map[lex.EtymID]int{}
	for i, e := range order {
		cls[e] = i
	}
	return cls
}

// DialectMapJSON gives every settled hex's form of a concept: the variants
// (form, root class, loan kind, communities) and, per hex, the index of its
// variant (-1 for none).
func DialectMapJSON(s *sim.Sim, c lex.ConceptID) []byte {
	var hexes []int32
	for _, com := range s.Communities() {
		hexes = append(hexes, com.Hex)
	}
	vs := variants(s, c, hexes)
	cls := rootClasses(vs)
	of := make([]int, len(s.ByHex))
	for i := range of {
		of[i] = -1
	}
	var j JSON
	j.Obj().KS("concept", lex.Concepts()[c].Label).K("variants").Arr()
	for i, v := range vs {
		for _, h := range v.hexes {
			of[h] = i
		}
		j.Obj().KS("form", v.form).KI("root", int64(cls[v.etym])).KI("count", int64(len(v.hexes)))
		if v.st.Loan() {
			j.KS("loan", stratumKind(v.st)).KS("donor", s.DonorName(v.st))
		}
		j.End()
	}
	j.EndArr().K("hex").Arr()
	for _, x := range of {
		j.I(int64(x))
	}
	j.EndArr().End()
	return j.Bytes()
}

// DialectTableJSON lists, for each concept, the local forms of one
// language with how many of its communities use each, the reference
// variety's form first among equals.
func DialectTableJSON(s *sim.Sim, lang int32, core bool) []byte {
	var j JSON
	if lang < 0 || int(lang) >= len(s.Langs) || !s.Langs[lang].Alive() {
		return j.Obj().KS("error", "no such living language").End().Bytes()
	}
	l := s.Langs[lang]
	ref := s.CentreVariety(l)
	j.Obj().KS("lang", l.Name).KI("communities", int64(len(l.Members))).KI("centre", int64(l.Centre)).K("rows").Arr()
	for _, c := range lex.Concepts() {
		if core && !c.Core() {
			continue
		}
		vs := variants(s, c.ID, l.Members)
		cls := rootClasses(vs)
		refForm := ""
		if rl := ref.Lex.At(c.ID); !rl.Absent() {
			refForm = phon.RenderWord(rl.Form)
		}
		j.Obj().KS("concept", c.Label).KS("ref", refForm).K("variants").Arr()
		for _, v := range vs {
			j.Obj().KS("form", v.form).KI("root", int64(cls[v.etym])).KI("count", int64(len(v.hexes)))
			if v.st.Loan() {
				j.KS("loan", stratumKind(v.st)).KS("donor", s.DonorName(v.st))
			}
			j.End()
		}
		j.EndArr().End()
	}
	j.EndArr().End()
	return j.Bytes()
}
