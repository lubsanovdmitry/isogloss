package sim

import (
	"slices"

	"isogloss/engine/fx"

	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/phon"
	"isogloss/engine/pop"
	"isogloss/engine/rng"
	"isogloss/engine/tmpl"
)

// Word-replacement proposals per 100 000 community-ticks, by stability class.
var lexRatePer100k = [6]int{0, 8, 25, 60, 120, 240}

// seedLevel is the adoption level an innovation starts with at its origin.
const seedLevelPermille = 200

// simplifying families are favoured by large, connected communities.
var simplifying = map[string]bool{"merger": true, "cluster": true, "erosion": true, "length": true}

// register records an innovation that is taken community by community
// rather than spreading as a wave.
func (s *Sim) register(in *history.Innovation, origin int32) *history.Innovation {
	in.Origin, in.Tick, in.Life = origin, s.Tick, 0
	return s.Reg.Add(in)
}

func (s *Sim) newInnov(r *rng.Stream, in *history.Innovation, origin int32) {
	in.Origin, in.Tick = origin, s.Tick
	in.Rate = signedQ16(int32(r.IntN(1000)) - 250) // −0.25 .. 0.75
	taken := in.Lex != nil && (in.Kind == history.Loan && in.Lex.Stratum.Kind == lex.Borrowed || in.Lex.Gap)
	if taken {
		// Loans from a prestigious neighbour and words for new things are
		// taken up by fashion and travel further: rate 0.15 .. 0.85, 8–20
		// ticks.
		in.Rate = signedQ16(int32(r.IntN(700)) + 150)
		in.Life = 8 + int32(r.IntN(13))
	}
	in.Life = 5 + int32(r.IntN(11))
	s.Reg.Add(in)
	if in.Kind == history.Sound {
		s.ap.Track(in.ID)
	}
	l := &liveInnov{in: in, wave: pop.NewWave(len(s.World.Hexes))}
	l.wave.Level[origin] = fx.FromPermille(seedLevelPermille)
	if taken {
		l.wave.Level[origin] = fx.One // the borrowing community uses the word
	}
	s.live = append(s.live, l)
}

// propose lets every community put forward new innovations.
func (s *Sim) propose() {
	s.invent()
	for _, c := range s.ByHex {
		if c == nil {
			continue
		}
		r := rng.New(s.Seed, rng.Key("propose"), uint64(s.Tick), uint64(c.Hex))
		if r.Permille(s.Cfg.SoundRate) {
			if cand := s.pickSound(r, c); cand != nil {
				s.newInnov(r, &history.Innovation{
					Kind: history.Sound, Template: cand.T.Name, Family: cand.T.Family,
					Src: cand.Src, Rule: cand.Rule, Diffuse: cand.T.Diffuse, Stress: cand.T.Stress,
				}, c.Hex)
			}
		}
		s.proposeGrammar(r, c)
		press := c.Variety.Pressure()
		for _, con := range lex.Concepts() {
			if c.Variety.Lex.At(con.ID).Absent() {
				continue // gaps are filled by invention and contact
			}
			rate := lexRatePer100k[con.Stab] * int(s.Cfg.LexRate) / 1000
			if con.Taboo {
				rate *= 3
			}
			if _, ok := press.DriftTo(con.ID); ok {
				rate *= driftBoost
			}
			if _, ok := press.ClashWith(con.ID); ok {
				rate = max(rate, clashRatePer100k*int(s.Cfg.LexRate)/1000)
			}
			if r.IntN(100000) >= rate {
				continue
			}
			if lc, kind := s.coin(r, c, con.ID, true); lc != nil {
				s.newInnov(r, &history.Innovation{Kind: kind, Lex: lc, Stress: -1}, c.Hex)
			}
		}
		s.proposeContact(r, c)
		s.proposeStandard(r, c)
	}
}

// pickSound samples candidate sound changes for a community and picks one by
// weight: template frequency (adjusted for community size and recency) to
// choose templates, then words affected and inventory pressure per candidate.
func (s *Sim) pickSound(r *rng.Stream, c *pop.Community) *tmpl.Candidate {
	lib := tmpl.Library()
	v := c.Variety
	inv := v.Inventory()

	// Recently fired templates in this variety (decay factor d_recent).
	recent := map[string]bool{}
	for h, n := v.Hist, 0; h != nil && n < 3; h = h.Parent {
		e := h.E
		if e.Kind != lex.EntryInnov {
			continue
		}
		if in := s.Reg.Get(e.Innov); in.Kind == history.Sound {
			recent[in.Template] = true
			n++
		}
	}
	links := len(s.contacts[c.Hex])
	small := c.Pop < 300 && links <= 2
	large := c.Pop > 1000 && links >= 4
	var mean uint64
	for _, t := range lib {
		mean += uint64(t.Weight)
	}
	mean /= uint64(len(lib))
	w := make([]uint32, len(lib))
	for i, t := range lib {
		x := uint64(t.Weight) * 1000
		if small { // flatten toward the mean: rare changes become likelier
			xi, m := int64(x), int64(mean*1000)
			xi += (m - xi) * int64(s.Cfg.SmallBias) / 2000
			x = uint64(max(xi, 1))
		}
		if large && simplifying[t.Family] {
			x = x * uint64(1000+s.Cfg.LargeBias) / 1000
		}
		if recent[t.Name] {
			x = x * 300 / 1000
		}
		w[i] = uint32(x / 1000)
	}

	var cands []*tmpl.Candidate
	var scores []uint32
	for k := 0; k < s.Cfg.Candidates; k++ {
		ti := r.Weighted(w)
		if ti < 0 {
			break
		}
		cand, ok := lib[ti].Instantiate(r, inv)
		if !ok {
			continue
		}
		cands = append(cands, cand)
		scores = append(scores, s.evaluate(cand, v, inv))
	}
	i := r.Weighted(scores)
	if i < 0 {
		return nil
	}
	return cands[i]
}

