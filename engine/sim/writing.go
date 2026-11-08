package sim

import (
	"isogloss/engine/contact"
	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/pop"
	"isogloss/engine/rng"
	"isogloss/engine/society"
)

// Writing rates, per-mille per tick.
const (
	borrowScriptPerm = 80  // a polity near one that writes takes up its script
	scriptReach      = 10  // ... if their capitals are this close
	standardPerm     = 40  // a polity that writes fixes a standard
	reformPerm       = 40  // a spelling reform, when spelling has drifted far
	reformGap        = 300 // mean spelling gap (per mille) that invites reform
	levelPermille    = 40  // a community levels one word to its capital's
	learnedPermille  = 30  // under diglossia, a community takes a learned loan
	vernacularPerm   = 15  // the vernacular replaces a classical standard
)

// writing: polities take up scripts (rarely inventing one, usually from a
// neighbour), fix standards, reform spelling, slide into diglossia when
// the spoken language leaves the standard behind, and return to the
// vernacular. Communities level words toward their capital's, and under
// diglossia borrow learned words from the classical language.
func (s *Sim) writing() {
	if !s.Cfg.Society {
		return
	}
	r := rng.New(s.Seed, rng.Key("writing"), uint64(s.Tick))
	for _, p := range s.Polities {
		if !p.Alive() {
			continue
		}
		capC := s.ByHex[p.Capital]
		v := capC.Variety
		if p.Script == nil {
			s.adoptScript(r, p, v)
			continue
		}
		p.Script.Update(v)
		if p.Standard == nil && r.Permille(standardPerm) {
			p.Standard, p.StandardTick = v, s.Tick
			s.log(history.Event{Kind: "standard", Hex: p.Capital, Lang: capC.Lang, Text: "the speech of " + p.Name + " is fixed in writing as a standard"})
		}
		if s.Tick%4 != 0 {
			continue
		}
		if p.Standard != nil && !p.Classical && s.Intel(v, p.Standard) < s.Threshold() {
			p.Classical = true
			s.log(history.Event{Kind: "diglossia", Hex: p.Capital, Lang: capC.Lang, Text: "Classical " + p.Name + " is written but no longer spoken"})
		} else if p.Classical && r.Permille(vernacularPerm*4) {
			p.Standard, p.StandardTick, p.Classical = v, s.Tick, false
			p.Script.Remap(v, s.Tick)
			s.log(history.Event{Kind: "vernacular", Hex: p.Capital, Lang: capC.Lang, Text: p.Name + " starts writing the spoken language"})
		} else if p.Script.MeanGap(v) > reformGap && r.Permille(reformPerm*4) {
			p.Script.Remap(v, s.Tick)
			s.log(history.Event{Kind: "reform", Hex: p.Capital, Lang: capC.Lang, Text: p.Name + " reforms its spelling"})
		}
	}
	s.conservative = s.conservative[:0]
	for _, p := range s.Polities {
		if p.Alive() && p.Standard != nil {
			s.conservative = append(s.conservative, p.Capital)
		}
	}
}

func (s *Sim) adoptScript(r *rng.Stream, p *society.Polity, v *lex.Variety) {
	var donor *society.Polity
	for _, q := range s.Polities {
		if q.Alive() && q.Script != nil && s.World.Distance(q.Capital, p.Capital) <= scriptReach {
			donor = q
			break
		}
	}
	switch {
	case donor != nil && r.Permille(borrowScriptPerm):
		t := donor.Script.Type
		if t == society.Abjad && r.Permille(300) {
			t = society.Alphabet // vowels get letters of their own, as in Greek
		}
		p.Script = society.NewOrthography(t, v, s.Tick)
		p.Script.From = donor.Name
		s.log(history.Event{Kind: "script", Hex: p.Capital, Lang: p.Lang, Text: p.Name + " takes up the " + t.String() + " of " + donor.Name})
	case donor == nil && len(s.members(p)) >= 6 && s.Tick-p.Founded >= 4 && r.Permille(s.Cfg.ScriptRate):
		t := society.ScriptType(r.Weighted([]uint32{30, 30, 40}))
		p.Script = society.NewOrthography(t, v, s.Tick)
		s.log(history.Event{Kind: "script", Hex: p.Capital, Lang: p.Lang, Text: p.Name + " invents writing: a " + t.String()})
	}
}

// proposeStandard: a community under a capital that writes levels words to
// the capital's speech, and under diglossia borrows learned words from the
// classical standard.
func (s *Sim) proposeStandard(r *rng.Stream, c *pop.Community) {
	p := s.polityOf(c)
	if p == nil || p.Standard == nil || c.Hex == p.Capital {
		return
	}
	capC := s.ByHex[p.Capital]
	if capC.Lang != c.Lang {
		return
	}
	if r.Permille(levelPermille) {
		con := lex.ConceptID(r.IntN(lex.NumConcepts()))
		if l, k := c.Variety.Lex.At(con), capC.Variety.Lex.At(con); !k.Absent() && l.Etym != k.Etym {
			if lc, kind := s.takeWord(r, c, capC, con, false); lc != nil {
				lc.How = "levelled to the speech of " + p.Name
				s.newInnov(r, &history.Innovation{Kind: kind, Lex: lc, Stress: -1}, c.Hex)
			}
		}
	}
	if p.Classical && r.Permille(learnedPermille) {
		std := p.Standard
		if con, ok := s.pickLoan(r, c.Variety, std); ok {
			src := std.Lex.At(con)
			lc := &history.LexChange{Concept: con, Form: contact.Nativize(src.Form, c.Variety, false), Freq: c.Variety.Lex.At(con).Freq,
				Etym: src.Etym, From: -1, Source: src.Form, Stratum: lex.Stratum{Kind: lex.Learned, Donor: p.ID, Tick: s.Tick},
				How: "learned loan from Classical " + p.Name + " ⟨" + p.Script.SpellingOf(con, src) + "⟩"}
			s.newInnov(r, &history.Innovation{Kind: history.Loan, Lex: lc, Stress: -1}, c.Hex)
		}
	}
}
