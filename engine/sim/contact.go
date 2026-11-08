package sim

import (
	"slices"

	"isogloss/engine/contact"
	"isogloss/engine/fx"
	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/morph"
	"isogloss/engine/phon"
	"isogloss/engine/pop"
	"isogloss/engine/rng"
	"isogloss/engine/society"
	"isogloss/engine/tmpl"
)

// Rates of meaning change and contact.
const (
	// clashRatePer100k: replacement proposals per 100 000 community-ticks
	// for the less frequent word of a homophone pair.
	clashRatePer100k = 2500
	// driftBoost multiplies the replacement rate of a concept another word
	// is gaining as a sense (the competitor covers it).
	driftBoost = 5
	// gapPermille: chance per tick that a community lacking a word for a
	// cultural item its neighbours have takes one over.
	gapPermille = 500
	// loanPermille: chance per tick and foreign contact (at full contact
	// weight, from a more prestigious community) of borrowing one word.
	loanPermille = 400
	// keepPermille: chance that a loan over a strong contact keeps its
	// foreign sounds.
	keepPermille = 200
)

// placeFreq is the frequency class place names change at.
const placeFreq = 3

// setupContact generates substrate languages and every hex's place name.
// homes are the families' homeland centres; homeland marks every hex
// settled at the start.
func (s *Sim) setupContact(homes []int32, homeland []bool) {
	s.Subs, s.Region = contact.GenerateSubstrates(s.Seed, s.World, homeland, int(s.Cfg.Substrates))
	used := map[string]bool{}
	for f := range s.Protos {
		used[s.FamilyName(uint8(f))] = true
	}
	person, _ := lex.ConceptByLabel("person")
	for _, sb := range s.Subs {
		n := phon.Capitalize(phon.Romanize(sb.V.Lex.At(person).Form))
		for k := 2; used[n]; k++ {
			n = phon.Capitalize(phon.Romanize(sb.V.Lex.At(person).Form)) + string(rune('0'+k))
		}
		used[n] = true
		sb.Name = n
	}
	s.Places = make([]contact.Place, len(s.World.Hexes))
	for _, h := range s.World.Land {
		r := rng.New(s.Seed, rng.Key("place"), uint64(h))
		fam := s.nearestFamily(homes, h)
		v, sub := s.Protos[fam].Variety, int32(-1)
		if k := s.Region[h]; k >= 0 {
			v, sub = s.Subs[k].V, k
		}
		s.Places[h] = contact.NamePlace(r, v, &s.World.Hexes[h], sub)
		s.Places[h].Fam = fam
	}
	s.invented = make([]bool, lex.NumConcepts())
	for i := range s.invented {
		for _, p := range s.Protos {
			s.invented[i] = s.invented[i] || !p.Variety.Lex.At(lex.ConceptID(i)).Absent()
		}
	}
}

// takeName gives a newly settled community its place name: the
// proto-language's own in the homeland, otherwise the substrate's,
// nativized.
func (s *Sim) takeName(c *pop.Community) {
	p := &s.Places[c.Hex]
	c.NameAt = c.Variety.Hist.Len()
	if p.Sub < 0 && p.Fam == c.Variety.Family {
		c.Name = phon.Word{Segs: p.Form.Segs, Syll: p.Form.Syll} // no longer analysed as a compound
		return
	}
	c.Name = contact.Nativize(p.Form, c.Variety, false)
}

// PlaceName returns a hex's current name in the community living there and
// its history since the community took it over.
func (s *Sim) PlaceName(h int32) (phon.Word, []history.TraceStep, bool) {
	c := s.ByHex[h]
	if c == nil {
		return phon.Word{}, nil, false
	}
	w, steps := history.Evolve(s.Reg, s.ProtoOf(c.Variety), c.Variety, c.Name, int(c.NameAt), placeFreq)
	return w, steps, true
}

