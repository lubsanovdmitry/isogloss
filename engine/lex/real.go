package lex

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"

	"isogloss/engine/data"
	"isogloss/engine/internal/ord"
	"isogloss/engine/morph"
	"isogloss/engine/phon"
	"isogloss/engine/rng"
)

// RealInfo describes a real-world proto-language shipped with the engine
// (engine/data/protos/<id>.tsv).
type RealInfo struct {
	ID, Name, Summary, Source string
	Forms                     int  // concepts with a reconstructed or attested form
	Attested                  bool // written down (Latin), not reconstructed
}

// realSpec is a parsed table.
type realSpec struct {
	RealInfo
	template string
	stress   phon.StressRule
	repair   phon.RepairMode
	epenth   phon.FeatureSet // zero: the default choice
	order    morph.Order
	extra    []phon.FeatureSet // phonemes outside the listed forms
	slots    []realSlot
	forms    []realForm
}

type realSlot struct {
	slot   morph.Slot
	stage  morph.Stage
	prefix bool
	mat    []phon.Segment // bound exponents
	source ConceptID      // free exponents
}

type realForm struct {
	c    ConceptID
	w    phon.Word
	same ConceptID // the form is the word of another concept ("=label")
	ok   bool      // same is set
}

var (
	realOnce  sync.Once
	realSpecs map[string]*realSpec
	realErr   error
)

func loadReal() {
	realSpecs = map[string]*realSpec{}
	tables := data.Protos()
	for _, id := range ord.Keys(tables) {
		sp, err := parseReal(id, tables[id])
		if err != nil {
			realErr = errors.Join(realErr, err)
			continue
		}
		realSpecs[id] = sp
	}
}

