// Package sim runs the tick loop. A run is fully determined by its seed,
// config and intervention log; replay rebuilds everything else.
package sim

import (
	"slices"

	"isogloss/engine/contact"
	"isogloss/engine/fx"
	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/pop"
	"isogloss/engine/rng"
	"isogloss/engine/scenario"
	"isogloss/engine/society"
	"isogloss/engine/world"
)

// Config holds every tunable that affects a run. All values are integers.
type Config struct {
	World world.Config
	// SoundRate: per-mille chance per community per tick of proposing a sound change.
	SoundRate uint32
	// LexRate scales word-replacement rates (1000 = default).
	LexRate uint32
	// Candidates: sound-change candidates sampled per proposal.
	Candidates int
	// ClusterEvery: ticks between language clustering.
	ClusterEvery int32
	// SmallBias, LargeBias: community-size biases (per-mille sliders, 500 = moderate).
	// Small isolated communities favour rarer templates; large connected ones
	// favour simplification.
	SmallBias, LargeBias uint32
	// Threshold: intelligibility (per-mille) for two varieties to be one language; 0 = default.
	Threshold uint32
	// HomelandRadius: hexes around the homeland settled at tick 0.
	HomelandRadius int32
	// GramRate, AnalogyRate, OrderRate scale grammaticalization steps,
	// analogical levelling and word-order drift (1000 = default).
	GramRate, AnalogyRate, OrderRate uint32
	// SemRate: per-mille chance per community per tick that a word starts
	// gaining a neighbouring sense (the first step of a meaning shift).
	SemRate uint32
	// LoanRate scales borrowing between languages (1000 = default).
	LoanRate uint32
	// InventRate: per-mille chance per tick that an unknown cultural item
	// appears somewhere.
	InventRate uint32
	// Substrates: languages spoken on the land before the family spreads.
	Substrates int32
	// SubstrateRate: per-mille chance that settling substrate land leaves
	// a substrate package (loans, a merger, word order).
	SubstrateRate uint32
	// CrossLang: per-mille strength of contact across a language boundary
	// for sound and grammatical changes. Words never cross by the wave:
	// they are borrowed.
	CrossLang uint32
	// Society turns on polities, conquest, disasters, migration, trade
	// routes and seafaring. PolityRate: per-mille chance per tick that a
	// large community founds a polity; ConquestRate: per-mille chance per
	// tick that a polity tries to expand; CollapseRate scales the chance
	// that it breaks up (1000 = default).
	Society                                bool
	PolityRate, ConquestRate, CollapseRate uint32
	// ScriptRate: per-mille chance per tick that a large polity with no
	// writing neighbour invents a script.
	ScriptRate uint32
	// Seafaring: every coastal community can cross the sea (scenarios).
	Seafaring bool
}

// DefaultConfig returns the standard configuration.
func DefaultConfig() Config {
	return Config{
		World:          world.DefaultConfig,
		SoundRate:      60,
		LexRate:        520,
		Candidates:     4,
		ClusterEvery:   4,
		SmallBias:      500,
		LargeBias:      500,
		HomelandRadius: 1,
		GramRate:       1000,
		AnalogyRate:    1000,
		OrderRate:      1000,
		SemRate:        30,
		LoanRate:       1000,
		InventRate:     40,
		Substrates:     4,
		SubstrateRate:  100,
		CrossLang:      500,
		Society:        true,
		PolityRate:     15,
		ConquestRate:   400,
		CollapseRate:   1000,
		ScriptRate:     10,
	}
}

// Language is a cluster of mutually intelligible varieties, tracked over time.
type Language struct {
	ID        int32
	Name      string
	Centre    int32 // hex
	Members   []int32
	Continuum int32
	Parent    int32 // language it split from, -1 for the first
	Born      int32
	Died      int32 // -1 while spoken
}

// Alive reports whether the language is still spoken.
func (l *Language) Alive() bool { return l.Died < 0 }

// LangFrame records language assignment per hex at one clustering tick.
type LangFrame struct {
	Tick int32
	Lang []int32 // per hex, -1 if unsettled
}

type liveInnov struct {
	in   *history.Innovation
	wave *pop.Wave
	age  int32
}