// DonorName names the source of a loan stratum.
func (s *Sim) DonorName(st lex.Stratum) string {
	switch st.Kind {
	case lex.Borrowed:
		if st.Donor >= 0 && int(st.Donor) < len(s.Langs) {
			return s.Langs[st.Donor].Name
		}
	case lex.Substrate:
		if st.Donor >= 0 && int(st.Donor) < len(s.Subs) {
			return s.Subs[st.Donor].Name
		}
	case lex.Learned:
		if st.Donor >= 0 && int(st.Donor) < len(s.Polities) {
			return "Classical " + s.Polities[st.Donor].Name
		}
	}
	return ""
}

// foreign lists a community's contacts that speak another language.
func (s *Sim) foreign(c *pop.Community) []pop.Contact {
	var out []pop.Contact
	for _, k := range s.contacts[c.Hex] {
		if o := s.ByHex[k.To]; o != nil && o.Lang != c.Lang && o.Lang >= 0 && c.Lang >= 0 {
			out = append(out, k)
		}
	}
	return out
}

// invent lets cultural items the proto-language had no word for appear:
// each tick an unknown item may be invented (or first adopted) in one
// community, which coins a word for it. Neighbours of the same language
// take the word over by the wave; other languages borrow it.
func (s *Sim) invent() {
	r := rng.New(s.Seed, rng.Key("invent"), uint64(s.Tick))
	cs := lex.Concepts()
	for _, con := range cs {
		if s.invented[con.ID] || !r.Permille(s.Cfg.InventRate) {
			continue
		}
		comms := s.Communities()
		w := make([]uint32, len(comms))
		for i, c := range comms {
			w[i] = uint32(c.Pop)
		}
		i := r.Weighted(w)
		if i < 0 {
			return
		}
		c := comms[i]
		lc, kind := s.coin(r, c, con.ID, false)
		if lc == nil || kind != history.Lexical {
			continue
		}
		lc.Gap = true
		lc.How = "a new thing, named by " + lc.How
		s.invented[con.ID] = true
		in := &history.Innovation{Kind: history.Lexical, Lex: lc, Stress: -1}
		s.newInnov(r, in, c.Hex) // adopted at once where invented
		s.log(history.Event{Kind: "invention", Hex: c.Hex, Innov: in.ID, Lang: c.Lang, Text: "'" + con.Label + "' first appears"})
	}
}

// proposeContact lets a community fill gaps from its neighbours and borrow
// from more prestigious foreign neighbours, and lets its words gain senses.
func (s *Sim) proposeContact(r *rng.Stream, c *pop.Community) {
	v := c.Variety
	press := v.Pressure()
	// Meaning drift, step one: a word gains a neighbouring sense.
	if r.Permille(s.Cfg.SemRate) {
		if sc := s.pickDrift(r, v); sc != nil {
			s.newInnov(r, &history.Innovation{Kind: history.Semantic, Sem: sc, Stress: -1}, c.Hex)
		}
	}
	// Gaps: take over a neighbour's word for a thing it knows (trade
	// routes reach far).
	trade := s.tradeContacts(c)
	for _, g := range press.Gaps {
		if !s.invented[g] || !r.Permille(gapPermille) {
			continue
		}
		best, bestW := int32(-1), int64(-1)
		for _, k := range append(slices.Clip(s.contacts[c.Hex]), trade...) {
			if o := s.ByHex[k.To]; o != nil && !o.Variety.Lex.At(g).Absent() {
				if w := int64(k.W) * int64(s.prestige[k.To]); w > bestW {
					best, bestW = k.To, w
				}
			}
		}
		if best >= 0 {
			if lc, kind := s.takeWord(r, c, s.ByHex[best], g, true); lc != nil {
				s.newInnov(r, &history.Innovation{Kind: kind, Lex: lc, Stress: -1}, c.Hex)
			}
		}
		break // one gap at a time
	}
	// Prestige borrowing across language boundaries (and along trade
	// routes); bilingual communities borrow three times as much.
	foreign := s.foreign(c)
	for _, k := range trade {
		if o := s.ByHex[k.To]; o.Lang != c.Lang && o.Lang >= 0 {
			foreign = append(foreign, k)
		}
	}
	for _, k := range foreign {
		pm := uint64(loanPermille) * uint64(s.Cfg.LoanRate) / 1000
		if c.Outcome == society.Bilingual || s.ByHex[k.To].Outcome == society.Bilingual {
			pm *= 3
		}
		pm = pm * uint64(k.W) / uint64(fx.FromPermille(450))
		if s.prestige[k.To] <= s.prestige[c.Hex] {
			pm = pm * 3 / 10
		}
		if !r.Permille(uint32(pm)) {
			continue
		}
		d := s.ByHex[k.To]
		if con, ok := s.pickLoan(r, v, d.Variety); ok {
			if lc, kind := s.takeWord(r, c, d, con, false); lc != nil {
				s.newInnov(r, &history.Innovation{Kind: kind, Lex: lc, Stress: -1}, c.Hex)
			}
		}
	}
}

