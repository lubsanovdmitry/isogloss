package render

import (
	"strings"

	"isogloss/engine/lex"
	"isogloss/engine/morph"
	"isogloss/engine/phon"
	"isogloss/engine/sim"
)

// NP is an abstract noun phrase.
type NP struct {
	Noun string // concept label
	PL   bool
	Def  bool
	Adj  string // optional property concept
	Gen  *NP    // optional possessor
	Pron string // a pronoun instead of a noun
}

// Clause is an abstract sentence: concepts, features and roles. Renderers
// apply each variety's lexicon, morphology and word order.
type Clause struct {
	Subj    NP
	Verb    string
	TAM     morph.Slot // PST, FUT, PRF, PROG; NumSlots = present
	Person  morph.Slot // agreement with the subject
	Obj     *NP
	Dat     *NP // recipient or goal
	Loc     *NP
	English string
}

const present = morph.NumSlots

// Fable is the parallel text every language is rendered in.
var Fable = []Clause{
	{Subj: NP{Noun: "child", PL: true, Def: true}, Verb: "see", TAM: morph.PST, Person: morph.P3PL,
		Obj: &NP{Noun: "river", Def: true}, English: "The children saw the river."},
	{Subj: NP{Noun: "male-person", Def: true, Adj: "old"}, Verb: "give", TAM: morph.PST, Person: morph.P3SG,
		Obj: &NP{Noun: "fish"}, Dat: &NP{Noun: "woman", Def: true}, English: "The old man gave the woman a fish."},
	{Subj: NP{Noun: "dog", Def: true}, Verb: "sleep", TAM: morph.PROG, Person: morph.P3SG,
		Loc: &NP{Noun: "house"}, English: "The dog is sleeping in the house."},
	{Subj: NP{Noun: "child", Def: true, Gen: &NP{Noun: "woman", Def: true}}, Verb: "eat", TAM: morph.PRF, Person: morph.P3SG,
		Obj: &NP{Noun: "fish", Def: true}, English: "The woman's child has eaten the fish."},
	{Subj: NP{Pron: "we"}, Verb: "drink", TAM: present, Person: morph.P1PL,
		Obj: &NP{Noun: "water"}, English: "We drink water."},
	{Subj: NP{Noun: "woman", Def: true}, Verb: "go", TAM: morph.FUT, Person: morph.P3SG,
		Dat: &NP{Noun: "mountain", Def: true}, English: "The woman will go to the mountain."},
}

// Word is one rendered word with its gloss.
type Word struct {
	Form, Gloss string
}

type renderer struct {
	v *lex.Variety
}

func concept(label string) lex.ConceptID {
	c, _ := lex.ConceptByLabel(label)
	return c
}

// glossOf is the English gloss of a concept label.
func glossOf(label string) string {
	switch label {
	case "male-person":
		return "man"
	}
	return label
}

func (r *renderer) bare(label string) Word {
	f := r.v.Lex.At(concept(label)).Form
	return Word{phon.RenderWord(f), glossOf(label)}
}

// freeMarker renders a periphrastic exponent: its source word, glossed by
// what it marks.
func (r *renderer) freeMarker(s morph.Slot) Word {
	e := r.v.M.Exp[s]
	if e.Source < 0 {
		return Word{"", s.String()}
	}
	w := r.v.Lex.At(lex.ConceptID(e.Source)).Form
	return Word{strings.ReplaceAll(phon.RenderWord(w), "+", "-"), s.String()}
}