// Sim is one simulation run.
type Sim struct {
	Seed   uint64
	Cfg    Config
	Tick   int32
	World  *world.World
	Proto  *lex.Proto   // the first family's proto-language (Protos[0])
	Protos []*lex.Proto // one per language family
	Reg    *history.Registry
	// Scenario the run started from, nil for the default start.
	Scenario *scenario.Scenario
	Roots    []history.Root

	ByHex []*pop.Community // nil for unsettled hexes
	Langs []*Language      // all languages ever, by ID
	Hist  []LangFrame
	Tree  []history.TreeNode
	Log   []history.Event

	Interventions []Intervention

	live     []*liveInnov
	ap       *history.Applier
	vars     map[uint64]*lex.Variety
	cl       *pop.Clusterer
	present  []bool
	contacts [][]pop.Contact
	prestige []fx.Q16
	nextRoot lex.EtymID

	// Contact: substrate languages, their areas (per hex, -1 for none),
	// every hex's original place name, cultural items known so far.
	Subs     []*contact.Substrate
	Region   []int32
	Places   []contact.Place
	invented []bool
	subRoots map[[2]int32]lex.EtymID
	lang     []int32 // per hex, for the language barrier in spread

	// Society: polities ever founded (by ID), current trade routes, hubs,
	// route edges and the far ends of each hub's routes.
	Polities   []*society.Polity
	Routes     []society.Route
	hub        map[int32]bool
	routeEdge  map[[2]int32]bool
	routeLinks map[int32][]int32
	// conservative: capitals with a written standard, which take up
	// innovations more slowly.
	conservative []int32
	// FixedRoutes are trade routes set by events; they always run.
	FixedRoutes [][2]int32
	// normOf: per hex, the polity whose standard (and capital's speech) its
	// community follows (-1 none).
	normOf []int32
	// levels holds one levelling or learned-loan innovation per (polity,
	// kind, concept, root), reused by every community that takes it.
	levels map[[4]int32]*history.Innovation
}

// homeland picks the most fertile hex near the middle of the continent.
func (s *Sim) homeland() int32 {
	best, score := s.World.Land[0], int32(-1<<30)
	cx, cy := s.World.W/2, s.World.H/2
	for _, h := range s.World.Land {
		hx := &s.World.Hexes[h]
		if hx.Biome == world.Island || hx.Biome == world.Mountain {
			continue
		}
		dx, dy := hx.Col-cx, hx.Row-cy
		sc := hx.Capacity - (dx*dx+dy*dy)*20
		if sc > score {
			best, score = h, sc
		}
	}
	return best
}

func (s *Sim) settle(h, people int32, v *lex.Variety, from *pop.Community) {
	c := &pop.Community{Hex: h, Pop: people, Variety: v, Lang: -1, Founded: s.Tick, Polity: -1}
	if from != nil {
		c.Lang = from.Lang
		if p := s.polityOf(from); p != nil && s.World.Distance(p.Capital, h) <= maxReach {
			c.Polity, c.Joined, c.Foreign = p.ID, s.Tick, from.Foreign // colonists stay subjects
		}
		c.Diffusing = append([]pop.Diffusion(nil), from.Diffusing...)
		for _, l := range s.live {
			l.wave.Level[h] = l.wave.Level[from.Hex]
			l.wave.Applied[h] = l.wave.Applied[from.Hex]
		}
	}
	s.ByHex[h] = c
	s.present[h] = true
	s.takeName(c)
	if from != nil {
		s.substratePackage(c)
	}
}

// Communities returns settled communities in hex order.
func (s *Sim) Communities() []*pop.Community {
	var out []*pop.Community
	for _, c := range s.ByHex {
		if c != nil {
			out = append(out, c)
		}
	}
	return out
}

func (s *Sim) log(e history.Event) {
	e.Tick = s.Tick
	s.Log = append(s.Log, e)
}

func (s *Sim) intern(v *lex.Variety) *lex.Variety {
	if old, ok := s.vars[v.Key]; ok {
		return old
	}
	s.vars[v.Key] = v
	return v
}

func (s *Sim) refreshContacts() {
	s.contacts = pop.ContactWeights(s.World, s.present)
	s.adjustContacts()
	var maxPop int32
	for _, c := range s.ByHex {
		if c != nil && c.Pop > maxPop {
			maxPop = c.Pop
		}
	}
	for h, c := range s.ByHex {
		if c != nil {
			s.prestige[h] = fx.Q16(int64(pop.Prestige(c.Pop, maxPop)) * s.prestigeFactor(c) / 1000)
		} else {
			s.prestige[h] = 0
		}
	}
}

// Step runs one tick (one generation, ~25 years) in the fixed order.
func (s *Sim) Step() {
	s.Tick++
	s.applyInterventions() // 1
	s.grow()               // 2
	s.society()
	s.writing()
	s.refreshContacts() // 3
	s.propose()         // 4
	s.spread()          // 5, 6, 7 (apply includes repair)
	s.diffuse()
	s.pruneVarieties()
	if s.Tick%4 == 0 {
		s.sweepMemo()
	}
	if s.Tick%s.Cfg.ClusterEvery == 0 { // 10
		s.cluster()
	}
}

