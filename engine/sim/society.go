package sim

import (
	"slices"

	"isogloss/engine/contact"
	"isogloss/engine/fx"
	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/phon"
	"isogloss/engine/pop"
	"isogloss/engine/rng"
	"isogloss/engine/society"
	"isogloss/engine/world"
)

// Society rates and thresholds (per-mille per tick unless noted).
const (
	polityPop      = 900 // population a community needs to found a polity
	polityRadius   = 3   // no two capitals closer than this
	maxReach       = 7   // a polity's hexes stay within this distance of its capital
	annexPermille  = 450 // a polity takes in an unaffiliated same-language neighbour
	conquerPerm    = 250 // ... a foreign-language one (scaled by strength)
	warPermille    = 120 // ... a hex of another polity (scaled by strength)
	ruleTicks      = 4   // foreign rule lasts this long before its outcome is decided
	shiftPermille  = 150 // under elite dominance, a community shifts per tick
	faminePermille = 5
	plaguePermille = 25 // per tick, somewhere
	migratePerm    = 25 // a full community with no empty neighbour sends migrants
	seafarePerm    = 15 // a polity with a coastal capital takes to the sea
	hubPop         = 900
	maxHubs        = 8
)

// Superstrate fields: what conquerors who shift to the local language leave
// behind.
var superstrateFields = map[string]bool{"Warfare and hunting": true, "Social and political relations": true, "Law": true, "Religion and belief": true}

// polityOf returns the community's living polity, or nil.
func (s *Sim) polityOf(c *pop.Community) *society.Polity {
	if c == nil || c.Polity < 0 {
		return nil
	}
	p := s.Polities[c.Polity]
	if !p.Alive() {
		return nil
	}
	return p
}

// society runs the world step's social part: disasters, polities forming,
// expanding, ruling and falling, migration, trade routes and seafaring.
func (s *Sim) society() {
	if !s.Cfg.Society {
		return
	}
	r := rng.New(s.Seed, rng.Key("society"), uint64(s.Tick))
	s.disasters(r)
	s.formPolities(r)
	s.expandPolities(r)
	s.ruleOutcomes(r)
	s.collapsePolities(r)
	s.migrate(r)
	s.takeToSea(r)
	if s.Tick%8 == 1 {
		s.planRoutes()
	}
}

// disasters: famine hits single communities (harsh land more often); plague
// starts somewhere populous and hits everyone within three hexes.
func (s *Sim) disasters(r *rng.Stream) {
	for _, c := range s.ByHex {
		if c == nil {
			continue
		}
		pm := uint32(faminePermille)
		switch s.World.Hexes[c.Hex].Biome {
		case world.Desert, world.Steppe, world.Mountain:
			pm *= 2
		}
		if r.Permille(pm) {
			c.Pop -= c.Pop * 4 / 10
			s.log(history.Event{Kind: "famine", Hex: c.Hex, Lang: c.Lang, Text: "famine"})
		}
	}
	if !r.Permille(plaguePermille) {
		return
	}
	comms := s.Communities()
	w := make([]uint32, len(comms))
	for i, c := range comms {
		w[i] = uint32(c.Pop)
		if s.hub[c.Hex] {
			w[i] *= 3
		}
	}
	i := r.Weighted(w)
	if i < 0 {
		return
	}
	o := comms[i]
	for _, c := range comms {
		if d := s.World.Distance(o.Hex, c.Hex); d <= 3 {
			c.Pop -= c.Pop * int32(4-d) / 10
		}
	}
	s.log(history.Event{Kind: "plague", Hex: o.Hex, Lang: o.Lang, Text: "plague breaks out"})
}

// formPolities: a large community far from other capitals may found a
// polity, taking in its unaffiliated same-language neighbours.
func (s *Sim) formPolities(r *rng.Stream) {
	for _, c := range s.ByHex {
		if c == nil || c.Polity >= 0 || c.Pop < polityPop || !r.Permille(s.Cfg.PolityRate) {
			continue
		}
		near := false
		for _, p := range s.Polities {
			if p.Alive() && s.World.Distance(p.Capital, c.Hex) < polityRadius {
				near = true
			}
		}
		if near {
			continue
		}
		s.foundPolity(c)
	}
}

