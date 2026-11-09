package render

import (
	"slices"
	"strconv"
	"strings"

	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/morph"
	"isogloss/engine/phon"
	"isogloss/engine/pop"
	"isogloss/engine/scenario"
	"isogloss/engine/sim"
	"isogloss/engine/society"
	"isogloss/engine/tmpl"
)

// WorldJSON describes the static map.
func WorldJSON(s *sim.Sim) []byte {
	var j JSON
	j.Obj().KI("w", int64(s.World.W)).KI("h", int64(s.World.H)).K("hexes").Arr()
	for _, h := range s.World.Hexes {
		j.Obj().KI("id", int64(h.ID)).KI("col", int64(h.Col)).KI("row", int64(h.Row)).
			KS("biome", h.Biome.String()).KI("elev", int64(h.Elev)).KB("river", h.River).
			KI("down", int64(h.Down)).KI("sub", int64(s.Region[h.ID])).End()
	}
	j.EndArr().K("substrates").Arr()
	for _, sb := range s.Subs {
		if sb.Seed >= 0 {
			j.Obj().KS("name", sb.Name).KI("seed", int64(sb.Seed)).End()
		}
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
	j.Obj().KI("tick", int64(s.Tick)).KS("seed", strconv.FormatUint(s.Seed, 10)).KS("proto", s.ProtoName()).KI("families", int64(len(s.Protos))).K("scenario").Raw(ScenarioJSON(s)).K("campaign").Raw(CampaignJSON(s, false)).
		KI("innovations", int64(len(s.Reg.All))).KI("live", int64(s.LiveCount()))
	j.K("comms").Arr()
	for _, c := range s.Communities() {
		j.Arr().I(int64(c.Hex)).I(int64(c.Pop)).I(int64(c.Lang)).I(int64(c.Polity)).EndArr()
	}
	j.EndArr()
	j.K("polities").Arr()
	for _, p := range s.Polities {
		j.Obj().KI("id", int64(p.ID)).KS("name", p.Name).KI("capital", int64(p.Capital)).KI("founded", int64(p.Founded)).
			KI("fell", int64(p.Fell)).KB("seafaring", p.Seafaring).End()
	}
	j.EndArr().K("routes").Arr()
	for _, rt := range s.Routes {
		j.Arr()
		for _, h := range rt.Path {
			j.I(int64(h))
		}
		j.EndArr()
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
			if !structural(in) {
				continue
			}
			j.Obj().KU("id", uint64(id)).KS("label", in.Label()).KS("kind", in.Kind.String()).End()
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
	Loan     string // how the word was borrowed, "" for native words
	Senses   []string
	Also     string // an emerging extra sense
	Doublets []Doublet
	Spelling string // in the script of the community's polity, "" if unwritten
	Gap      int    // spelling vs pronunciation, per mille
	Script   string
}

// Doublet is another word of the same variety from the same root.
type Doublet struct {
	Concept, Form string
	Loan          bool
}

// RootDesc describes a root: inherited, or coined (and how).
func RootDesc(s *sim.Sim, id lex.EtymID) string {
	switch {
	case id == 0:
		return ""
	case id < lex.FirstCoinedRoot:
		f, c, _ := lex.RootOrigin(id)
		return s.FamilyName(f) + " root of '" + lex.Concepts()[c].Label + "'"
	}
	for _, r := range s.Roots {
		if r.ID == id {
			return "coined at t" + strconv.Itoa(int(r.Tick)) + " (" + r.How + ")"
		}
	}
	return ""
}

// stratumKind names a loan stratum for JSON.
func stratumKind(st lex.Stratum) string {
	switch st.Kind {
	case lex.Substrate:
		return "substrate"
	case lex.Learned:
		return "learned"
	}
	return "loan"
}

// writtenBy returns the polity whose script writes the community's
// language: its own polity, if its capital speaks the same language.
func writtenBy(s *sim.Sim, c *pop.Community) *society.Polity {
	if c == nil || c.Polity < 0 {
		return nil
	}
	p := s.Polities[c.Polity]
	if !p.Alive() || p.Script == nil || s.ByHex[p.Capital].Lang != c.Lang {
		return nil
	}
	return p
}

// StratumDesc describes how a loan entered: "borrowed from Zhich at t23".
func StratumDesc(s *sim.Sim, st lex.Stratum) string {
	switch st.Kind {
	case lex.Borrowed:
		return "borrowed from " + s.DonorName(st) + " at t" + strconv.Itoa(int(st.Tick))
	case lex.Substrate:
		return "from the " + s.DonorName(st) + " substrate, at t" + strconv.Itoa(int(st.Tick))
	case lex.Learned:
		return "learned loan from " + s.DonorName(st) + " at t" + strconv.Itoa(int(st.Tick))
	}
	return ""
}

// EtymOf traces concept c in the community at hex.
func EtymOf(s *sim.Sim, hex int32, c lex.ConceptID) (*Etym, bool) {
	if hex < 0 || int(hex) >= len(s.ByHex) || s.ByHex[hex] == nil || int(c) >= lex.NumConcepts() {
		return nil, false
	}
	com := s.ByHex[hex]
	proto := s.ProtoOf(com.Variety)
	steps, final := history.Trace(s.Reg, proto, com.Variety, c)
	e := &Etym{
		Concept: lex.Concepts()[c], Hex: hex, Proto: phon.RenderWord(proto.Lex.At(c).Form),
		ProtoLng: s.FamilyName(com.Variety.Family), Form: phon.RenderWord(final.Form), Steps: steps,
	}
	if com.Lang >= 0 {
		e.Lang = s.Langs[com.Lang].Name
	}
	v := com.Variety
	switch {
	case final.Absent():
		e.Root = "no word: the thing is not known here yet"
	case final.Etym == lex.FamilyRoot(com.Variety.Family, c) && !final.Stratum.Loan():
		e.Root = "inherited from " + e.ProtoLng
	default:
		e.Root = RootDesc(s, final.Etym)
	}
	e.Loan = StratumDesc(s, final.Stratum)
	for _, x := range v.Senses(c) {
		if x != c {
			e.Senses = append(e.Senses, lex.Concepts()[x].Label)
		}
	}
	if a, ok := final.AlsoSense(); ok {
		e.Also = lex.Concepts()[a].Label
	}
	if p := writtenBy(s, com); p != nil && !final.Absent() {
		e.Spelling, e.Gap, e.Script = p.Script.SpellingOf(c, &final), p.Script.Gap(c, &final), p.Script.Type.String()+" of "+p.Name
	}
	for _, x := range v.Doublets(c) {
		l := v.Lex.At(x)
		e.Doublets = append(e.Doublets, Doublet{lex.Concepts()[x].Label, phon.RenderWord(l.Form), l.Stratum.Loan()})
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
	if e.Loan != "" {
		b.WriteString("  " + e.Loan + "\n")
	}
	b.WriteString("  root: " + e.Root + "\n")
	if len(e.Senses) > 0 {
		b.WriteString("  also means: " + strings.Join(e.Senses, ", ") + "\n")
	}
	for _, d := range e.Doublets {
		b.WriteString("  doublet: " + d.Form + " '" + d.Concept + "'\n")
	}
	return b.String()
}

// JSON encodes an etymology.
func (e *Etym) JSON() []byte {
	var j JSON
	j.Obj().KS("concept", e.Concept.Label).KS("gloss", e.Concept.Gloss).KS("lang", e.Lang).
		KI("hex", int64(e.Hex)).KS("proto", e.Proto).KS("protoLang", e.ProtoLng).KS("form", e.Form).
		KS("root", e.Root).KS("loan", e.Loan).KS("also", e.Also).KS("spelling", e.Spelling).KI("gap", int64(e.Gap)).KS("script", e.Script).KS("text", e.Text()).K("senses").Arr()
	for _, x := range e.Senses {
		j.S(x)
	}
	j.EndArr().K("doublets").Arr()
	for _, d := range e.Doublets {
		j.Obj().KS("concept", d.Concept).KS("form", d.Form).KB("loan", d.Loan).End()
	}
	j.EndArr().K("steps").Arr()
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
	Protos int // the first Protos columns are proto-languages
	Langs  []string
	Rows   []TableRow
}

// TableRow is one concept.
type TableRow struct {
	Concept lex.Concept
	Forms   []string // per language; index 0 is the proto-language; "" for no word
	Cog     []int    // cognate class per language, -1 for no word
	Strata  []lex.Stratum
	Dbl     []string // a doublet: another concept with the same root, "" for none
}

// Compare builds the comparative table over living languages (each
// represented by its centre community). core restricts to Swadesh-207.
func Compare(s *sim.Sim, core bool) Table {
	langs := s.LivingLanguages()
	t := Table{Protos: len(s.Protos)}
	for f := range s.Protos {
		t.Langs = append(t.Langs, s.FamilyName(uint8(f)))
	}
	for _, l := range langs {
		t.Langs = append(t.Langs, l.Name)
	}
	for _, c := range lex.Concepts() {
		if core && !c.Core() {
			continue
		}
		row := TableRow{Concept: c}
		classes := map[lex.EtymID]int{}
		add := func(v *lex.Variety) {
			l := v.Lex.At(c.ID)
			row.Strata = append(row.Strata, l.Stratum)
			dbl := ""
			if d := v.Doublets(c.ID); len(d) > 0 {
				dbl = lex.Concepts()[d[0]].Label
			}
			row.Dbl = append(row.Dbl, dbl)
			if l.Absent() {
				row.Forms = append(row.Forms, "")
				row.Cog = append(row.Cog, -1)
				return
			}
			k, ok := classes[l.Etym]
			if !ok {
				k = len(classes)
				classes[l.Etym] = k
			}
			row.Forms = append(row.Forms, phon.RenderWord(l.Form))
			row.Cog = append(row.Cog, k)
		}
		for _, p := range s.Protos {
			add(p.Variety)
		}
		for _, l := range langs {
			add(s.CentreVariety(l))
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// JSON encodes the table. Loans and doublets are listed sparsely per row:
// "loans": [[language index, "loan"|"substrate", donor, tick]], "dbl":
// [[language index, other concept]].
func (t Table) JSON(s *sim.Sim) []byte {
	var j JSON
	j.Obj().KI("protos", int64(t.Protos)).K("langs").Arr()
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
		j.EndArr().K("loans").Arr()
		for i, st := range r.Strata {
			if st.Loan() {
				j.Arr().I(int64(i)).S(stratumKind(st)).S(s.DonorName(st)).I(int64(st.Tick)).EndArr()
			}
		}
		j.EndArr().K("dbl").Arr()
		for i, d := range r.Dbl {
			if d != "" {
				j.Arr().I(int64(i)).S(d).EndArr()
			}
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
	inv := v.Inventory()
	for i, lab := range phon.Labels(inv) {
		if inv[i].Has(phon.Syllabic) {
			vs = append(vs, lab)
		} else {
			cs = append(cs, lab)
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
		if !structural(in) || e.Lo != e.Hi && !(e.Lo == 1 && e.Hi == 5) {
			continue
		}
		if e.Lo == e.Hi && e.Lo != 1 && e.Lo != 5 {
			continue // diffusion extension steps
		}
		if in.Kind == history.Gram && !gramTook(v, in) {
			continue // adopted, but the variety had gone another way
		}
		j.Obj().KI("tick", int64(e.Tick)).KU("innov", uint64(in.ID)).KS("label", in.Label()).KS("rule", in.Src).
			KS("kind", in.Kind.String()).End()
	}
	j.EndArr()
	var lexCount int
	for _, id := range history.InnovSet(v) {
		if s.Reg.Get(id).Kind == history.Lexical {
			lexCount++
		}
	}
	j.KI("replacements", int64(lexCount))
	stratumJSON(&j, s, v)
	writingJSON(&j, s, s.ByHex[l.Centre])
	j.End()
	return j.Bytes()
}

// stratumJSON lists a variety's loan strata: per donor, how many of its
// words survive, when the first and last arrived, and a few examples.
func stratumJSON(j *JSON, s *sim.Sim, v *lex.Variety) {
	type layer struct {
		st          lex.Stratum
		n           int
		first, last int32
		ex          []string
	}
	var layers []*layer
	native, absent := 0, 0
	for i := 0; i < v.Lex.Len(); i++ {
		l := v.Lex.At(lex.ConceptID(i))
		if l.Absent() {
			absent++
			continue
		}
		if !l.Stratum.Loan() {
			native++
			continue
		}
		var ly *layer
		for _, x := range layers {
			if x.st.Kind == l.Stratum.Kind && x.st.Donor == l.Stratum.Donor {
				ly = x
			}
		}
		if ly == nil {
			ly = &layer{st: l.Stratum, first: l.Stratum.Tick, last: l.Stratum.Tick}
			layers = append(layers, ly)
		}
		ly.n++
		ly.first, ly.last = min(ly.first, l.Stratum.Tick), max(ly.last, l.Stratum.Tick)
		if len(ly.ex) < 4 {
			ly.ex = append(ly.ex, lex.Concepts()[i].Label)
		}
	}
	slices.SortStableFunc(layers, func(a, b *layer) int { return b.n - a.n })
	j.KI("native", int64(native)).KI("unknown", int64(absent)).K("strata").Arr()
	for _, ly := range layers {
		j.Obj().KS("kind", stratumKind(ly.st)).KS("donor", s.DonorName(ly.st)).KI("count", int64(ly.n)).
			KI("first", int64(ly.first)).KI("last", int64(ly.last)).K("examples").Arr()
		for _, e := range ly.ex {
			j.S(e)
		}
		j.EndArr().End()
	}
	j.EndArr()
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
	j.K("curve").Arr()
	for _, a := range in.Curve {
		j.Arr().I(int64(a.Applied)).I(int64(a.Reached)).EndArr()
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
			KS("field", c.Field)
		if i := lex.TrackedIndex(c.ID); i >= 0 {
			j.KS("pos", lex.Tracked()[i].POS.String())
		}
		j.End()
	}
	j.EndArr()
	return j.Bytes()
}

// CellsJSON lists paradigm cell labels per part of speech.
func CellsJSON() []byte {
	var j JSON
	j.Obj()
	for _, p := range []morph.POS{morph.Noun, morph.Verb} {
		j.K(p.String()).Arr()
		for _, c := range morph.Cells(p) {
			j.S(c.Label)
		}
		j.EndArr()
	}
	j.End()
	return j.Bytes()
}

// IntelJSON is the pairwise intelligibility (per-mille) of living languages,
// each represented by its centre variety, plus the language threshold.
func IntelJSON(s *sim.Sim) []byte {
	ls := s.LivingLanguages()
	var j JSON
	j.Obj().KI("threshold", int64(s.Threshold().Permille())).K("langs").Arr()
	for _, l := range ls {
		langJSON(&j, l)
	}
	j.EndArr().K("m").Arr()
	for _, a := range ls {
		j.Arr()
		for _, b := range ls {
			j.I(int64(s.Intel(s.CentreVariety(a), s.CentreVariety(b)).Permille()))
		}
		j.EndArr()
	}
	j.EndArr().End()
	return j.Bytes()
}

// structural reports changes that define subgroups: sound changes,
// grammaticalization steps and word-order changes (not single words).
func structural(in *history.Innovation) bool {
	return in.Kind == history.Sound || in.Kind == history.Gram || in.Kind == history.Order
}

// gramTook reports whether a grammaticalization step is reflected in v (its
// slot has reached at least that stage with that source).
func gramTook(v *lex.Variety, in *history.Innovation) bool {
	e := v.M.Exp[in.Gram.Slot]
	return e.Source == in.Gram.Source && e.Stage >= in.Gram.To
}

// PlaceJSON describes a hex's place name: its form today in the community
// living there, what it meant in the language that coined it, and how the
// community's sound changes reshaped it.
func PlaceJSON(s *sim.Sim, hex int32) []byte {
	var j JSON
	if hex < 0 || int(hex) >= len(s.Places) || !s.World.Hexes[hex].Land() {
		return j.Null().Bytes()
	}
	p := &s.Places[hex]
	by := s.FamilyName(p.Fam)
	if p.Sub >= 0 {
		by = s.Subs[p.Sub].Name
	}
	j.Obj().KS("original", phon.RenderWord(phon.Word{Segs: p.Form.Segs})).KS("gloss", p.Gloss).KS("namedBy", by).KB("substrate", p.Sub >= 0).
		KS("originalName", phon.Capitalize(phon.Romanize(p.Form)))
	if w, steps, ok := s.PlaceName(hex); ok {
		c := s.ByHex[hex]
		j.KS("form", phon.RenderWord(w)).KS("name", phon.Capitalize(phon.Romanize(w))).KI("since", int64(c.Founded)).
			KS("adopted", phon.RenderWord(c.Name))
		if c.Lang >= 0 {
			j.KS("lang", s.Langs[c.Lang].Name)
		}
		if c.Polity >= 0 && s.Polities[c.Polity].Alive() {
			p := s.Polities[c.Polity]
			j.KS("polity", p.Name).KB("capital", p.Capital == hex)
			if c.Outcome != 0 {
				j.KS("rule", c.Outcome.String())
			}
		}
		j.K("steps").Arr()
		for _, st := range steps {
			j.Obj().KI("tick", int64(st.Tick)).KU("innov", uint64(st.Innov)).KS("label", st.Label).
				KS("before", phon.RenderWord(st.Before)).KS("after", phon.RenderWord(st.After)).End()
		}
		j.EndArr()
	}
	j.End()
	return j.Bytes()
}

// writingJSON describes how a language is written, if it is: the script,
// the standard and diglossia, the mean spelling gap and the words whose
// spelling has drifted furthest from their sound.
func writingJSON(j *JSON, s *sim.Sim, c *pop.Community) {
	p := writtenBy(s, c)
	if p == nil {
		return
	}
	o := p.Script
	capV := s.ByHex[p.Capital].Variety
	j.K("writing").Obj().KS("polity", p.Name).KS("script", o.Type.String()).KI("since", int64(o.Since)).
		KI("reformed", int64(o.Reformed)).KS("from", o.From).KB("classical", p.Classical).KI("gap", int64(o.MeanGap(capV)))
	if p.Standard != nil {
		j.KI("standard", int64(p.StandardTick))
	}
	type ex struct {
		c   lex.ConceptID
		gap int
	}
	var xs []ex
	for _, con := range lex.Concepts() {
		l := capV.Lex.At(con.ID)
		if con.Core() && !l.Absent() {
			xs = append(xs, ex{con.ID, o.Gap(con.ID, l)})
		}
	}
	slices.SortStableFunc(xs, func(a, b ex) int { return b.gap - a.gap })
	j.K("examples").Arr()
	for _, x := range xs[:min(8, len(xs))] {
		j.Obj().KS("concept", lex.Concepts()[x.c].Label).KS("spelling", o.SpellingOf(x.c, capV.Lex.At(x.c))).
			KS("form", phon.RenderWord(capV.Lex.At(x.c).Form)).KS("now", o.Write(capV.Lex.At(x.c).Form)).KI("gap", int64(x.gap)).End()
	}
	j.EndArr().End()
}

// ScenariosJSON lists the starter scenarios.
func ScenariosJSON() []byte {
	var j JSON
	j.Arr()
	for _, st := range scenario.Starters() {
		sc, err := scenario.Read(st.Src)
		if err != nil {
			continue
		}
		j.Obj().KS("id", st.ID).KS("name", sc.Name).KS("summary", sc.Summary).KS("watch", sc.Watch).KI("ticks", int64(sc.Ticks)).
			KI("families", int64(max(1, len(sc.Families)))).End()
	}
	j.EndArr()
	return j.Bytes()
}

// ScenarioJSON describes the run's scenario (null without one).
func ScenarioJSON(s *sim.Sim) []byte {
	var j JSON
	sc := s.Scenario
	if sc == nil {
		return j.Null().Bytes()
	}
	j.Obj().KS("name", sc.Name).KS("summary", sc.Summary).KS("watch", sc.Watch).KI("ticks", int64(sc.Ticks)).K("events").Arr()
	for _, e := range sc.Events {
		j.Obj().KI("tick", int64(e.Tick)).KS("do", e.Do).End()
	}
	j.EndArr().End()
	return j.Bytes()
}

// CampaignJSON is the steering game's state (null in the sandbox): the
// player's polity, influence, goals and score; with detail, also action
// costs and the changes spreading in the player's land.
func CampaignJSON(s *sim.Sim, detail bool) []byte {
	var j JSON
	cp := s.Camp
	if cp == nil {
		return j.Null().Bytes()
	}
	p := s.Polities[cp.Polity]
	goals, score := s.CampaignStatus()
	j.Obj().KI("polity", int64(p.ID)).KS("name", p.Name).KB("alive", p.Alive()).KI("capital", int64(p.Capital)).
		KI("influence", int64(cp.Influence)).KI("earned", int64(cp.Earned)).KI("spent", int64(cp.Spent)).KI("score", score).
		KB("script", p.Script != nil).KB("standard", p.Standard != nil).KB("seafaring", p.Seafaring).K("goals").Arr()
	for _, g := range goals {
		j.Obj().KS("kind", g.Goal.Kind).KI("n", int64(g.Goal.N)).KI("value", int64(g.Value)).KB("met", g.Met).KS("text", g.Describe).End()
	}
	j.EndArr()
	if detail {
		j.K("costs").Obj()
		for _, k := range []string{"script", "standard", "seafaring", "route", "encourage", "suppress", "expand", "conquer"} {
			j.KI(k, sim.ActionCost[k])
		}
		j.End().K("innovations").Arr()
		ids, applied, steer := s.TerritoryInnovations()
		for i, id := range ids {
			in := s.Reg.Get(id)
			j.Obj().KU("id", uint64(id)).KS("label", in.Label()).KS("kind", in.Kind.String()).KS("rule", in.Src).
				KI("applied", int64(applied[i])).KI("steer", int64(steer[i])).End()
		}
		j.EndArr().KI("members", int64(memberCount(s, p.ID)))
	}
	j.End()
	return j.Bytes()
}

func memberCount(s *sim.Sim, id int32) int {
	n := 0
	for _, c := range s.Communities() {
		if c.Polity == id {
			n++
		}
	}
	return n
}