// pickDrift picks a word and a CLICS neighbour of its concept for it to
// gain as a sense, weighted by how many families colexify the two.
func (s *Sim) pickDrift(r *rng.Stream, v *lex.Variety) *history.SemChange {
	a := lex.ConceptID(r.IntN(lex.NumConcepts()))
	la := v.Lex.At(a)
	edges := lex.Colex(a)
	if la.Absent() || la.Also != 0 || len(edges) == 0 {
		return nil
	}
	w := make([]uint32, len(edges))
	for i, e := range edges {
		if lb := v.Lex.At(e.To); !lb.Absent() && lb.Etym != la.Etym {
			w[i] = uint32(e.Families)
		}
	}
	i := r.Weighted(w)
	if i < 0 {
		return nil
	}
	return &history.SemChange{Word: a, Sense: edges[i].To, Etym: la.Etym}
}

// pickLoan picks a concept to borrow from donor variety d by borrowability,
// among those where the donor's word differs from v's.
func (s *Sim) pickLoan(r *rng.Stream, v, d *lex.Variety) (lex.ConceptID, bool) {
	bw := contact.Borrowability()
	w := make([]uint32, len(bw))
	senses := map[[2]uint64]int{} // words (root, form) by number of senses
	for i := 0; i < v.Lex.Len(); i++ {
		l := v.Lex.At(lex.ConceptID(i))
		senses[[2]uint64{uint64(l.Etym), l.Hash}]++
	}
	for i, x := range bw {
		lv, ld := v.Lex.At(lex.ConceptID(i)), d.Lex.At(lex.ConceptID(i))
		if ld.Absent() || ld.Stratum.Kind == lex.Substrate || ld.Hash == lv.Hash {
			continue
		}
		if ld.Etym == lv.Etym {
			x *= 2 // a sister language's form of the same word: dialect borrowing is easiest
			if senses[[2]uint64{uint64(lv.Etym), lv.Hash}] > 1 {
				// The native word has several senses; the sister's form
				// lets one of them have a word of its own (shirt/skirt).
				x *= 3
			}
		}
		w[i] = x
	}
	i := r.Weighted(w)
	return lex.ConceptID(i), i >= 0
}