func (s *Sim) foundPolity(c *pop.Community) *society.Polity {
	name, _, _ := s.PlaceName(c.Hex)
	p := &society.Polity{ID: int32(len(s.Polities)), Name: phon.Capitalize(phon.Romanize(name)), Capital: c.Hex, Founded: s.Tick, Fell: -1, Lang: c.Lang}
	s.Polities = append(s.Polities, p)
	s.join(c, p)
	for _, nb := range s.World.Hexes[c.Hex].Nbr {
		if o := s.byHex(nb); o != nil && o.Polity < 0 && o.Lang == c.Lang {
			s.join(o, p)
		}
	}
	s.log(history.Event{Kind: "polity", Hex: c.Hex, Lang: c.Lang, Text: p.Name + " becomes the seat of a polity"})
	return p
}

func (s *Sim) byHex(h int32) *pop.Community {
	if h < 0 {
		return nil
	}
	return s.ByHex[h]
}

func (s *Sim) join(c *pop.Community, p *society.Polity) {
	c.Polity, c.Joined, c.Outcome = p.ID, s.Tick, society.Undecided
}

// members lists a polity's communities in hex order.
func (s *Sim) members(p *society.Polity) []*pop.Community {
	var out []*pop.Community
	for _, c := range s.ByHex {
		if c != nil && c.Polity == p.ID {
			out = append(out, c)
		}
	}
	return out
}

// expandPolities: each polity may take one neighbouring hex per tick: by
// union with unaffiliated speakers of its language, by conquest of
// foreign-language land, or by war with another polity. Strength is total
// population; a weaker attacker rarely wins.
func (s *Sim) expandPolities(r *rng.Stream) {
	strength := map[int32]int64{}
	for _, c := range s.ByHex {
		if c != nil && c.Polity >= 0 {
			strength[c.Polity] += int64(c.Pop)
		}
	}
	for _, p := range s.Polities {
		if !p.Alive() || !r.Permille(s.Cfg.ConquestRate) {
			continue
		}
		capLang := s.ByHex[p.Capital].Lang
		var cands []int32
		for _, c := range s.members(p) {
			for _, nb := range s.World.Hexes[c.Hex].Nbr {
				if o := s.byHex(nb); o != nil && o.Polity != p.ID && s.World.Distance(p.Capital, nb) <= maxReach && !slices.Contains(cands, nb) {
					cands = append(cands, nb)
				}
			}
		}
		if len(cands) == 0 {
			continue
		}
		slices.Sort(cands)
		t := s.ByHex[cands[r.IntN(len(cands))]]
		own := strength[p.ID]
		var pm int64
		switch op := s.polityOf(t); {
		case op == nil && t.Lang == capLang:
			pm = annexPermille
		case op == nil:
			pm = conquerPerm * own / max(own+int64(t.Pop)*4, 1) * 2
		default:
			pm = warPermille * own / max(own+strength[op.ID], 1) * 2
			if t.Hex == op.Capital {
				pm /= 3
			}
		}
		if !r.Permille(uint32(min(pm, 900))) {
			continue
		}
		if op := s.polityOf(t); op != nil && t.Hex == op.Capital {
			s.fall(op, "its capital falls to "+p.Name)
		}
		s.join(t, p)
		if t.Lang != capLang {
			s.log(history.Event{Kind: "conquest", Hex: t.Hex, Lang: t.Lang, Text: p.Name + " conquers a " + s.langName(t.Lang) + "-speaking community"})
		}
	}
}

func (s *Sim) langName(l int32) string {
	if l < 0 || int(l) >= len(s.Langs) {
		return "?"
	}
	return s.Langs[l].Name
}

