// Package lex holds concepts, lexemes and varieties, and generates
// proto-languages.
package lex

import (
	"strconv"
	"sync"

	"isogloss/engine/data"
)

// ConceptID is a dense index into Concepts().
type ConceptID uint16

// Role says where a concept comes from.
type Role uint8

const (
	RoleSwadesh Role = iota
	RoleGram         // grammaticalization source
	RoleCore         // ordinary vocabulary outside the Swadesh list
	RoleCulture      // enters the world through events
)

// Concept is one meaning tracked by every variety.
type Concept struct {
	ID          ConceptID
	Concepticon int
	Label       string // short English label, unique
	Gloss       string // Concepticon gloss
	Role        Role
	Field       string // Concepticon semantic field
	Category    string // Concepticon ontological category
	Freq        uint8  // 1 (most frequent) .. 5
	Stab        uint8  // 1 (very stable) .. 5
	Taboo       bool
	FieldID     uint8 // dense index of Field
}

// Core reports whether the concept is on the Swadesh-207 list.
func (c *Concept) Core() bool { return c.Role == RoleSwadesh }

// Edge is a CLICS colexification link.
type Edge struct {
	To       ConceptID
	Families uint16
}

var (
	conceptOnce sync.Once
	concepts    []Concept
	byLabel     map[string]ConceptID
	byCID       map[int]ConceptID
	colex       [][]Edge
)

func loadConcepts() {
	byLabel = map[string]ConceptID{}
	byCID = map[int]ConceptID{}
	fields := map[string]uint8{}
	for _, f := range data.Lines(data.Concepts) {
		cid, _ := strconv.Atoi(f[0])
		freq, _ := strconv.Atoi(f[6])
		stab, _ := strconv.Atoi(f[7])
		c := Concept{
			ID: ConceptID(len(concepts)), Concepticon: cid, Label: f[1], Gloss: f[2],
			Field: f[4], Category: f[5], Freq: uint8(freq), Stab: uint8(stab), Taboo: f[8] == "1",
		}
		switch f[3] {
		case "gram":
			c.Role = RoleGram
		case "core":
			c.Role = RoleCore
		case "culture":
			c.Role = RoleCulture
		}
		fid, ok := fields[c.Field]
		if !ok {
			fid = uint8(len(fields))
			fields[c.Field] = fid
		}
		c.FieldID = fid
		byLabel[c.Label] = c.ID
		byCID[cid] = c.ID
		concepts = append(concepts, c)
	}
	colex = make([][]Edge, len(concepts))
	for _, f := range data.Lines(data.Colex) {
		a, _ := strconv.Atoi(f[0])
		b, _ := strconv.Atoi(f[1])
		fam, _ := strconv.Atoi(f[2])
		ia, okA := byCID[a]
		ib, okB := byCID[b]
		if !okA || !okB {
			continue
		}
		colex[ia] = append(colex[ia], Edge{ib, uint16(fam)})
		colex[ib] = append(colex[ib], Edge{ia, uint16(fam)})
	}
}

// Concepts returns all concepts, ordered by Concepticon ID.
func Concepts() []Concept {
	conceptOnce.Do(loadConcepts)
	return concepts
}

// NumConcepts is len(Concepts()).
func NumConcepts() int { return len(Concepts()) }

// ConceptByLabel looks up a concept by its label ("water").
func ConceptByLabel(label string) (ConceptID, bool) {
	Concepts()
	id, ok := byLabel[label]
	return id, ok
}

// ConceptByConcepticon looks up a concept by Concepticon ID.
func ConceptByConcepticon(cid int) (ConceptID, bool) {
	Concepts()
	id, ok := byCID[cid]
	return id, ok
}

// Colex returns the CLICS neighbours of a concept.
func Colex(c ConceptID) []Edge {
	Concepts()
	return colex[c]
}
