package lex

const chunkBits = 5
const chunkSize = 1 << chunkBits

type chunk [chunkSize]Lexeme

// Lexicon is a copy-on-write array of lexemes indexed by ConceptID. Clones
// share chunks; writing to a shared chunk copies just that chunk, so a
// one-word change costs one chunk rather than the whole lexicon.
type Lexicon struct {
	chunks []*chunk
	owned  []bool
	n      int
}

// NewLexicon allocates an owned lexicon of n entries.
func NewLexicon(n int) Lexicon {
	k := (n + chunkSize - 1) / chunkSize
	x := Lexicon{chunks: make([]*chunk, k), owned: make([]bool, k), n: n}
	for i := range x.chunks {
		x.chunks[i] = new(chunk)
		x.owned[i] = true
	}
	return x
}

// Len is the number of entries.
func (x *Lexicon) Len() int { return x.n }

// At returns entry i. The lexeme must not be modified; use Set.
func (x *Lexicon) At(i ConceptID) *Lexeme {
	return &x.chunks[i>>chunkBits][i&(chunkSize-1)]
}

// Set stores entry i, copying its chunk first if it is shared.
func (x *Lexicon) Set(i ConceptID, l Lexeme) {
	k := int(i >> chunkBits)
	if !x.owned[k] {
		c := *x.chunks[k]
		x.chunks[k] = &c
		x.owned[k] = true
	}
	x.chunks[k][i&(chunkSize-1)] = l
}

// Clone returns a lexicon sharing all chunks with x. Neither may write to a
// shared chunk without copying, so x loses ownership too.
func (x *Lexicon) Clone() Lexicon {
	for i := range x.owned {
		x.owned[i] = false
	}
	return Lexicon{chunks: append([]*chunk(nil), x.chunks...), owned: make([]bool, len(x.chunks)), n: x.n}
}
