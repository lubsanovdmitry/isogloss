package render

import (
	"errors"
	"strings"

	"isogloss/engine/lex"
	"isogloss/engine/morph"
	"isogloss/engine/phon"
	"isogloss/engine/sim"
	"isogloss/engine/world"
)

// The storyteller (an LLM in the browser) never touches the simulation: the
// engine supplies material (name candidates, an era's events, the words a
// myth may use) and renders what comes back. Names are picked from the
// candidates, so every name obeys the language's own phonotactics.

// Candidate is a possible native name with what it means.
type Candidate struct {
	Form  string // IPA
	Name  string // romanized, capitalized
	Gloss string
}

// candidates builds names from word combinations of a variety: each entry
// is a list of concept labels joined as one word (a compound), in the
// language's modifier order for two-part names.
func candidates(v *lex.Variety, extra []Candidate, combos [][]string) []Candidate {
	out := append([]Candidate(nil), extra...)
	seen := map[string]bool{}
	for _, c := range out {
		seen[c.Name] = true
	}
	for _, combo := range combos {
		var segs []phon.Segment
		var gloss []string
		ok := true
		parts := combo
		if len(parts) == 2 && v.M.Order[morph.AdjN] == 1 {
			parts = []string{parts[1], parts[0]} // noun–modifier order
		}
		for _, p := range parts {
			id, known := lex.ConceptByLabel(p)
			if !known {
				ok = false
				break
			}
			l := v.Lex.At(id)
			if l.Absent() {
				ok = false
				break
			}
			segs = append(segs, l.Form.Segs...)
		}
		for _, p := range combo {
			gloss = append(gloss, glossOf(p))
		}
		if !ok || len(segs) == 0 || len(segs) > 12 {
			continue
		}
		w := v.Finish(phon.Word{Segs: segs})
		c := Candidate{phon.RenderWord(w), phon.Capitalize(phon.Romanize(w)), strings.Join(gloss, " ")}
		if seen[c.Name] || len(c.Name) < 2 {
			continue
		}
		seen[c.Name] = true
		out = append(out, c)
		if len(out) >= 8 {
			break
		}
	}
	return out
}

// landscape is the word that best describes a hex.
func landscape(h *world.Hex) string {
	switch {
	case h.River:
		return "river"
	case h.Biome == world.Mountain:
		return "mountain"
	case h.Biome == world.Coast || h.Biome == world.Island:
		return "sea"
	case h.Biome == world.Forest:
		return "forest"
	case h.Biome == world.Desert:
		return "sand"
	}
	return "earth"
}

// LangNameCandidates offers native names for a living language's people,
// built from its centre's words: 'person', 'river people', 'good people',
// the people of its central place.
func LangNameCandidates(s *sim.Sim, id int32) ([]Candidate, error) {
	if id < 0 || int(id) >= len(s.Langs) || !s.Langs[id].Alive() {
		return nil, errors.New("no such living language")
	}
	l := s.Langs[id]
	v := s.CentreVariety(l)
	var extra []Candidate
	if w, _, ok := s.PlaceName(l.Centre); ok {
		extra = append(extra, Candidate{phon.RenderWord(w), phon.Capitalize(phon.Romanize(w)), "(the place) " + s.Places[l.Centre].Gloss})
	}
	land := landscape(&s.World.Hexes[l.Centre])
	return candidates(v, extra, [][]string{
		{"person"}, {land, "person"}, {"good", "person"}, {"old", "person"}, {"correct", "person"},
		{"sun", "person"}, {land}, {"say", "person"}, {"big", "person"},
	}), nil
}

