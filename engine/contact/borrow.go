package contact

import (
	"sync"

	"isogloss/engine/lex"
)

// Borrowability follows the usual hierarchy: nouns > verbs > adjectives >
// numerals and function words. Unstable meanings are borrowed more easily,
// cultural items most easily of all; grammatical sources are function
// words. Weights are relative, ×1000.
var (
	categoryWeight = map[string]uint32{"Person/Thing": 100, "Action/Process": 45, "Property": 30, "Number": 4, "Other": 6}
	stabWeight     = [6]uint32{0, 4, 15, 40, 80, 100}
)

var (
	borrowOnce sync.Once
	borrowW    []uint32
)

// Borrowability returns the weight of each concept as a loan candidate.
func Borrowability() []uint32 {
	borrowOnce.Do(func() {
		cs := lex.Concepts()
		borrowW = make([]uint32, len(cs))
		for i, c := range cs {
			w := categoryWeight[c.Category] * stabWeight[c.Stab]
			switch c.Role {
			case lex.RoleCulture:
				w *= 4
			case lex.RoleGram:
				w /= 3
			}
			borrowW[i] = w
		}
	})
	return borrowW
}

// substrateFields are the semantic fields substrate languages leave words
// in: local flora, fauna and landscape.
var substrateFields = map[string]bool{"Animals": true, "Agriculture and vegetation": true, "The physical world": true}

// SubstrateWeight is the weight of concept c as a substrate loan (0 when
// substrates do not supply it).
func SubstrateWeight(c lex.ConceptID) uint32 {
	cc := &lex.Concepts()[c]
	if !substrateFields[cc.Field] || cc.Role == lex.RoleCulture {
		return 0
	}
	return stabWeight[cc.Stab]
}