// evaluate scores a candidate (per-mille) on the variety's core vocabulary:
// changes must affect enough words; mergers that create homophones are
// dispreferred; typologically rare outputs are dispreferred; collapsing
// small vowel systems is resisted and moving into a vacated slot (chain
// shift) is boosted. Changes that leave no vowels or too few consonants
// are rejected.
func (s *Sim) evaluate(cand *tmpl.Candidate, v *lex.Variety, inv []phon.FeatureSet) uint32 {
	affected, coreN := 0, 0
	oldForms := map[uint64]int{}
	newForms := map[uint64]int{}
	newSegs := map[phon.FeatureSet]bool{}
	for i := 0; i < v.Lex.Len(); i++ {
		if !lex.Concepts()[i].Core() {
			continue
		}
		coreN++
		l := v.Lex.At(lex.ConceptID(i))
		oldForms[l.Hash]++
		w, ch := cand.Rule.Apply(l.Form, &v.Phonotax)
		h := l.Hash
		if ch {
			affected++
			h = lex.FormHash(w)
		} else {
			w = l.Form
		}
		newForms[h]++
		for _, sg := range w.Segs {
			newSegs[sg.F] = true
		}
	}
	if affected < 3 {
		return 0
	}
	score := uint64(1000)
	if affected*100 > coreN*60 {
		score = score * 600 / 1000
	}
	homo := func(m map[uint64]int) int {
		n := 0
		for _, c := range m { //lint:ordered summing is order-independent
			if c > 1 {
				n += c
			}
		}
		return n
	}
	created := homo(newForms) - homo(oldForms)
	limit := coreN * 3 / 100
	switch {
	case created > limit:
		score = score * 150 / 1000
	case created > 0:
		score = score * uint64(1000-created*100) / 1000
	}
	// New phonemes: typicality from PHOIBLE; chain-shift boost.
	inInv := func(f phon.FeatureSet) bool {
		_, ok := slices.BinarySearchFunc(inv, f, cmpFeat)
		return ok
	}
	vowelsAfter, consAfter := 0, 0
	newSorted := make([]phon.FeatureSet, 0, len(newSegs))
	for f := range newSegs { //lint:ordered sorted below
		newSorted = append(newSorted, f)
	}
	slices.SortFunc(newSorted, cmpFeat)
	vowelsBefore := 0
	for _, f := range inv {
		if f.Has(phon.Syllabic) {
			vowelsBefore++
		}
	}
	for _, f := range newSorted {
		if f.Has(phon.Syllabic) {
			vowelsAfter++
		} else {
			consAfter++
		}
		if inInv(f) {
			continue
		}
		typ := uint64(min(max(phon.Frequency(f)*1000/2000, 100), 1000))
		score = score * typ / 1000
		if s.Tick-v.VacTick <= 4 && slices.Contains(v.Vacated, f) {
			score = score * 3
		}
	}
	if vowelsAfter < 2 || consAfter < 5 {
		return 0
	}
	if vowelsAfter < vowelsBefore {
		switch {
		case vowelsBefore <= 4:
			score = score * 300 / 1000
		case vowelsBefore >= 7:
			score = score * 1500 / 1000
		}
	}
	return uint32(min(score, 1<<30))
}

func cmpFeat(a, b phon.FeatureSet) int {
	switch {
	case a.Val < b.Val:
		return -1
	case a.Val > b.Val:
		return 1
	case a.Mask < b.Mask:
		return -1
	case a.Mask > b.Mask:
		return 1
	}
	return 0
}