// RealProtos lists the shipped real proto-languages, by name.
func RealProtos() []RealInfo {
	realOnce.Do(loadReal)
	var out []RealInfo
	for _, sp := range realSpecs { //lint:ordered (sorted below)
		out = append(out, sp.RealInfo)
	}
	slices.SortFunc(out, func(a, b RealInfo) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// RealByID returns a shipped proto-language's description.
func RealByID(id string) (RealInfo, bool) {
	realOnce.Do(loadReal)
	sp, ok := realSpecs[id]
	if !ok {
		return RealInfo{}, false
	}
	return sp.RealInfo, true
}

// realLoadError reports tables that failed to parse (for tests).
func realLoadError() error {
	realOnce.Do(loadReal)
	return realErr
}

func parseReal(id, src string) (*realSpec, error) {
	sp := &realSpec{RealInfo: RealInfo{ID: id}, template: "(C)(C)V(C)(C)", stress: phon.StressInitial}
	fail := func(line []string, msg string) error {
		return errors.New("protos/" + id + ".tsv: " + strings.Join(line, " ") + ": " + msg)
	}
	seen := map[ConceptID]bool{}
	var order []string
	for _, f := range data.Lines(src) {
		if strings.HasPrefix(f[0], "@") {
			if len(f) < 2 {
				return nil, fail(f, "a header line needs a value")
			}
			switch f[0] {
			case "@name":
				sp.Name = f[1]
			case "@summary":
				sp.Summary = f[1]
			case "@source":
				sp.Source = f[1]
			case "@attested":
				sp.Attested = f[1] == "yes"
			case "@template":
				sp.template = f[1]
			case "@stress":
				i := slices.Index([]string{"initial", "penult", "final", "latin", "lexical"}, f[1])
				if i < 0 {
					return nil, fail(f, "stress is initial, penult, final, latin or lexical")
				}
				sp.stress = []phon.StressRule{phon.StressInitial, phon.StressPenult, phon.StressFinal, phon.StressLatin, phon.StressLexical}[i]
			case "@repair":
				if f[1] == "deletion" {
					sp.repair = phon.RepairDeletion
				}
			case "@epenthetic":
				w, err := phon.ParseWord(f[1])
				if err != nil || len(w.Segs) != 1 {
					return nil, fail(f, "one vowel")
				}
				sp.epenth = w.Segs[0].F
			case "@extra":
				for _, s := range f[1:] {
					w, err := phon.ParseWord(s)
					if err != nil || len(w.Segs) != 1 {
						return nil, fail(f, "one segment per field: "+s)
					}
					sp.extra = append(sp.extra, w.Segs[0].F)
				}
			case "@order":
				order = f[1:]
			case "@slot":
				s, err := parseSlot(f)
				if err != nil {
					return nil, fail(f, err.Error())
				}
				sp.slots = append(sp.slots, s)
			default:
				return nil, fail(f, "unknown header")
			}
			continue
		}
		c, ok := ConceptByLabel(f[0])
		if !ok || len(f) < 2 {
			return nil, fail(f, "unknown concept or no form")
		}
		if seen[c] {
			return nil, fail(f, "listed twice")
		}
		seen[c] = true
		if t, ok := strings.CutPrefix(f[1], "="); ok {
			o, ok := ConceptByLabel(t)
			if !ok {
				return nil, fail(f, "unknown concept "+t)
			}
			sp.forms = append(sp.forms, realForm{c: c, same: o, ok: true})
			continue
		}
		w, err := phon.ParseWord(f[1])
		if err != nil {
			return nil, fail(f, err.Error())
		}
		sp.forms = append(sp.forms, realForm{c: c, w: w})
	}
	if sp.Name == "" {
		return nil, errors.New("protos/" + id + ".tsv: no @name")
	}
	if len(order) != int(morph.NumParams) {
		return nil, errors.New("protos/" + id + ".tsv: @order needs " + strconv.Itoa(int(morph.NumParams)) + " values")
	}
	for p := morph.Param(0); p < morph.NumParams; p++ {
		i := slices.Index(p.Values(), strings.ReplaceAll(order[p], "-", "–"))
		if i < 0 {
			return nil, errors.New("protos/" + id + ".tsv: " + p.Name() + " is one of " + strings.Join(p.Values(), ", "))
		}
		sp.order[p] = uint8(i)
	}
	for _, f := range sp.forms {
		if f.ok && !seen[f.same] {
			return nil, errors.New("protos/" + id + ".tsv: " + Concepts()[f.c].Label + " points to an unlisted concept")
		}
	}
	sp.Forms = len(sp.forms)
	return sp, nil
}

// parseSlot reads "@slot SLOT STAGE POSITION MATERIAL": POSITION is prefix
// or suffix (before or after its host); MATERIAL is an affix or clitic's
// IPA, or the source concept of a periphrasis or fixed construction.
func parseSlot(f []string) (realSlot, error) {
	var s realSlot
	i := -1
	for k := morph.Slot(0); k < morph.NumSlots; k++ {
		if k.String() == f[1] {
			i = int(k)
		}
	}
	if i < 0 {
		return s, errors.New("unknown slot")
	}
	s.slot = morph.Slot(i)
	if len(f) != 5 {
		return s, errors.New("needs a stage, a position (prefix or suffix) and material")
	}
	switch f[3] {
	case "prefix", "before":
		s.prefix = true
	case "suffix", "after":
	default:
		return s, errors.New("position is prefix or suffix")
	}
	switch f[2] {
	case "affix", "clitic":
		s.stage = morph.Affix
		if f[2] == "clitic" {
			s.stage = morph.Clitic
		}
		w, err := phon.ParseWord(f[4])
		if err != nil {
			return s, err
		}
		s.mat = w.Segs
		for i := range s.mat {
			s.mat[i].Stress = 0
		}
	case "periphrasis", "fixed":
		s.stage = morph.Periphrasis
		if f[2] == "fixed" {
			s.stage = morph.Fixed
		}
		c, ok := ConceptByLabel(f[4])
		if !ok {
			return s, errors.New("unknown source concept " + f[4])
		}
		s.source = c
	default:
		return s, errors.New("stage is affix, clitic, periphrasis or fixed")
	}
	return s, nil
}

// realProto builds a proto-language from a shipped table. Concepts the
// table leaves out are coined from the language's own sounds (Proto.Coined);
// cultural items it leaves out are unknown, as in generated languages.
func realProto(r *rng.Stream, opt ProtoOptions, sp *realSpec) *Proto {
	p := &Proto{freqOf: map[phon.FeatureSet]int{}, Name: sp.Name, Real: sp.ID, Attested: sp.Attested}
	v := &Variety{Lex: NewLexicon(NumConcepts())}
	// Inventory, weighted by use in the listed forms.
	for _, f := range sp.forms {
		for _, s := range f.w.Segs {
			p.addReal(s.F, 10)
		}
	}
	for _, f := range sp.extra {
		p.addReal(f, 5)
	}
	v.Phonotax.Template, _ = phon.ParseTemplate(sp.template)
	for _, f := range p.Vowels {
		v.Phonotax.Template.LongV = v.Phonotax.Template.LongV || f.Has(phon.Long)
	}
	v.Phonotax.SClusters = v.Phonotax.Template.MaxOnset >= 2
	v.Stress, v.Repair = sp.stress, sp.repair
	p.Variety = v
	v.Epenth = phon.Segment{F: sp.epenth}
	if sp.epenth == (phon.FeatureSet{}) {
		v.Epenth = phon.Segment{F: p.epenthetic()}
	}
	p.maxSyl = opt.MaxSyllables

	given := map[ConceptID]*realForm{}
	for i := range sp.forms {
		given[sp.forms[i].c] = &sp.forms[i]
	}
	seen := map[uint64]bool{}
	finish := func(w phon.Word) phon.Word {
		w = w.Clone()
		if v.Stress == phon.StressLexical {
			phon.CleanStress(&w, &v.Phonotax)
			phon.AssignStress(&w, v.Stress)
		} else {
			phon.Resyllabify(&w, &v.Phonotax)
			phon.AssignStress(&w, v.Stress)
		}
		return w
	}
	for _, f := range sp.forms {
		if !f.ok {
			seen[FormHash(finish(f.w))] = true
		}
	}
	p.Coined = make([]bool, NumConcepts())
	cs := Concepts()
	for _, c := range cs {
		etym := FamilyRoot(opt.Family, c.ID)
		g := given[c.ID]
		var w phon.Word
		switch {
		case g != nil && g.ok:
			continue // filled below, once its source is set
		case g != nil:
			w = finish(g.w)
		case c.Role == RoleCulture:
			v.Set(c.ID, Lexeme{Concept: c.ID, Freq: c.Freq, Stratum: Native, Hash: FormHash(phon.Word{})})
			continue
		default:
			for try := 0; try < 40; try++ {
				w = p.root(r, c.Freq)
				if h := FormHash(w); !seen[h] {
					seen[h] = true
					break
				}
			}
			p.Coined[c.ID] = true
		}
		v.Set(c.ID, Lexeme{Concept: c.ID, Form: w, Freq: c.Freq, Stratum: Native, Etym: etym, Hash: FormHash(w)})
	}
	for _, f := range sp.forms {
		if f.ok {
			src := v.Lex.At(f.same)
			v.Set(f.c, Lexeme{Concept: f.c, Form: src.Form, Freq: cs[f.c].Freq, Stratum: Native, Etym: src.Etym, Hash: src.Hash})
		}
	}

	// Grammar: word order and exponents from the table; slots it leaves
	// out have none (they may grammaticalize later).
	m := &v.M
	m.Order = sp.order
	for i := range m.Exp {
		m.Exp[i].Source = -1
	}
	mats := make([][]phon.Segment, morph.NumSlots)
	for _, s := range sp.slots {
		e := &m.Exp[s.slot]
		e.Stage, e.Prefix = s.stage, s.prefix
		if s.stage.Free() {
			e.Source = int32(s.source)
		} else {
			mats[s.slot] = s.mat
		}
	}
	p.buildParadigms(mats)
	v.Family = opt.Family
	v.Key = 0x1505 ^ uint64(opt.Family)<<40
	return p
}

func (p *Proto) addReal(f phon.FeatureSet, n int) {
	// Syllabic consonants (PIE *r̥) are kept out of the pools new roots
	// are coined from: they arise from their non-syllabic partners.
	if _, ok := p.freqOf[f]; !ok {
		switch {
		case f.Has(phon.Syllabic) && f.Has(phon.Consonantal):
		case f.Has(phon.Syllabic):
			p.Vowels = append(p.Vowels, f)
		default:
			p.Consonants = append(p.Consonants, f)
		}
	}
	p.freqOf[f] += n
}