// ruleOutcomes decides what foreign rule does to a community once it has
// lasted ruleTicks: by the ratio of the rulers' speakers to the locals'
// within the polity. Many rulers: the locals shift (elite dominance);
// few: the rulers' language leaves only loans (elite assimilation); in
// between, both persist (bilingualism), which long rule turns into shift.
func (s *Sim) ruleOutcomes(r *rng.Stream) {
	for _, c := range s.ByHex {
		p := s.polityOf(c)
		if p == nil {
			continue
		}
		capC := s.ByHex[p.Capital]
		if c.Lang == capC.Lang || c.Lang < 0 {
			continue
		}
		ruled := s.Tick - c.Joined
		if ruled < ruleTicks {
			continue
		}
		if c.Outcome == society.Undecided || c.Outcome == society.Bilingual && ruled >= 4*ruleTicks {
			var rulers, locals int64
			for _, m := range s.members(p) {
				switch m.Lang {
				case capC.Lang:
					rulers += int64(m.Pop)
				case c.Lang:
					locals += int64(m.Pop)
				}
			}
			ratio := rulers * 1000 / max(locals, 1)
			prev := c.Outcome
			switch {
			case ratio >= 600 || prev == society.Bilingual && ratio >= 250:
				c.Outcome = society.EliteDominance
			case ratio < 250:
				c.Outcome = society.EliteAssimilate
				s.superstrate(r, c, capC)
			default:
				c.Outcome = society.Bilingual
			}
			if c.Outcome != prev {
				s.log(history.Event{Kind: "rule", Hex: c.Hex, Lang: c.Lang, Text: c.Outcome.String() + " under " + p.Name})
			}
		}
		if c.Outcome == society.EliteDominance && r.Permille(shiftPermille) {
			s.shiftLanguage(c, capC)
		}
	}
}

// superstrate: rulers who shift to the local language leave words for
// rule, war, law and religion.
func (s *Sim) superstrate(r *rng.Stream, c, ruler *pop.Community) {
	w := make([]uint32, lex.NumConcepts())
	for i, con := range lex.Concepts() {
		lv, ld := c.Variety.Lex.At(con.ID), ruler.Variety.Lex.At(con.ID)
		if superstrateFields[con.Field] && !ld.Absent() && ld.Hash != lv.Hash {
			w[i] = 10
		}
	}
	for k := 0; k < 3; k++ {
		i := r.Weighted(w)
		if i < 0 {
			return
		}
		w[i] = 0
		if lc, kind := s.takeWord(r, c, ruler, lex.ConceptID(i), false); lc != nil {
			lc.How = "superstrate: " + lc.How
			s.newInnov(r, &history.Innovation{Kind: kind, Lex: lc, Stress: -1}, c.Hex)
		}
	}
}

// shiftLanguage: community c gives up its language for the rulers'. The
// old language becomes a substrate: it may leave words, a merger and word
// order, and the place name is nativized again.
func (s *Sim) shiftLanguage(c, to *pop.Community) {
	old := c.Variety
	oldName := s.langName(c.Lang)
	name, _, _ := s.PlaceName(c.Hex)
	k := s.substrateFor(old, oldName)
	c.Variety = to.Variety
	c.Lang = to.Lang
	c.Diffusing = append([]pop.Diffusion(nil), to.Diffusing...)
	c.Name = contact.Nativize(name, c.Variety, false)
	c.NameAt = c.Variety.Hist.Len()
	c.Outcome = society.Undecided
	s.log(history.Event{Kind: "shift", Hex: c.Hex, Lang: c.Lang, Text: "the " + oldName + " speakers here shift to " + s.langName(to.Lang)})
	s.leaveSubstrate(c, k, s.Cfg.SubstrateRate*4)
}

// substrateFor registers a variety a community gave up as a substrate
// (once per language name).
func (s *Sim) substrateFor(v *lex.Variety, name string) int32 {
	for i, sb := range s.Subs {
		if sb.Name == name {
			return int32(i)
		}
	}
	s.Subs = append(s.Subs, &contact.Substrate{ID: int32(len(s.Subs)), Name: name, V: v, Seed: -1})
	return int32(len(s.Subs) - 1)
}

// collapsePolities: large, old polities break up more easily.
func (s *Sim) collapsePolities(r *rng.Stream) {
	for _, p := range s.Polities {
		if !p.Alive() {
			continue
		}
		n := int32(len(s.members(p)))
		pm := uint32(s.Cfg.CollapseRate) * uint32(4+n/4+(s.Tick-p.Founded)/4) / 1000
		if s.ByHex[p.Capital] == nil || s.ByHex[p.Capital].Polity != p.ID || r.Permille(min(pm, 300)) {
			s.fall(p, "it breaks up")
		}
	}
}