// PolityNameCandidates offers names for a polity from its capital's words.
func PolityNameCandidates(s *sim.Sim, id int32) ([]Candidate, error) {
	if id < 0 || int(id) >= len(s.Polities) {
		return nil, errors.New("no such polity")
	}
	p := s.Polities[id]
	c := s.ByHex[p.Capital]
	if c == nil {
		return nil, errors.New("the capital is empty")
	}
	var extra []Candidate
	if w, _, ok := s.PlaceName(p.Capital); ok {
		extra = append(extra, Candidate{phon.RenderWord(w), phon.Capitalize(phon.Romanize(w)), "(the capital) " + s.Places[p.Capital].Gloss})
	}
	land := landscape(&s.World.Hexes[p.Capital])
	return candidates(c.Variety, extra, [][]string{
		{"big", "earth"}, {"king", "earth"}, {"good", "earth"}, {land, "earth"}, {"chief", "earth"}, {"new", "earth"}, {"old", "earth"},
	}), nil
}

// CandidatesJSON encodes name candidates.
func CandidatesJSON(cs []Candidate, err error) []byte {
	var j JSON
	if err != nil {
		return j.Obj().KS("error", err.Error()).End().Bytes()
	}
	j.Arr()
	for _, c := range cs {
		j.Obj().KS("form", c.Form).KS("name", c.Name).KS("gloss", c.Gloss).End()
	}
	j.EndArr()
	return j.Bytes()
}

// eraKinds are the chronicle-worthy event kinds (single sound changes are
// summarised as counts).
var eraKinds = map[string]bool{
	"split": true, "extinct": true, "polity": true, "fall": true, "conquest": true, "shift": true, "rule": true,
	"script": true, "standard": true, "diglossia": true, "vernacular": true, "reform": true, "trade": true,
	"seafaring": true, "invention": true, "migration": true, "plague": true, "campaign": true, "settle": true,
}

// EraJSON gives an era (ticks from ≤ t < to) as structured data for a
// chronicle: its events (famines counted, not listed), how many sound
// changes began, and the languages and polities at its end.
func EraJSON(s *sim.Sim, from, to int32) []byte {
	var j JSON
	j.Obj().KI("from", int64(from)*25).KI("to", int64(to)*25).KS("protoLanguage", s.ProtoName()).K("events").Arr()
	famine, sounds, n := 0, 0, 0
	for _, e := range s.Log {
		if e.Tick < from || e.Tick >= to {
			continue
		}
		switch {
		case e.Kind == "famine":
			famine++
		case e.Kind == "innovation":
			sounds++
		case eraKinds[e.Kind] && n < 80:
			n++
			j.Obj().KI("year", int64(e.Tick)*25).KS("kind", e.Kind).KS("text", e.Text)
			if e.Lang >= 0 && int(e.Lang) < len(s.Langs) {
				j.KS("language", s.Langs[e.Lang].Name)
			}
			j.End()
		}
	}
	j.EndArr().KI("famines", int64(famine)).KI("soundChangesBegun", int64(sounds)).K("languages").Arr()
	for _, l := range s.LivingLanguages() {
		j.Obj().KS("name", l.Name).KI("communities", int64(len(l.Members))).KI("since", int64(l.Born)*25)
		if l.Parent >= 0 && int(l.Parent) < len(s.Langs) {
			j.KS("splitFrom", s.Langs[l.Parent].Name)
		}
		j.End()
	}
	j.EndArr().K("polities").Arr()
	for _, p := range s.Polities {
		if p.Alive() {
			j.Obj().KS("name", p.Name).KI("founded", int64(p.Founded)*25).KB("writes", p.Script != nil).KB("seafaring", p.Seafaring).End()
		}
	}
	j.EndArr().End()
	return j.Bytes()
}

// pronounPerson maps pronoun concepts to agreement.
var pronounPerson = map[string]morph.Slot{"i": morph.P1SG, "thou": morph.P2SG, "he": morph.P3SG, "we": morph.P1PL, "you": morph.P2PL, "they": morph.P3PL}

var tenses = map[string]morph.Slot{"present": present, "past": morph.PST, "future": morph.FUT, "perfect": morph.PRF, "progressive": morph.PROG}