// takeWord builds the change by which community c takes over community
// d's word for con: within one language the word spreads as it is
// (keeping its stratum); from another language it is a nativized loan.
func (s *Sim) takeWord(r *rng.Stream, c, d *pop.Community, con lex.ConceptID, gap bool) (*history.LexChange, history.Kind) {
	src := d.Variety.Lex.At(con)
	cur := c.Variety.Lex.At(con)
	if d.Lang == c.Lang {
		lc := &history.LexChange{Concept: con, Form: src.Form, Freq: cur.Freq, Etym: src.Etym, From: -1, Gap: gap,
			Stratum: src.Stratum, How: "taken over from a neighbouring dialect"}
		return lc, history.Lexical
	}
	keep := false
	for _, k := range s.contacts[c.Hex] {
		if k.To == d.Hex && k.W >= fx.FromPermille(350) {
			keep = r.Permille(keepPermille)
		}
	}
	f := contact.Nativize(src.Form, c.Variety, keep)
	if !gap && src.Etym == cur.Etym && f.Equal(cur.Form) {
		return nil, 0
	}
	donor := s.Langs[d.Lang].Name
	how := "borrowed from " + donor + " " + phon.RenderWord(src.Form)
	if gap {
		how = "a new thing, " + how
	}
	lc := &history.LexChange{Concept: con, Form: f, Freq: cur.Freq, Etym: src.Etym, From: -1, Gap: gap,
		Stratum: lex.Stratum{Kind: lex.Borrowed, Donor: d.Lang, Tick: s.Tick}, Source: src.Form, Keep: keep, How: how}
	return lc, history.Loan
}

// substratePackage: when the family settles land a substrate language is
// spoken on, its speakers shift and may leave words for local plants,
// animals and landscape, a merger of a contrast their language lacked, or
// their word order.
func (s *Sim) substratePackage(c *pop.Community) {
	if k := s.Region[c.Hex]; k >= 0 {
		s.leaveSubstrate(c, k, s.Cfg.SubstrateRate)
	}
}

// leaveSubstrate: with chance pm‰, substrate k leaves its package in
// community c, whose speakers have just shifted away from it.
func (s *Sim) leaveSubstrate(c *pop.Community, k int32, pm uint32) {
	r := rng.New(s.Seed, rng.Key("substrate-shift"), uint64(s.Tick), uint64(c.Hex))
	if !r.Permille(pm) {
		return
	}
	sb := s.Subs[k]
	v := c.Variety
	n := 1 + r.IntN(2)
	w := make([]uint32, lex.NumConcepts())
	for i := range w {
		w[i] = contact.SubstrateWeight(lex.ConceptID(i))
		if v.Lex.At(lex.ConceptID(i)).Stratum.Kind == lex.Substrate || sb.V.Lex.At(lex.ConceptID(i)).Absent() {
			w[i] = 0
		}
	}
	for j := 0; j < n; j++ {
		i := r.Weighted(w)
		if i < 0 {
			break
		}
		w[i] = 0
		con := lex.ConceptID(i)
		src := sb.V.Lex.At(con).Form
		etym := sb.V.Lex.At(con).Etym // a sister language given up: its words keep their roots
		if sb.Seed >= 0 {
			etym = s.substrateRoot(k, con, c.Hex)
		}
		lc := &history.LexChange{Concept: con, Form: contact.Nativize(src, v, false), Freq: v.Lex.At(con).Freq,
			Etym: etym, From: -1, Source: src,
			Stratum: lex.Stratum{Kind: lex.Substrate, Donor: k, Tick: s.Tick},
			How:     "from the " + sb.Name + " substrate " + phon.RenderWord(src)}
		s.newInnov(r, &history.Innovation{Kind: history.Loan, Lex: lc, Stress: -1}, c.Hex)
	}
	if r.Permille(250) {
		if in := substrateMerger(r, v, sb); in != nil {
			s.newInnov(r, in, c.Hex)
		}
	}
	if r.Permille(150) {
		p := morph.Param(r.IntN(int(morph.NumParams)))
		if val := sb.V.M.Order[p]; val != v.M.Order[p] {
			s.newInnov(r, &history.Innovation{Kind: history.Order, Order: &history.OrderChange{Param: p, Value: val}, Stress: -1,
				Template: "substrate"}, c.Hex)
		}
	}
	if sb.Seed >= 0 {
		s.log(history.Event{Kind: "shift", Hex: c.Hex, Lang: c.Lang, Text: "the " + sb.Name + " speakers here shift to the newcomers' language"})
	}
}