// np renders a noun phrase in a case (NumSlots = nominative).
func (r *renderer) np(n *NP, cas morph.Slot) []Word {
	m := &r.v.M
	o := m.Order
	if n.Pron != "" {
		w := r.bare(n.Pron)
		out := []Word{w}
		if cas != present {
			out = r.adposition(out, cas)
		}
		return out
	}
	slots := []morph.Slot{}
	var before, after []Word
	if n.PL {
		switch e := m.Exp[morph.PL]; {
		case e.Stage.Bound():
			slots = append(slots, morph.PL)
		case e.Stage.Free():
			if o[morph.AdjN] == 0 {
				before = append(before, r.freeMarker(morph.PL))
			} else {
				after = append(after, r.freeMarker(morph.PL))
			}
		}
	}
	caseBound := cas != present && m.Exp[cas].Stage.Bound()
	if caseBound {
		slots = append(slots, cas)
	}
	defOutside := false // a bound article outside an oblique case form
	if n.Def {
		switch e := m.Exp[morph.DEF]; {
		case e.Stage.Bound() && cas == present:
			slots = append(slots, morph.DEF)
		case e.Stage.Bound():
			defOutside = true
		case e.Stage.Free():
			if o[morph.DemN] == 0 {
				before = append([]Word{r.freeMarker(morph.DEF)}, before...)
			} else {
				after = append(after, r.freeMarker(morph.DEF))
			}
		}
	}
	c := concept(n.Noun)
	k := morph.CellIndex(morph.Noun, slots...)
	if k < 0 {
		k = 0
	}
	f := r.v.CellForm(c, morph.Noun, k)
	cell := morph.Cells(morph.Noun)[k]
	if defOutside {
		// Definite and oblique: the article sits inside the case marker
		// (Romanian om-ul-ui, Albanian burr-i-t): start from the stored
		// definite form and add the case's own material outside it.
		dk := morph.CellIndex(morph.Noun, morph.DEF)
		if n.PL {
			dk = morph.CellIndex(morph.Noun, morph.PL, morph.DEF)
		}
		if pre, suf, ok := r.v.Productive(morph.Noun, k); ok {
			pre, suf = onlySlot(pre, cas), onlySlot(suf, cas)
			f = r.v.Finish(morph.Join(pre, r.v.CellForm(c, morph.Noun, dk), suf))
			cell = morph.Cell{Slots: append(append([]morph.Slot(nil), morph.Cells(morph.Noun)[dk].Slots...), cas)}
		}
	}
	form, gloss := Gloss(f, glossOf(n.Noun), cell, m)
	head := []Word{{form, gloss}}
	if n.Adj != "" {
		a := r.bare(n.Adj)
		if o[morph.AdjN] == 0 {
			before = append(before, a)
		} else {
			after = append([]Word{a}, after...)
		}
	}
	out := append(append(before, head...), after...)
	if n.Gen != nil {
		g := r.np(n.Gen, morph.GEN)
		if o[morph.GenN] == 0 {
			out = append(g, out...)
		} else {
			out = append(out, g...)
		}
	}
	if cas != present && !caseBound {
		out = r.adposition(out, cas)
	}
	return out
}

// adposition adds a free case marker on the side word order puts it.
func (r *renderer) adposition(ws []Word, cas morph.Slot) []Word {
	if !r.v.M.Exp[cas].Stage.Free() {
		return ws
	}
	mk := r.freeMarker(cas)
	if r.v.M.Order.Postpositional() {
		return append(ws, mk)
	}
	return append([]Word{mk}, ws...)
}

// verb renders the verb with tense and agreement.
func (r *renderer) verb(c Clause) []Word {
	m := &r.v.M
	var slots []morph.Slot
	var aux []Word
	if c.TAM != present {
		switch e := m.Exp[c.TAM]; {
		case e.Stage.Bound():
			slots = append(slots, c.TAM)
		case e.Stage.Free():
			aux = append(aux, r.freeMarker(c.TAM))
		}
	}
	var v Word
	if src, ok := r.auxVerb(c.TAM); ok {
		// A periphrastic tense built on a verb ('go', 'have', 'sit'): the
		// auxiliary agrees with the subject, the main verb stays plain.
		ak := morph.CellIndex(morph.Verb, c.Person)
		form, gloss := Gloss(r.v.CellForm(src, morph.Verb, ak), c.TAM.String(), morph.Cells(morph.Verb)[ak], m)
		aux = []Word{{form, gloss}}
		v = r.bare(c.Verb)
	} else {
		v = r.finite(c, slots)
	}
	if len(aux) == 0 {
		return []Word{v}
	}
	if m.Order.OV() {
		return append([]Word{v}, aux...)
	}
	return append(aux, v)
}

// auxVerb reports whether a tense is expressed by a separate verb (its
// source concept), which then carries agreement.
func (r *renderer) auxVerb(tam morph.Slot) (lex.ConceptID, bool) {
	if tam == present {
		return 0, false
	}
	e := r.v.M.Exp[tam]
	if !e.Stage.Free() || e.Source < 0 {
		return 0, false
	}
	if p, ok := lex.POSOf(lex.ConceptID(e.Source)); !ok || p != morph.Verb {
		return 0, false
	}
	return lex.ConceptID(e.Source), true
}

// finite renders the main verb with its bound tense and agreement. Only
// the 3rd singular is stored for the future, perfect and progressive; other
// persons swap its agreement for the person's own, as this verb's present
// tense shows it.
func (r *renderer) finite(c Clause, slots []morph.Slot) Word {
	m := &r.v.M
	verb := concept(c.Verb)
	k := morph.CellIndex(morph.Verb, append(slots, c.Person)...)
	if k >= 0 {
		f := r.v.CellForm(verb, morph.Verb, k)
		form, gloss := Gloss(f, c.Verb, morph.Cells(morph.Verb)[k], m)
		return Word{form, gloss}
	}
	k = morph.CellIndex(morph.Verb, append(slots, morph.P3SG)...)
	if k < 0 {
		k = morph.BaseCell(morph.Verb)
	}
	f := withoutSlot(r.v.CellForm(verb, morph.Verb, k), morph.P3SG)
	if pre, suf, ok := r.v.Productive(morph.Verb, morph.CellIndex(morph.Verb, c.Person)); ok {
		f = r.v.Finish(morph.Join(onlySlot(pre, c.Person), f, onlySlot(suf, c.Person)))
	}
	cell := morph.Cell{Slots: append(append([]morph.Slot(nil), slots...), c.Person)}
	form, gloss := Gloss(f, c.Verb, cell, m)
	return Word{form, gloss}
}