func (s *Sim) fall(p *society.Polity, why string) {
	p.Fell = s.Tick
	for _, c := range s.members(p) {
		c.Polity, c.Outcome = -1, society.Undecided
	}
	s.log(history.Event{Kind: "fall", Hex: p.Capital, Lang: p.Lang, Text: p.Name + " falls: " + why})
}

// migrate: a full community with no empty land next to it may send a group
// to the nearest empty land within six hexes (across the sea for
// seafarers).
func (s *Sim) migrate(r *rng.Stream) {
	type move struct {
		from *pop.Community
		to   int32
	}
	var moves []move
	claimed := map[int32]bool{}
	for _, c := range s.ByHex {
		if c == nil || c.Pop*10 < s.World.Hexes[c.Hex].Capacity*9 || !r.Permille(migratePerm) {
			continue
		}
		free := false
		for _, nb := range s.World.Hexes[c.Hex].Nbr {
			if nb >= 0 && s.World.Hexes[nb].Land() && s.ByHex[nb] == nil {
				free = true
			}
		}
		if free {
			continue // ordinary expansion handles it
		}
		dist, _ := society.Costs(s.World, c.Hex, s.seafarer(c), 2500)
		best, bestCost := int32(-1), int32(1<<30)
		for _, h := range s.World.Land {
			if s.ByHex[h] != nil || claimed[h] || dist[h] < 0 || s.World.Distance(c.Hex, h) > 6 {
				continue
			}
			if dist[h] < bestCost {
				best, bestCost = h, dist[h]
			}
		}
		if best >= 0 {
			claimed[best] = true
			moves = append(moves, move{c, best})
		}
	}
	for _, m := range moves {
		people := m.from.Pop / 4
		m.from.Pop -= people
		s.settle(m.to, people, m.from.Variety, m.from)
		s.log(history.Event{Kind: "migration", Hex: m.to, Lang: m.from.Lang, Text: "migrants from a crowded land settle here"})
	}
}

// seafarer reports whether a community belongs to a seafaring polity and
// lies on the coast.
func (s *Sim) seafarer(c *pop.Community) bool {
	p := s.polityOf(c)
	return p != nil && p.Seafaring && s.coastal(c.Hex)
}

func (s *Sim) coastal(h int32) bool {
	for _, nb := range s.World.Hexes[h].Nbr {
		if nb >= 0 && s.World.Hexes[nb].Biome == world.Sea {
			return true
		}
	}
	return false
}

// takeToSea: a polity whose capital is on the coast may take to the sea.
func (s *Sim) takeToSea(r *rng.Stream) {
	for _, p := range s.Polities {
		if p.Alive() && !p.Seafaring && s.coastal(p.Capital) && r.Permille(seafarePerm) {
			p.Seafaring = true
			s.log(history.Event{Kind: "seafaring", Hex: p.Capital, Lang: p.Lang, Text: p.Name + " takes to the sea"})
		}
	}
}

