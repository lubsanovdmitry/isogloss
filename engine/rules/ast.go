// Package rules implements Isogloss's sound-change rule language: parsing,
// matching and application. Built-in templates and user rules share it, so
// every change the simulation makes can be printed and replayed with
// `isogloss sca`.
//
// Syntax summary:
//
//	class stop = [-cont -son -syl]      # feature matrix
//	class vl   = p t k                  # IPA list (ordered, for correspondences)
//	rule name @diffuse(freq-first):
//	    TARGET => REPLACEMENT / LEFT _ RIGHT, LEFT2 _ RIGHT2
//	    except / LEFT _ RIGHT
//
// Elements: IPA literals (a token may hold several segments: "kt"), class
// names, matrices [+f -f class ipa stress:N tone:T phon:P], lists {a b c},
// boundaries # . + =, postfix * (zero or more), (optional groups), * alone
// (deletion on the right, insertion on the left). Whitespace-separated plain
// single-segment literals on either side of => form a list, so p t k => f θ x
// is a positional mapping. C and V are predefined as [-syl] and [+syl].
package rules

import "isogloss/engine/phon"

// Class is a named segment pattern.
type Class struct {
	Name string
	Pat  *SegPat
}

// SegPat matches one segment.
type SegPat struct {
	Feat    phon.FeatureSet   // required features
	Alts    []phon.FeatureSet // if non-nil, segment features must equal one of these (ordered)
	Classes []*Class          // all must match
	Stress  int8              // -1 = any
	Tone    int8              // -1 = any
	Phon    int8              // -1 = any
	Src     string            // source text, for formatting
}

func newPat() *SegPat { return &SegPat{Stress: -1, Tone: -1, Phon: -1} }

// ElemKind distinguishes pattern elements.
type ElemKind uint8

const (
	ESeg   ElemKind = iota // consumes one segment
	EBound                 // zero-width boundary check
	EOpt                   // optional group
)

// Elem is one element of a target or environment.
type Elem struct {
	Kind  ElemKind
	Seg   *SegPat
	Star  bool // ESeg: zero or more
	Bound byte // EBound: '#', '.', '+', '='
	Group []Elem
}

// OutKind distinguishes replacement elements.
type OutKind uint8

const (
	ODelete OutKind = iota // *
	OLit                   // literal segments
	OMatrix                // feature/prosody changes to the matched segment
	OList                  // correspondence by index with a list/class target
)

// Out is one replacement element.
type Out struct {
	Kind   OutKind
	Lit    []phon.Segment   // OLit
	Delta  phon.FeatureSet  // OMatrix
	Stress int8             // OMatrix, -1 = unchanged
	Tone   int8             // OMatrix, -1 = unchanged
	Phon   int8             // OMatrix, -1 = unchanged
	List   [][]phon.Segment // OList; an empty entry means deletion
	Src    string
}

// Env is one environment: Left _ Right.
type Env struct {
	Left, Right []Elem
	Src         string
}

// Clause is one TARGET => REPLACEMENT line with its environments.
type Clause struct {
	Target []Elem // ESeg only, no Star; empty means insertion
	Repl   []Out
	Envs   []Env // any may match; none = everywhere
	Except []Env // any blocks
	Line   int
	Src    string
}

// Rule is a named, ordered list of clauses applied one after another.
type Rule struct {
	Name    string
	Diffuse string // @diffuse argument; empty if regular
	Clauses []*Clause
	Line    int
}

// File is a parsed rule file.
type File struct {
	Classes []*Class
	Rules   []*Rule
}