// withoutSlot cuts slot s's material out of a form (nothing if the slot
// has no material of its own).
func withoutSlot(w phon.Word, s morph.Slot) phon.Word {
	for i, b := range w.Bounds {
		sl, _, ok := morph.SlotOfTag(b.Tag)
		if !ok || sl != s {
			continue
		}
		lo, hi := b.At, len(w.Segs)
		if i+1 < len(w.Bounds) {
			hi = w.Bounds[i+1].At
		}
		if b.Tag&phon.TagPrefix != 0 {
			lo, hi = 0, b.At
			if i > 0 {
				lo = w.Bounds[i-1].At
			}
		}
		out := phon.Word{Segs: append(append([]phon.Segment(nil), w.Segs[:lo]...), w.Segs[hi:]...)}
		for j, x := range w.Bounds {
			if j == i {
				continue
			}
			if x.At >= hi {
				x.At -= hi - lo
			}
			if x.At > 0 && x.At < len(out.Segs) {
				out.Bounds = append(out.Bounds, x)
			}
		}
		return out
	}
	return w
}

// Render renders a clause in a variety.
func renderClause(v *lex.Variety, c Clause) []Word {
	r := &renderer{v}
	o := v.M.Order
	S := r.np(&c.Subj, present)
	V := r.verb(c)
	var objs [][]Word
	if c.Obj != nil {
		objs = append(objs, r.np(c.Obj, morph.ACC))
	}
	if c.Dat != nil {
		objs = append(objs, r.np(c.Dat, morph.DAT))
	}
	if c.Loc != nil {
		objs = append(objs, r.np(c.Loc, morph.LOC))
	}
	var O []Word
	if o.OV() {
		for i := len(objs) - 1; i >= 0; i-- {
			O = append(O, objs[i]...)
		}
	} else {
		for _, x := range objs {
			O = append(O, x...)
		}
	}
	var parts [][]Word
	switch o[morph.Clause] {
	case 0:
		parts = [][]Word{S, O, V}
	case 1:
		parts = [][]Word{S, V, O}
	case 2:
		parts = [][]Word{V, S, O}
	case 3:
		parts = [][]Word{V, O, S}
	case 4:
		parts = [][]Word{O, V, S}
	default:
		parts = [][]Word{O, S, V}
	}
	var out []Word
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// FableJSON renders the fable in the proto-language(s) and every living
// language, interlinear.
func FableJSON(s *sim.Sim) []byte { return textJSON(s, Fable) }

func textJSON(s *sim.Sim, clauses []Clause) []byte {
	var j JSON
	j.Obj().K("english").Arr()
	for _, c := range clauses {
		j.S(c.English)
	}
	j.EndArr().K("langs").Arr()
	emit := func(name string, id int32, v *lex.Variety) {
		j.Obj().KS("name", name).KI("id", int64(id)).KS("order", v.M.Order.ValueName(morph.Clause)).K("lines").Arr()
		for _, c := range clauses {
			j.Arr()
			for _, w := range renderClause(v, c) {
				j.Obj().KS("f", w.Form).KS("g", w.Gloss).End()
			}
			j.EndArr()
		}
		j.EndArr().End()
	}
	for f, p := range s.Protos {
		emit(s.FamilyName(uint8(f)), -1-int32(f), p.Variety)
	}
	for _, l := range s.LivingLanguages() {
		emit(l.Name, l.ID, s.CentreVariety(l))
	}
	j.EndArr().End()
	return j.Bytes()
}

// onlySlot keeps the part of an affix string that belongs to slot s.
func onlySlot(w phon.Word, s morph.Slot) phon.Word {
	lo, hi := -1, len(w.Segs)
	for i, b := range w.Bounds {
		sl, _, ok := morph.SlotOfTag(b.Tag)
		if ok && sl == s {
			lo = b.At
			if i+1 < len(w.Bounds) {
				hi = w.Bounds[i+1].At
			}
			if b.Tag&phon.TagPrefix != 0 { // a prefix ends at its boundary
				hi = b.At
				lo = 0
				if i > 0 {
					lo = w.Bounds[i-1].At
				}
			}
			out := phon.Word{Segs: append([]phon.Segment(nil), w.Segs[lo:hi]...)}
			bb := b
			bb.At -= lo
			out.Bounds = []phon.Boundary{bb}
			return out
		}
	}
	return phon.Word{}
}