// MythSpecJSON lists what an abstract text may use: nouns, verbs,
// adjectives and pronouns by concept label, and the tenses.
func MythSpecJSON() []byte {
	var j JSON
	j.Obj()
	for _, part := range []struct{ key, cat string }{{"nouns", "Person/Thing"}, {"verbs", "Action/Process"}, {"adjectives", "Property"}} {
		j.K(part.key).Arr()
		for _, c := range lex.Concepts() {
			if c.Category == part.cat {
				j.S(c.Label)
			}
		}
		j.EndArr()
	}
	j.K("pronouns").Arr()
	for _, p := range []string{"i", "thou", "he", "we", "you", "they"} {
		j.S(p)
	}
	j.EndArr().K("tenses").Arr()
	for _, t := range []string{"present", "past", "future", "perfect", "progressive"} {
		j.S(t)
	}
	j.EndArr().End()
	return j.Bytes()
}

// NPSpec and ClauseSpec are an abstract text as the storyteller writes it:
// concept labels and features, no words.
type NPSpec struct {
	Noun, Pron, Adj string
	PL, Def         bool
	Gen             *NPSpec
}

type ClauseSpec struct {
	Subj          NPSpec
	Verb, Tense   string
	Obj, Dat, Loc *NPSpec
	English       string
}

func checkConcept(lbl, cat, what string) error {
	c, ok := lex.ConceptByLabel(lbl)
	if !ok {
		return errors.New("unknown " + what + " '" + lbl + "'")
	}
	if lex.Concepts()[c].Category != cat {
		return errors.New("'" + lbl + "' is not a " + what)
	}
	return nil
}

func buildNP(n *NPSpec) (*NP, error) {
	if n == nil {
		return nil, nil
	}
	out := &NP{PL: n.PL, Def: n.Def}
	switch {
	case n.Pron != "":
		if _, ok := pronounPerson[n.Pron]; !ok {
			return nil, errors.New("unknown pronoun '" + n.Pron + "'")
		}
		out.Pron = n.Pron
	case n.Noun != "":
		if err := checkConcept(n.Noun, "Person/Thing", "noun"); err != nil {
			return nil, err
		}
		out.Noun = n.Noun
	default:
		return nil, errors.New("a noun phrase needs a noun or a pronoun")
	}
	if n.Adj != "" {
		if err := checkConcept(n.Adj, "Property", "adjective"); err != nil {
			return nil, err
		}
		out.Adj = n.Adj
	}
	if n.Gen != nil {
		g, err := buildNP(n.Gen)
		if err != nil {
			return nil, err
		}
		out.Gen = g
	}
	return out, nil
}

// BuildClauses validates an abstract text and turns it into clauses.
func BuildClauses(specs []ClauseSpec) ([]Clause, error) {
	if len(specs) == 0 || len(specs) > 12 {
		return nil, errors.New("a text has 1–12 clauses")
	}
	var out []Clause
	for i, sp := range specs {
		fail := func(err error) error {
			return errors.New("clause " + string(rune('1'+i)) + ": " + err.Error())
		}
		subj, err := buildNP(&sp.Subj)
		if err != nil {
			return nil, fail(err)
		}
		if err := checkConcept(sp.Verb, "Action/Process", "verb"); err != nil {
			return nil, fail(err)
		}
		tam, ok := tenses[sp.Tense]
		if !ok {
			return nil, fail(errors.New("unknown tense '" + sp.Tense + "'"))
		}
		person := morph.P3SG
		switch {
		case subj.Pron != "":
			person = pronounPerson[subj.Pron]
		case subj.PL:
			person = morph.P3PL
		}
		c := Clause{Subj: *subj, Verb: sp.Verb, TAM: tam, Person: person, English: sp.English}
		for _, x := range []struct {
			in  *NPSpec
			out **NP
		}{{sp.Obj, &c.Obj}, {sp.Dat, &c.Dat}, {sp.Loc, &c.Loc}} {
			np, err := buildNP(x.in)
			if err != nil {
				return nil, fail(err)
			}
			*x.out = np
		}
		out = append(out, c)
	}
	return out, nil
}

// TextJSON renders clauses in every proto-language and living language,
// interlinear, in the same shape as FableJSON.
func TextJSON(s *sim.Sim, clauses []Clause) []byte {
	return textJSON(s, clauses)
}
