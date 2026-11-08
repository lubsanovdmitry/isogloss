package render

import (
	"slices"
	"strconv"
	"strings"

	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/phon"
	"isogloss/engine/sim"
	"isogloss/engine/tmpl"
)

// WorldJSON describes the static map.
func WorldJSON(s *sim.Sim) []byte {
	var j JSON
	j.Obj().KI("w", int64(s.World.W)).KI("h", int64(s.World.H)).K("hexes").Arr()
	for _, h := range s.World.Hexes {
		j.Obj().KI("id", int64(h.ID)).KI("col", int64(h.Col)).KI("row", int64(h.Row)).
			KS("biome", h.Biome.String()).KI("elev", int64(h.Elev)).KB("river", h.River).
			KI("down", int64(h.Down)).End()
	}
	j.EndArr().End()
	return j.Bytes()
}

func langJSON(j *JSON, l *sim.Language) {
	j.Obj().KI("id", int64(l.ID)).KS("name", l.Name).KI("centre", int64(l.Centre)).
		KI("size", int64(len(l.Members))).KI("continuum", int64(l.Continuum)).
		KI("parent", int64(l.Parent)).KI("born", int64(l.Born)).KI("died", int64(l.Died)).End()
}

func innovLabel(s *sim.Sim, id uint32) string { return s.Reg.Get(id).Label() }

// SnapshotJSON is the per-step state the UI renders: communities, languages,
// tree, and events from index sinceEvent on.
func SnapshotJSON(s *sim.Sim, sinceEvent int) []byte {
	var j JSON
	j.Obj().KI("tick", int64(s.Tick)).KS("proto", s.ProtoName()).
		KI("innovations", int64(len(s.Reg.All))).KI("live", int64(s.LiveCount()))
	j.K("comms").Arr()
	for _, c := range s.Communities() {
		j.Arr().I(int64(c.Hex)).I(int64(c.Pop)).I(int64(c.Lang)).EndArr()
	}
	j.EndArr()
	j.K("langs").Arr()
	for _, l := range s.Langs {
		langJSON(&j, l)
	}
	j.EndArr()
	j.K("tree").Arr()
	for _, n := range s.Tree {
		if n.Parent == -2 {
			continue // collapsed
		}
		j.Obj().KI("id", int64(n.ID)).KI("lang", int64(n.Lang)).KI("parent", int64(n.Parent)).K("children").Arr()
		for _, c := range n.Children {
			j.I(int64(c))
		}
		j.EndArr().KI("shared", int64(len(n.Shared))).K("defining").Arr()
		for _, id := range n.Defining {
			in := s.Reg.Get(id)
			if in.Kind != history.Sound {
				continue
			}
			j.Obj().KU("id", uint64(id)).KS("label", in.Label()).End()
		}
		j.EndArr().KI("definingLex", int64(len(n.Defining))).End()
	}
	j.EndArr()
	j.KI("eventCount", int64(len(s.Log))).K("events").Arr()
	for _, e := range s.Log[min(sinceEvent, len(s.Log)):] {
		eventJSON(&j, e)
	}
	j.EndArr().End()
	return j.Bytes()
}

func eventJSON(j *JSON, e history.Event) {
	j.Obj().KI("tick", int64(e.Tick)).KS("kind", e.Kind).KI("hex", int64(e.Hex)).
		KU("innov", uint64(e.Innov)).KI("lang", int64(e.Lang)).KS("text", e.Text).End()
}

// FramesJSON gives the language of every hex at each clustering tick, for
// scrubbing the map through time.
func FramesJSON(s *sim.Sim) []byte {
	var j JSON
	j.Arr()
	for _, f := range s.Hist {
		j.Obj().KI("tick", int64(f.Tick)).K("lang").Arr()
		for _, l := range f.Lang {
			j.I(int64(l))
		}
		j.EndArr().End()
	}
	j.EndArr()
	return j.Bytes()
}

// Etym is a word's etymology in one community.
type Etym struct {
	Concept  lex.Concept
	Lang     string
	Hex      int32
	Proto    string // proto form
	ProtoLng string
	Form     string
	Steps    []history.TraceStep
	Root     string // root description
}

// EtymOf traces concept c in the community at hex.
func EtymOf(s *sim.Sim, hex int32, c lex.ConceptID) (*Etym, bool) {
	if hex < 0 || int(hex) >= len(s.ByHex) || s.ByHex[hex] == nil || int(c) >= lex.NumConcepts() {
		return nil, false
	}
	com := s.ByHex[hex]
	steps, final := history.Trace(s.Reg, s.Proto.Variety, com.Variety, c)
	e := &Etym{
		Concept: lex.Concepts()[c], Hex: hex, Proto: phon.RenderWord(s.Proto.Variety.Lex.At(c).Form),
		ProtoLng: s.ProtoName(), Form: phon.RenderWord(final.Form), Steps: steps,
	}
	if com.Lang >= 0 {
		e.Lang = s.Langs[com.Lang].Name
	}
	switch {
	case final.Etym == lex.ProtoRoot(c):
		e.Root = "inherited from " + e.ProtoLng
	case final.Etym < lex.FirstCoinedRoot:
		e.Root = "inherited root of '" + lex.Concepts()[final.Etym-1].Label + "'"
	default:
		for _, r := range s.Roots {
			if r.ID == final.Etym {
				e.Root = "coined at t" + strconv.Itoa(int(r.Tick)) + " (" + r.How + ")"
			}
		}
	}
	return e, true
}

