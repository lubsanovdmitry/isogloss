// Package history owns innovations, how they change varieties, etymology
// traces (by replaying a variety's history), the event log and the family
// tree built from shared innovations.
package history

import (
	"isogloss/engine/lex"
	"isogloss/engine/morph"
	"isogloss/engine/phon"
	"isogloss/engine/rules"
)

// Kind of innovation.
type Kind uint8

const (
	Sound Kind = iota
	Lexical
	Semantic // M3
	Morph    // M2
	Gram     // M2
	Order    // M2
	Loan     // M3
	Spelling // M4
	Syllable // the syllable template loosens or tightens
)

var kindNames = [...]string{"sound", "lexical", "semantic", "morph", "gram", "order", "loan", "spelling", "syllable"}

func (k Kind) String() string { return kindNames[k] }

// LexChange gives a concept a new word: a coinage or meaning shift
// (Lexical) or a loan (Loan).
type LexChange struct {
	Concept lex.ConceptID
	Form    phon.Word // the word as coined or borrowed at the origin
	Freq    uint8
	Etym    lex.EtymID
	How     string // "shift from 'tree'", "compound 'water'+'big'", "new root"
	// From: for a meaning shift, the concept whose word takes over; each
	// adopting variety uses its own form of that word. -1 otherwise.
	From int32
	// Gap: the change only fills a gap (a concept with no word yet).
	Gap bool
	// Stratum of the new word: native, or the donor and tick of a loan.
	// Inside a language a loan spreads as a loan, keeping its stratum.
	Stratum lex.Stratum
	// Source is the donor's form of a loan before nativization.
	Source phon.Word
	// Keep: foreign segments survive nativization (heavy bilingualism).
	Keep bool
}

// SemChange: Word's word gains Sense as an emerging extra meaning. It
// applies where the variety's word for Word still has root Etym.
type SemChange struct {
	Word, Sense lex.ConceptID
	Etym        lex.EtymID
}

// GramChange moves a slot's exponent one stage along the grammaticalization
// cycle. It applies only where the variety's exponent is at the previous
// stage with the same source (neighbours may have gone another way).
type GramChange struct {
	Slot   morph.Slot
	To     morph.Stage // Periphrasis (a source is recruited), Fixed, Clitic or Affix
	Source int32       // concept ID, or -1 for an inherited exponent
	Prefix bool        // Periphrasis: the construction precedes its host
	Renew  bool        // Periphrasis: replaces a free construction already in use
}

// OrderChange sets a word-order parameter.
type OrderChange struct {
	Param morph.Param
	Value uint8
}

// Innovation is one change, born in one community, spreading by the wave
// model. IDs start at 1 and index Registry.
type Innovation struct {
	ID       uint32
	Kind     Kind
	Origin   int32 // hex
	Tick     int32
	Template string // template name, or "user" for interventions
	Family   string
	Src      string // rule source (Sound)
	Rule     *rules.Rule
	Diffuse  string     // "", "freq-first", "rare-first"
	Stress   int8       // -1, or the phon.StressRule this change installs
	Lex      *LexChange // Lexical and Loan
	Sem      *SemChange
	Gram     *GramChange
	Target   lex.ConceptID // Morph: the lexeme analogy regularizes
	Order    *OrderChange
	Syll     *SyllChange
	Rate     int32 // local logistic growth rate, signed Q16
	Life     int32 // ticks during which the innovation actively spreads
	Forced   bool
	// Curve records adoption at the end of each tick while the innovation
	// is live; Curve[0] is tick Tick (the tick it was proposed).
	Curve []Adoption
}

// SyllChange widens or narrows the syllable template: how many consonants
// may begin (Onset) or end (Coda) a syllable, by ±1. Narrowing repairs
// the words that no longer fit (epenthesis or deletion, by the variety's
// repair strategy), as when a language comes to allow only open syllables.
type SyllChange struct {
	Onset, Coda int8
}

// Apply returns template t changed by sc (onsets 1..3, as every language
// allows CV; codas 0..3; a minimum never exceeds its maximum).
func (sc SyllChange) Apply(t phon.Template) phon.Template {
	t.MaxOnset = max(1, min(3, t.MaxOnset+int(sc.Onset)))
	t.MaxCoda = max(0, min(3, t.MaxCoda+int(sc.Coda)))
	t.MinOnset = min(t.MinOnset, t.MaxOnset)
	t.MinCoda = min(t.MinCoda, t.MaxCoda)
	return t
}

// Label describes the change.
func (sc SyllChange) Label() string {
	part, d := "onsets", sc.Onset
	if sc.Coda != 0 {
		part, d = "codas", sc.Coda
	}
	if d > 0 {
		return "syllables allow longer " + part
	}
	return "syllables allow shorter " + part + " only"
}

// Adoption is how far an innovation has got at one tick: communities that
// applied it, and communities where it is present at any level.
type Adoption struct {
	Applied, Reached uint16
}

// Label is a short human description.
func (in *Innovation) Label() string {
	switch in.Kind {
	case Sound:
		return in.Template
	case Lexical:
		return "new word for '" + lex.Concepts()[in.Lex.Concept].Label + "' (" + in.Lex.How + ")"
	case Loan:
		return "'" + lex.Concepts()[in.Lex.Concept].Label + "' " + in.Lex.How
	case Semantic:
		cs := lex.Concepts()
		return "the word for '" + cs[in.Sem.Word].Label + "' comes to mean '" + cs[in.Sem.Sense].Label + "' too"
	case Gram:
		return in.Gram.Label()
	case Morph:
		return "analogy regularizes '" + lex.Concepts()[in.Target].Label + "'"
	case Order:
		return in.Order.Param.Name() + " becomes " + in.Order.Param.Values()[in.Order.Value]
	case Syllable:
		return in.Syll.Label()
	}
	return in.Kind.String()
}

// Registry maps innovation IDs to innovations.
type Registry struct {
	All []*Innovation // All[id-1]
}

// Add registers an innovation and assigns its ID.
func (r *Registry) Add(in *Innovation) *Innovation {
	in.ID = uint32(len(r.All) + 1)
	r.All = append(r.All, in)
	return in
}

// Get returns the innovation with the given ID.
func (r *Registry) Get(id uint32) *Innovation { return r.All[id-1] }

// Root records where a coined etymon came from.
type Root struct {
	ID     lex.EtymID
	Tick   int32
	Origin int32
	How    string
	// Compound roots record their parts (modifier, head), so another
	// language can translate them part by part (a calque).
	Compound  bool
	Mod, Head lex.ConceptID
}

// Label describes a grammaticalization step.
func (g *GramChange) Label() string {
	info := g.Slot.Info()
	src := "an old marker"
	if g.Source >= 0 {
		src = "'" + lex.Concepts()[g.Source].Label + "'"
	}
	switch g.To {
	case morph.Periphrasis:
		return src + " starts to mark the " + info.Name
	case morph.Fixed:
		return "the " + info.Name + " with " + src + " becomes a fixed construction"
	case morph.Clitic:
		return "the " + info.Name + " marker " + src + " becomes a clitic"
	case morph.Affix:
		side := "suffix"
		if g.Prefix {
			side = "prefix"
		}
		return "the " + info.Name + " clitic " + src + " becomes a " + side
	}
	return "grammatical change"
}