// Run steps n ticks.
func (s *Sim) Run(n int) {
	for i := 0; i < n; i++ {
		s.Step()
	}
}

// grow applies logistic population growth and expansion into empty land.
func (s *Sim) grow() {
	type colony struct {
		from *pop.Community
		to   int32
	}
	var colonies []colony
	claimed := map[int32]bool{}
	for _, c := range s.ByHex {
		if c == nil {
			continue
		}
		cap := s.World.Hexes[c.Hex].Capacity
		c.Pop += c.Pop * 500 / 1000 * (cap - c.Pop) / max(cap, 1)
		if c.Pop < 10 {
			c.Pop = 10
		}
		if c.Pop*10 < cap*4 {
			continue
		}
		best, bestCost := int32(-1), int32(1<<30)
		nbrs := s.World.Hexes[c.Hex].Nbr[:]
		if s.seafarer(c) {
			nbrs = append(slices.Clip(nbrs), society.SeaReach(s.World, c.Hex, 2)...)
		}
		for _, nb := range nbrs {
			if nb < 0 || s.ByHex[nb] != nil || claimed[nb] {
				continue
			}
			cost := s.World.Cost(c.Hex, nb)
			if cost < 0 {
				continue
			}
			cost -= s.World.Hexes[nb].Capacity / 20 // prefer fertile land
			if cost < bestCost {
				best, bestCost = nb, cost
			}
		}
		if best >= 0 {
			claimed[best] = true
			colonies = append(colonies, colony{c, best})
		}
	}
	for _, col := range colonies {
		people := col.from.Pop / 3
		col.from.Pop -= people
		s.settle(col.to, people, col.from.Variety, col.from)
		if !slices.Contains(s.World.Hexes[col.from.Hex].Nbr[:], col.to) {
			s.founderEffect(s.ByHex[col.to]) // a small group crossed the sea
		}
	}
}

// signedQ16 converts per-mille (may be negative) to signed Q16.
func signedQ16(pm int32) int32 { return pm * 65536 / 1000 }

func hashIH(innov uint32, hex int) uint32 {
	h := uint64(innov)*0x9E3779B97F4A7C15 ^ uint64(hex)*0xC2B2AE3D27D4EB4F
	h ^= h >> 31
	h *= 0xbf58476d1ce4e5b9
	h ^= h >> 29
	return uint32(h)
}

// spread advances every live innovation's wave and applies it where it
// crosses one half, in innovation order then hex order.
func (s *Sim) spread() {
	type cross struct {
		l   *liveInnov
		hex int32
	}
	var crossings []cross
	fade := signedQ16(-350)
	if len(s.lang) != len(s.ByHex) {
		s.lang = make([]int32, len(s.ByHex))
	}
	for h, c := range s.ByHex {
		s.lang[h] = -1
		if c != nil {
			s.lang[h] = c.Lang
		}
	}
	structural := fx.FromPermille(s.Cfg.CrossLang)
	for _, l := range s.live {
		barrier := structural
		switch l.in.Kind {
		case history.Lexical, history.Loan, history.Semantic:
			barrier = 0
		}
		spreading := l.age < l.in.Life
		id := l.in.ID
		rate := func(h int) int32 {
			if !spreading {
				return fade
			}
			f := int64(300 + hashIH(id, h)%1400) // per-mille local receptiveness
			if slices.Contains(s.conservative, int32(h)) {
				f /= 2 // the seat of a written standard changes slowly
			}
			if p := s.normOf[h]; p >= 0 && s.Polities[p].Capital != l.in.Origin {
				f = f * 20 / 100 // the norm resists innovations from elsewhere
			}
			return int32(int64(l.in.Rate) * f / 1000)
		}
		s.broadcast(l)
		for _, h := range l.wave.Step(s.present, s.contacts, s.prestige, s.lang, barrier, rate, spreading) {
			crossings = append(crossings, cross{l, h})
		}
		l.in.Curve = append(l.in.Curve, history.Adoption{Applied: uint16(l.wave.Count()), Reached: uint16(l.wave.Reach())})
		l.age++
	}
	for _, c := range crossings {
		s.adopt(s.ByHex[c.hex], c.l.in)
		if c.l.in.Kind == history.Sound && c.l.wave.Count() == 1 {
			s.log(history.Event{Kind: "innovation", Hex: c.hex, Innov: c.l.in.ID, Lang: s.ByHex[c.hex].Lang, Text: c.l.in.Label()})
		}
	}
	kept := s.live[:0]
	for _, l := range s.live {
		if l.age >= l.in.Life && (!l.wave.Alive() || l.age > l.in.Life+8) {
			continue // its memoised results go at the next sweepMemo
		}
		kept = append(kept, l)
	}
	s.live = kept
}