// Text formats an etymology like a dictionary entry.
func (e *Etym) Text() string {
	var b strings.Builder
	b.WriteString(e.Form + "  '" + e.Concept.Label + "'  " + e.Lang + "\n")
	b.WriteString("  < *" + e.Proto + "  " + e.ProtoLng + "\n")
	w := 4
	for _, s := range e.Steps {
		w = max(w, len([]rune(s.Label)))
	}
	for i, s := range e.Steps {
		label := s.Label + strings.Repeat(" ", w-len([]rune(s.Label)))
		after := phon.RenderWord(s.After)
		if i < len(e.Steps)-1 {
			after = "*" + after // intermediate stages are reconstructed forms
		}
		b.WriteString("  t" + strconv.Itoa(int(s.Tick)) + strings.Repeat(" ", max(1, 4-len(strconv.Itoa(int(s.Tick))))) +
			label + "  *" + phon.RenderWord(s.Before) + " → " + after + "\n")
	}
	b.WriteString("  root: " + e.Root + "\n")
	return b.String()
}

// JSON encodes an etymology.
func (e *Etym) JSON() []byte {
	var j JSON
	j.Obj().KS("concept", e.Concept.Label).KS("gloss", e.Concept.Gloss).KS("lang", e.Lang).
		KI("hex", int64(e.Hex)).KS("proto", e.Proto).KS("protoLang", e.ProtoLng).KS("form", e.Form).
		KS("root", e.Root).KS("text", e.Text()).K("steps").Arr()
	for _, s := range e.Steps {
		j.Obj().KI("tick", int64(s.Tick)).KU("innov", uint64(s.Innov)).KS("label", s.Label).
			KS("before", phon.RenderWord(s.Before)).KS("after", phon.RenderWord(s.After)).End()
	}
	j.EndArr().End()
	return j.Bytes()
}

// Table is a comparative word list: concepts × languages, with cognate
// classes per row (same root = same class; class 0 is the proto root).
type Table struct {
	Langs []string
	Rows  []TableRow
}

// TableRow is one concept.
type TableRow struct {
	Concept lex.Concept
	Forms   []string // per language; index 0 is the proto-language
	Cog     []int    // cognate class per language
}