// planRoutes links the most populous communities (hubs, at least four hexes
// apart) to their two nearest hubs along the cheapest path.
func (s *Sim) planRoutes() {
	comms := s.Communities()
	slices.SortStableFunc(comms, func(a, b *pop.Community) int { return int(b.Pop - a.Pop) })
	var hubs []*pop.Community
	for _, c := range comms {
		if c.Pop < hubPop || len(hubs) >= maxHubs {
			break
		}
		ok := true
		for _, h := range hubs {
			if s.World.Distance(h.Hex, c.Hex) < 4 {
				ok = false
			}
		}
		if ok {
			hubs = append(hubs, c)
		}
	}
	old := map[[2]int32]bool{}
	for _, rt := range s.Routes {
		old[[2]int32{rt.A, rt.B}] = true
	}
	s.Routes = s.Routes[:0:0]
	s.hub = map[int32]bool{}
	s.routeEdge = map[[2]int32]bool{}
	s.routeLinks = map[int32][]int32{}
	for _, a := range hubs {
		type cand struct {
			b    *pop.Community
			path []int32
		}
		var cs []cand
		dist, prev := society.Costs(s.World, a.Hex, s.seafarer(a), 3000)
		for _, b := range hubs {
			if b == a {
				continue
			}
			if p := society.Trace(prev, dist, a.Hex, b.Hex); p != nil {
				cs = append(cs, cand{b, p})
			}
		}
		slices.SortStableFunc(cs, func(x, y cand) int { return int(dist[x.b.Hex] - dist[y.b.Hex]) })
		for _, c := range cs[:min(2, len(cs))] {
			x, y := a.Hex, c.b.Hex
			if x > y {
				x, y = y, x
			}
			if slices.ContainsFunc(s.Routes, func(rt society.Route) bool { return rt.A == x && rt.B == y }) {
				continue
			}
			path := c.path
			if path[0] != x {
				path = slices.Clone(path)
				slices.Reverse(path)
			}
			s.Routes = append(s.Routes, society.Route{A: x, B: y, Path: path, Since: s.Tick})
			s.hub[x], s.hub[y] = true, true
			s.routeLinks[x] = append(s.routeLinks[x], y)
			s.routeLinks[y] = append(s.routeLinks[y], x)
			for i := 0; i+1 < len(path); i++ {
				e := [2]int32{min(path[i], path[i+1]), max(path[i], path[i+1])}
				s.routeEdge[e] = true
			}
			if !old[[2]int32{x, y}] {
				n1, _, _ := s.PlaceName(x)
				n2, _, _ := s.PlaceName(y)
				s.log(history.Event{Kind: "trade", Hex: x, Lang: s.ByHex[x].Lang,
					Text: "a trade route opens between " + phon.Capitalize(phon.Romanize(n1)) + " and " + phon.Capitalize(phon.Romanize(n2))})
			}
		}
	}
}

// tradeContacts are a hub's links to the far ends of its routes: words for
// new things travel along them (Wanderwörter).
func (s *Sim) tradeContacts(c *pop.Community) []pop.Contact {
	var out []pop.Contact
	for _, h := range s.routeLinks[c.Hex] {
		if s.ByHex[h] != nil {
			out = append(out, pop.Contact{To: h, W: fx.FromPermille(250)})
		}
	}
	return out
}

// adjustContacts applies society to terrain-based contact weights: a shared
// polity and a trade route strengthen links, bilingual communities talk
// more across the language border, and seafarers reach across the sea.
func (s *Sim) adjustContacts() {
	if !s.Cfg.Society {
		return
	}
	cap := fx.FromPermille(600)
	for i, cs := range s.contacts {
		ci := s.ByHex[i]
		if ci == nil {
			continue
		}
		for k := range cs {
			cj := s.ByHex[cs[k].To]
			w := int64(cs[k].W)
			if ci.Polity >= 0 && ci.Polity == cj.Polity && s.polityOf(ci) != nil {
				w = w * 3 / 2
			}
			if s.routeEdge[[2]int32{min(int32(i), cs[k].To), max(int32(i), cs[k].To)}] {
				w *= 2
			}
			if ci.Outcome == society.Bilingual || cj.Outcome == society.Bilingual {
				w = w * 3 / 2
			}
			cs[k].W = fx.Q16(min(w, int64(cap)))
		}
	}
	for _, c := range s.ByHex {
		if c == nil || !s.seafarer(c) {
			continue
		}
		for _, h := range society.SeaReach(s.World, c.Hex, 2) {
			if s.ByHex[h] == nil {
				continue
			}
			s.addContact(c.Hex, h)
			s.addContact(h, c.Hex)
		}
	}
}

func (s *Sim) addContact(a, b int32) {
	for _, k := range s.contacts[a] {
		if k.To == b {
			return
		}
	}
	s.contacts[a] = append(s.contacts[a], pop.Contact{To: b, W: fx.FromPermille(120)})
}

// prestigeFactor: capitals, trade hubs and (later) centres of writing carry
// weight, ×1000.
func (s *Sim) prestigeFactor(c *pop.Community) int64 {
	f := int64(1000)
	if p := s.polityOf(c); p != nil {
		if p.Capital == c.Hex {
			f = f * 1500 / 1000
		} else {
			f = f * 1100 / 1000
		}
	}
	if s.hub[c.Hex] {
		f = f * 1250 / 1000
	}
	return f
}
