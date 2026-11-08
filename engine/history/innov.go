// Package history owns innovations, how they change varieties, etymology
// traces (by replaying a variety's history), the event log and the family
// tree built from shared innovations.
package history

import (
	"isogloss/engine/lex"
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
)

var kindNames = [...]string{"sound", "lexical", "semantic", "morph", "gram", "order", "loan", "spelling"}

func (k Kind) String() string { return kindNames[k] }

// LexChange replaces a concept's word.
type LexChange struct {
	Concept lex.ConceptID
	Form    phon.Word
	Freq    uint8
	Etym    lex.EtymID
	How     string // "shift from 'tree'", "compound 'water'+'big'", "new root"
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
	Diffuse  string // "", "freq-first", "rare-first"
	Stress   int8   // -1, or the phon.StressRule this change installs
	Lex      *LexChange
	Rate     int32 // local logistic growth rate, signed Q16
	Life     int32 // ticks during which the innovation actively spreads
	Forced   bool
}

// Label is a short human description.
func (in *Innovation) Label() string {
	switch in.Kind {
	case Sound:
		return in.Template
	case Lexical:
		return "new word for '" + lex.Concepts()[in.Lex.Concept].Label + "' (" + in.Lex.How + ")"
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
}