// substrateRoot is the root of substrate k's word for con (one root per
// substrate word, so the same substrate loan in two places is cognate).
func (s *Sim) substrateRoot(k int32, con lex.ConceptID, hex int32) lex.EtymID {
	key := [2]int32{k, int32(con)}
	if id, ok := s.subRoots[key]; ok {
		return id
	}
	id := s.newRoot(hex, s.Subs[k].Name+" substrate word")
	s.subRoots[key] = id
	return id
}

// substrateMerger builds a sound change merging a phoneme the substrate
// lacked into its nearest neighbour, if that affects enough words.
func substrateMerger(r *rng.Stream, v *lex.Variety, sb *contact.Substrate) *history.Innovation {
	inv, sinv := v.Inventory(), sb.V.Inventory()
	off := r.IntN(len(inv))
	for j := range inv {
		x := inv[(j+off)%len(inv)]
		if y := contact.Nearest(x, sinv); y == x {
			continue
		}
		y := contact.Nearest(contact.Nearest(x, sinv), inv)
		if y == x || y.Has(phon.Syllabic) != x.Has(phon.Syllabic) {
			continue
		}
		src := "rule substrate-merger:\n    " + phon.RenderFeatures(x) + " => " + phon.RenderFeatures(y) + "\n"
		rule, err := tmpl.Compile(src)
		if err != nil {
			continue
		}
		n := 0
		for i := 0; i < v.Lex.Len() && n < 3; i++ {
			if _, ch := rule.Apply(v.Lex.At(lex.ConceptID(i)).Form, &v.Phonotax); ch {
				n++
			}
		}
		if n < 3 {
			continue
		}
		return &history.Innovation{Kind: history.Sound, Template: "substrate merger (" + sb.Name + ")", Family: "substrate",
			Src: src, Rule: rule, Stress: -1}
	}
	return nil
}

// founderPermille: chance that a small group settling across the sea loses
// one of its rarer sounds (serial founder effects thin inventories).
const founderPermille = 1000

// founderEffect: a founding group may lose its rarest sound, merging it
// into the nearest other phoneme.
func (s *Sim) founderEffect(c *pop.Community) {
	if c == nil {
		return
	}
	r := rng.New(s.Seed, rng.Key("founder"), uint64(s.Tick), uint64(c.Hex))
	if !r.Permille(founderPermille) {
		return
	}
	v := c.Variety
	inv := v.Inventory()
	count := map[phon.FeatureSet]int{}
	for i := 0; i < v.Lex.Len(); i++ {
		for _, sg := range v.Lex.At(lex.ConceptID(i)).Form.Segs {
			count[sg.F]++
		}
	}
	rarest, n := phon.FeatureSet{}, 1<<30
	for _, f := range inv {
		if count[f] > 0 && count[f] < n {
			rarest, n = f, count[f]
		}
	}
	var others []phon.FeatureSet
	for _, f := range inv {
		if f != rarest && f.Has(phon.Syllabic) == rarest.Has(phon.Syllabic) {
			others = append(others, f)
		}
	}
	if n == 1<<30 || len(others) < 2 {
		return
	}
	y := contact.Nearest(rarest, others)
	src := "rule founder-loss:\n    " + phon.RenderFeatures(rarest) + " => " + phon.RenderFeatures(y) + "\n"
	rule, err := tmpl.Compile(src)
	if err != nil {
		return
	}
	s.newInnov(r, &history.Innovation{Kind: history.Sound, Template: "founder effect: " + phon.RenderFeatures(rarest) + " lost", Family: "merger",
		Src: src, Rule: rule, Stress: -1}, c.Hex)
	l := s.live[len(s.live)-1]
	l.wave.Level[c.Hex] = fx.One // the founders never had the sound,
	l.in.Life = 0                // and nobody else loses it: only their descendants
}