// adopt applies an innovation in one community. Diffusible sound changes
// start with one frequency class and creep on in diffuse().
func (s *Sim) adopt(c *pop.Community, in *history.Innovation) {
	e := lex.Entry{Kind: lex.EntryInnov, Innov: in.ID, Tick: s.Tick, Lo: 1, Hi: 5}
	switch in.Diffuse {
	case "freq-first":
		e.Hi = 1
		c.Diffusing = append(c.Diffusing, pop.Diffusion{Innov: in.ID, Next: 2, Step: 1})
	case "rare-first":
		e.Lo = 5
		c.Diffusing = append(c.Diffusing, pop.Diffusion{Innov: in.ID, Next: 4, Step: -1})
	}
	c.Variety = s.apply(c.Variety, e)
}

// apply adopts entry e in variety v. A word change that would change
// nothing here (the gap is filled, the word is already this one) is not
// recorded, so it costs nothing and leaves the variety shared.
func (s *Sim) apply(v *lex.Variety, e lex.Entry) *lex.Variety {
	if in := s.Reg.Get(e.Innov); in.Lex != nil {
		if _, _, ok := history.NewForm(v, in); !ok {
			return v
		}
		if l := v.Lex.At(in.Lex.Concept); l.Etym == in.Lex.Etym && l.Stratum == in.Lex.Stratum && in.Lex.From < 0 && in.Kind == history.Lexical {
			return v // the same word already
		}
	}
	return s.intern(s.ap.Apply(v, e))
}

// diffuse extends lexically diffusing changes by one frequency class per
// tick; each step may stall for good, leaving exceptions.
func (s *Sim) diffuse() {
	for _, c := range s.ByHex {
		if c == nil || len(c.Diffusing) == 0 {
			continue
		}
		r := rng.New(s.Seed, rng.Key("diffuse"), uint64(s.Tick), uint64(c.Hex))
		kept := c.Diffusing[:0]
		for _, d := range c.Diffusing {
			if r.Permille(150) {
				continue // stalled
			}
			e := lex.Entry{Kind: lex.EntryInnov, Innov: d.Innov, Tick: s.Tick, Lo: d.Next, Hi: d.Next}
			c.Variety = s.apply(c.Variety, e)
			next := int8(d.Next) + d.Step
			if next >= 1 && next <= 5 {
				d.Next = uint8(next)
				kept = append(kept, d)
			}
		}
		c.Diffusing = kept
	}
}

// Intel returns the mutual intelligibility of two varieties (cached).
func (s *Sim) Intel(a, b *lex.Variety) fx.Q16 { return s.cl.Intel(a, b) }

// Threshold is the intelligibility at which varieties count as one language.
func (s *Sim) Threshold() fx.Q16 { return s.cl.Thr }

// sweepMemo keeps memoised rule results only for innovations that are
// still spreading or diffusing somewhere.
func (s *Sim) sweepMemo() {
	keep := map[uint32]bool{}
	for _, l := range s.live {
		keep[l.in.ID] = true
	}
	for _, c := range s.ByHex {
		if c != nil {
			for _, d := range c.Diffusing {
				keep[d.Innov] = true
			}
		}
	}
	s.ap.Retain(keep)
}

// LiveCount returns how many innovations are still spreading.
func (s *Sim) LiveCount() int { return len(s.live) }

// Wave returns the live wave of an innovation, or nil.
func (s *Sim) Wave(id uint32) *pop.Wave {
	for _, l := range s.live {
		if l.in.ID == id {
			return l.wave
		}
	}
	return nil
}

// pruneVarieties keeps in the intern table only varieties some community
// speaks now (and the proto-languages): every adoption makes a new
// variety, and the intermediate ones would otherwise pile up.
func (s *Sim) pruneVarieties() {
	n := len(s.Protos)
	for _, c := range s.ByHex {
		if c != nil {
			n++
		}
	}
	if len(s.vars) <= 2*n {
		return
	}
	keep := make(map[uint64]*lex.Variety, n)
	for _, p := range s.Protos {
		keep[p.Variety.Key] = p.Variety
	}
	for _, c := range s.ByHex {
		if c != nil {
			keep[c.Variety.Key] = s.vars[c.Variety.Key]
			if keep[c.Variety.Key] == nil {
				keep[c.Variety.Key] = c.Variety
			}
		}
	}
	s.vars = keep
}
