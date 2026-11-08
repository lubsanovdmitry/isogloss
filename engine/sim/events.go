package sim

import (
	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/pop"
	"isogloss/engine/rng"
	"isogloss/engine/society"
)

// applyEvent carries out a scripted event or a sandbox action other than a
// forced sound change. Events that no longer make sense (a polity that has
// fallen, an empty origin) do nothing.
func (s *Sim) applyEvent(iv Intervention) {
	if !s.pay(iv) {
		return
	}
	r := rng.New(s.Seed, rng.Key("event"), uint64(s.Tick), uint64(iv.Hex+1), uint64(iv.From+1))
	var p *society.Polity
	if iv.Polity >= 0 && int(iv.Polity) < len(s.Polities) && s.Polities[iv.Polity].Alive() {
		p = s.Polities[iv.Polity]
	}
	switch iv.Kind {
	case "encourage", "suppress":
		if s.Camp != nil {
			d, verb := int8(1), "encourages"
			if iv.Kind == "suppress" {
				d, verb = -1, "frowns on"
			}
			s.Camp.steer[iv.Innov] = d
			name := "the polity"
			if p := s.Polities[s.Camp.Polity]; p.Alive() {
				name = p.Name
			}
			s.log(history.Event{Kind: "campaign", Hex: -1, Innov: iv.Innov, Lang: -1, Text: name + " " + verb + " " + s.Reg.Get(iv.Innov).Label()})
		}
	case "expand":
		s.expand(iv)
	case "collapse":
		if p != nil {
			s.fall(p, "it breaks apart")
		}
	case "plague", "famine":
		o := iv.Hex
		if o < 0 {
			o = s.homeland()
		}
		reach, hit := int32(3), int32(4)
		if iv.Kind == "famine" {
			reach, hit = 1, 5
		}
		for _, c := range s.Communities() {
			if d := s.World.Distance(o, c.Hex); d <= reach {
				c.Pop -= c.Pop * (hit - d) / 10
			}
		}
		s.log(history.Event{Kind: iv.Kind, Hex: o, Lang: s.langAt(o), Text: iv.Kind + " (scripted)"})
	case "migrate":
		s.scriptedMigration(iv)
	case "conquer":
		if p == nil {
			return
		}
		for _, c := range s.Communities() {
			if s.World.Distance(iv.Hex, c.Hex) <= 2 && c.Polity != p.ID {
				if op := s.polityOf(c); op != nil && c.Hex == op.Capital {
					s.fall(op, "its capital falls to "+p.Name)
				}
				s.join(c, p)
			}
		}
		s.log(history.Event{Kind: "conquest", Hex: iv.Hex, Lang: s.langAt(iv.Hex), Text: p.Name + " conquers the land around"})
	case "script":
		if p != nil && p.Script == nil {
			p.Script = society.NewOrthography(scriptType(iv.Rule), s.ByHex[p.Capital].Variety, s.Tick)
			s.log(history.Event{Kind: "script", Hex: p.Capital, Lang: p.Lang, Text: p.Name + " adopts writing: a " + p.Script.Type.String()})
		}
	case "standard":
		if p != nil {
			v := s.ByHex[p.Capital].Variety
			if p.Script == nil {
				p.Script = society.NewOrthography(society.Alphabet, v, s.Tick)
			}
			p.Standard, p.StandardTick, p.Classical = v, s.Tick, false
			s.log(history.Event{Kind: "standard", Hex: p.Capital, Lang: p.Lang, Text: "the speech of " + p.Name + " is fixed as a standard"})
		}
	case "seafaring":
		if p != nil {
			p.Seafaring = true
			s.log(history.Event{Kind: "seafaring", Hex: p.Capital, Lang: p.Lang, Text: p.Name + " takes to the sea"})
		}
	case "route":
		s.FixedRoutes = append(s.FixedRoutes, [2]int32{iv.Hex, iv.From})
		s.planRoutes()
	case "invent":
		con, _ := lex.ConceptByLabel(iv.Rule)
		c := s.nearestSettled(iv.Hex)
		if c == nil || !c.Variety.Lex.At(con).Absent() {
			return
		}
		if lc, kind := s.coin(r, c, con, false); lc != nil && kind == history.Lexical {
			lc.Gap = true
			lc.How = "a new thing, named by " + lc.How
			s.invented[con] = true
			in := &history.Innovation{Kind: history.Lexical, Lex: lc, Stress: -1}
			s.newInnov(r, in, c.Hex)
			s.log(history.Event{Kind: "invention", Hex: c.Hex, Innov: in.ID, Lang: c.Lang, Text: "'" + iv.Rule + "' first appears"})
		}
	}
}

func (s *Sim) langAt(h int32) int32 {
	if c := s.byHex(h); c != nil {
		return c.Lang
	}
	return -1
}

// nearestSettled returns the community at or nearest to hex h.
func (s *Sim) nearestSettled(h int32) *pop.Community {
	var best *pop.Community
	bestD := int32(1 << 30)
	for _, c := range s.Communities() {
		if d := s.World.Distance(h, c.Hex); d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

// scriptedMigration moves a third of the community nearest iv.From to the
// empty land nearest iv.Hex. If the destination is all taken, the migrants
// join the community there, which becomes bilingual.
func (s *Sim) scriptedMigration(iv Intervention) {
	from := s.nearestSettled(iv.From)
	if from == nil {
		return
	}
	best, bestD := int32(-1), int32(1<<30)
	for _, h := range s.World.Land {
		if s.ByHex[h] == nil {
			if d := s.World.Distance(iv.Hex, h); d < bestD && d <= 3 {
				best, bestD = h, d
			}
		}
	}
	people := from.Pop / 3
	from.Pop -= people
	if best >= 0 {
		s.settle(best, people, from.Variety, from)
		s.log(history.Event{Kind: "migration", Hex: best, Lang: from.Lang, Text: "a migration settles new land"})
		return
	}
	if c := s.byHex(iv.Hex); c != nil {
		c.Pop += people
		c.Outcome = society.Bilingual
		s.log(history.Event{Kind: "migration", Hex: iv.Hex, Lang: c.Lang, Text: "migrants arrive and settle among the people here"})
	}
}