// coin builds a new word for a concept: a meaning shift along a CLICS edge
// (the neighbour's word gains this sense), a compound of native words, a
// fresh root or, with allowLoan, a loan from a foreign neighbour. Taboo
// concepts favour euphemistic compounds. A word another word is already
// gaining as a sense is most likely replaced by it; a word that clashes
// with a homophone says so in its etymology.
func (s *Sim) coin(r *rng.Stream, c *pop.Community, con lex.ConceptID, allowLoan bool) (*history.LexChange, history.Kind) {
	v := c.Variety
	cs := lex.Concepts()
	cur := v.Lex.At(con)
	press := v.Pressure()
	edges := lex.Colex(con)
	wShift, wComp, wRoot, wLoan := uint32(20), uint32(25), uint32(25), uint32(0)
	if len(edges) == 0 {
		wShift = 0
	}
	if cs[con].Taboo {
		wComp, wRoot = 70, 5
	}
	if cs[con].Category != "Person/Thing" {
		wComp = 0 // only things are named by compounds
	}
	var foreign []pop.Contact
	if allowLoan {
		foreign = s.foreign(c)
		if len(foreign) > 0 {
			wLoan = 25
		}
	}
	drift, drifting := press.DriftTo(con)
	if drifting {
		wShift = 400
	}
	why := ""
	if w, ok := press.ClashWith(con); ok {
		why = ", after it came to sound like '" + cs[w].Label + "'"
	}
	switch r.Weighted([]uint32{wShift, wComp, wRoot, wLoan}) {
	case 0:
		var from lex.ConceptID
		if drifting && r.Permille(800) {
			from = drift
		} else {
			ew := make([]uint32, len(edges))
			for i, e := range edges {
				if !v.Lex.At(e.To).Absent() {
					ew[i] = uint32(e.Families)
				}
			}
			i := r.Weighted(ew)
			if i < 0 {
				return nil, 0
			}
			from = edges[i].To
		}
		src := v.Lex.At(from)
		if src.Etym == cur.Etym || src.Absent() {
			return nil, 0
		}
		return &history.LexChange{Concept: con, Form: src.Form.Clone(), Freq: cur.Freq, Etym: src.Etym, From: int32(from),
			Stratum: src.Stratum, How: "shift from '" + cs[from].Label + "'" + why}, history.Lexical
	case 1:
		h, m, ok := compoundParts(r, con)
		if !ok {
			return nil, 0
		}
		hw, mw := v.Lex.At(h).Form, v.Lex.At(m).Form
		if v.Lex.At(h).Absent() || v.Lex.At(m).Absent() || len(hw.Segs)+len(mw.Segs) > 10 {
			return nil, 0
		}
		w := phon.Word{Segs: append(append([]phon.Segment(nil), mw.Segs...), hw.Segs...)}
		for i := range w.Segs {
			w.Segs[i].Stress = 0
		}
		w.Bounds = []phon.Boundary{{At: len(mw.Segs), Kind: phon.BMorpheme}}
		how := "compound '" + cs[m].Label + "'+'" + cs[h].Label + "'"
		if cs[con].Taboo {
			how = "euphemism " + how
		}
		return &history.LexChange{Concept: con, Form: w, Freq: cur.Freq, Etym: s.newRoot(c.Hex, how), From: -1, How: how + why}, history.Lexical
	case 2:
		w := lex.NewRoot(r, v, cur.Freq)
		return &history.LexChange{Concept: con, Form: w, Freq: cur.Freq, Etym: s.newRoot(c.Hex, "new root"), From: -1, How: "new root" + why}, history.Lexical
	default:
		k := foreign[r.IntN(len(foreign))]
		d := s.ByHex[k.To]
		if d.Variety.Lex.At(con).Absent() {
			return nil, 0
		}
		lc, kind := s.takeWord(r, c, d, con, false)
		if lc != nil {
			lc.How += why
		}
		return lc, kind
	}
}

func (s *Sim) newRoot(origin int32, how string) lex.EtymID {
	id := s.nextRoot
	s.nextRoot++
	s.Roots = append(s.Roots, history.Root{ID: id, Tick: s.Tick, Origin: origin, How: how})
	return id
}

// Modifiers that commonly classify things in compounds ("blackbird",
// "big-river"); taboo animals favour descriptions ("the brown one", "the
// old one").
var (
	classifiers = []string{"big", "small", "old", "new", "good", "bad", "black", "white", "red", "long", "round", "sharp"}
	euphemisms  = []string{"old", "big", "black", "yellow", "good"}
)

// compoundParts picks a head and a modifier for a compound naming con. Only
// things are compounded. The head is a colexified neighbour (weighted by how
// many families colexify them), or failing that a thing from the same
// semantic field; the modifier is a classifying property.
func compoundParts(r *rng.Stream, con lex.ConceptID) (head, mod lex.ConceptID, ok bool) {
	cs := lex.Concepts()
	if cs[con].Category != "Person/Thing" {
		return 0, 0, false
	}
	var heads []lex.ConceptID
	var hw []uint32
	for _, e := range lex.Colex(con) {
		if cs[e.To].Category == "Person/Thing" {
			heads = append(heads, e.To)
			hw = append(hw, uint32(e.Families))
		}
	}
	if len(heads) == 0 {
		for _, x := range cs {
			if x.ID != con && x.Field == cs[con].Field && x.Category == "Person/Thing" {
				heads = append(heads, x.ID)
				hw = append(hw, 1)
			}
		}
	}
	if len(heads) == 0 {
		return 0, 0, false
	}
	names := classifiers
	if cs[con].Taboo {
		names = euphemisms
	}
	var mods []lex.ConceptID
	for _, n := range names {
		if id, found := lex.ConceptByLabel(n); found && id != con {
			mods = append(mods, id)
		}
	}
	if len(mods) == 0 {
		return 0, 0, false
	}
	return heads[r.Weighted(hw)], mods[r.IntN(len(mods))], true
}