// Compare builds the comparative table over living languages (each
// represented by its centre community). core restricts to Swadesh-207.
func Compare(s *sim.Sim, core bool) Table {
	langs := s.LivingLanguages()
	t := Table{Langs: []string{s.ProtoName()}}
	for _, l := range langs {
		t.Langs = append(t.Langs, l.Name)
	}
	for _, c := range lex.Concepts() {
		if core && !c.Core() {
			continue
		}
		row := TableRow{Concept: c}
		classes := map[lex.EtymID]int{}
		add := func(l *lex.Lexeme) {
			k, ok := classes[l.Etym]
			if !ok {
				k = len(classes)
				classes[l.Etym] = k
			}
			row.Forms = append(row.Forms, phon.RenderWord(l.Form))
			row.Cog = append(row.Cog, k)
		}
		add(s.Proto.Variety.Lex.At(c.ID))
		for _, l := range langs {
			add(s.CentreVariety(l).Lex.At(c.ID))
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// JSON encodes the table.
func (t Table) JSON() []byte {
	var j JSON
	j.Obj().K("langs").Arr()
	for _, l := range t.Langs {
		j.S(l)
	}
	j.EndArr().K("rows").Arr()
	for _, r := range t.Rows {
		j.Obj().KS("concept", r.Concept.Label).KI("id", int64(r.Concept.ID)).K("forms").Arr()
		for _, f := range r.Forms {
			j.S(f)
		}
		j.EndArr().K("cog").Arr()
		for _, c := range r.Cog {
			j.I(int64(c))
		}
		j.EndArr().End()
	}
	j.EndArr().End()
	return j.Bytes()
}

// LangJSON is a short grammar sketch of one language: inventory,
// phonotactics, stress and its sound changes in order.
func LangJSON(s *sim.Sim, id int32) []byte {
	var j JSON
	if id < 0 || int(id) >= len(s.Langs) {
		return j.Null().Bytes()
	}
	l := s.Langs[id]
	v := s.ByHex[l.Centre].Variety
	j.Obj()
	j.K("lang")
	langJSON(&j, l)
	var vs, cs []string
	for _, f := range v.Inventory() {
		if f.Has(phon.Syllabic) {
			vs = append(vs, phon.RenderFeatures(f))
		} else {
			cs = append(cs, phon.RenderFeatures(f))
		}
	}
	j.K("vowels").Arr()
	for _, x := range vs {
		j.S(x)
	}
	j.EndArr().K("consonants").Arr()
	for _, x := range cs {
		j.S(x)
	}
	j.EndArr().KS("template", v.Phonotax.Template.String()).KS("stress", v.Stress.String()).
		KS("repair", v.Repair.String())
	j.K("changes").Arr()
	for _, e := range v.Adopted() {
		if e.Kind == lex.EntryRelax {
			j.Obj().KI("tick", int64(e.Tick)).KS("label", "phonotactics relaxed to "+e.Template.String()).End()
			continue
		}
		in := s.Reg.Get(e.Innov)
		if in.Kind != history.Sound || e.Lo != e.Hi && !(e.Lo == 1 && e.Hi == 5) {
			continue
		}
		if e.Lo == e.Hi && e.Lo != 1 && e.Lo != 5 {
			continue // diffusion extension steps
		}
		j.Obj().KI("tick", int64(e.Tick)).KU("innov", uint64(in.ID)).KS("label", in.Label()).KS("rule", in.Src).End()
	}
	j.EndArr()
	var lexCount int
	for _, id := range history.InnovSet(v) {
		if s.Reg.Get(id).Kind == history.Lexical {
			lexCount++
		}
	}
	j.KI("replacements", int64(lexCount)).End()
	return j.Bytes()
}

// InnovJSON describes one innovation and where it applies now.
func InnovJSON(s *sim.Sim, id uint32) []byte {
	var j JSON
	if id == 0 || int(id) > len(s.Reg.All) {
		return j.Null().Bytes()
	}
	in := s.Reg.Get(id)
	j.Obj().KU("id", uint64(id)).KS("kind", in.Kind.String()).KS("label", in.Label()).KS("rule", in.Src).
		KI("origin", int64(in.Origin)).KI("tick", int64(in.Tick)).K("applied").Arr()
	for _, c := range s.Communities() {
		if _, ok := slices.BinarySearch(history.InnovSet(c.Variety), id); ok {
			j.I(int64(c.Hex))
		}
	}
	j.EndArr()
	if w := s.Wave(id); w != nil {
		j.K("level").Arr()
		for _, a := range w.Level {
			j.I(int64(a.Permille()))
		}
		j.EndArr()
	}
	j.End()
	return j.Bytes()
}

// InnovListJSON lists sound innovations adopted in at least minHexes
// communities, most widespread first.
func InnovListJSON(s *sim.Sim, minHexes int) []byte {
	count := map[uint32]int{}
	for _, c := range s.Communities() {
		for _, id := range history.InnovSet(c.Variety) {
			count[id]++
		}
	}
	var ids []uint32
	for id := range count { //lint:ordered sorted below
		if count[id] >= minHexes && s.Reg.Get(id).Kind == history.Sound {
			ids = append(ids, id)
		}
	}
	slices.SortFunc(ids, func(a, b uint32) int {
		if count[a] != count[b] {
			return count[b] - count[a]
		}
		return int(a) - int(b)
	})
	var j JSON
	j.Arr()
	for _, id := range ids {
		in := s.Reg.Get(id)
		j.Obj().KU("id", uint64(id)).KS("label", in.Label()).KI("tick", int64(in.Tick)).
			KI("origin", int64(in.Origin)).KI("hexes", int64(count[id])).End()
	}
	j.EndArr()
	return j.Bytes()
}

// PreviewJSON shows what a rule would change in one community's lexicon,
// without touching the simulation.
func PreviewJSON(s *sim.Sim, hex int32, src string) []byte {
	var j JSON
	r, err := tmpl.Compile(src)
	if err != nil {
		return j.Obj().KS("error", err.Error()).End().Bytes()
	}
	if hex < 0 || int(hex) >= len(s.ByHex) || s.ByHex[hex] == nil {
		return j.Obj().KS("error", "hex is not settled").End().Bytes()
	}
	v := s.ByHex[hex].Variety
	j.Obj().K("changes").Arr()
	n := 0
	for _, c := range lex.Concepts() {
		before := v.Lex.At(c.ID).Form
		after, ch := r.Apply(before, &v.Phonotax)
		if !ch {
			continue
		}
		n++
		if n <= 60 {
			j.Obj().KS("concept", c.Label).KS("before", phon.RenderWord(before)).KS("after", phon.RenderWord(after)).End()
		}
	}
	j.EndArr().KI("count", int64(n)).End()
	return j.Bytes()
}

// ConceptsJSON lists concepts.
func ConceptsJSON() []byte {
	var j JSON
	j.Arr()
	for _, c := range lex.Concepts() {
		j.Obj().KI("id", int64(c.ID)).KS("label", c.Label).KS("gloss", c.Gloss).KB("core", c.Core()).
			KS("field", c.Field).End()
	}
	j.EndArr()
	return j.Bytes()
}
