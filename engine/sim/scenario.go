package sim

import (
	"errors"
	"strconv"
	"strings"

	"isogloss/engine/fx"
	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/pop"
	"isogloss/engine/rng"
	"isogloss/engine/scenario"
	"isogloss/engine/society"
	"isogloss/engine/world"
)

// New creates a run at tick 0: map, proto-language and homeland.
func New(seed uint64, cfg Config) *Sim {
	s, err := NewScenario(seed, cfg, nil)
	if err != nil {
		panic(err) // without a scenario nothing can fail
	}
	return s
}

// NewScenario creates a run from a scenario (nil for the default start):
// the scenario's world and config overrides apply first, then its
// families settle, its polities are founded, and its events are scheduled
// as interventions, so a scenario run replays like any other.
func NewScenario(seed uint64, cfg Config, sc *scenario.Scenario) (*Sim, error) {
	if sc != nil {
		if sc.Seed != 0 {
			seed = sc.Seed
		}
		applyWorld(&cfg.World, sc.World)
		for _, st := range sc.Config {
			if err := applySetting(&cfg, st.Key, st.Value); err != nil {
				return nil, errors.New("scenario: line " + strconv.Itoa(st.Line) + ": " + err.Error())
			}
		}
	}
	s := &Sim{
		Seed: seed, Cfg: cfg, Scenario: sc,
		World:      world.Generate(seed, cfg.World),
		Reg:        &history.Registry{},
		vars:       map[uint64]*lex.Variety{},
		cl:         pop.NewClusterer(),
		nextRoot:   lex.FirstCoinedRoot,
		subRoots:   map[[2]int32]lex.EtymID{},
		hub:        map[int32]bool{},
		routeEdge:  map[[2]int32]bool{},
		routeLinks: map[int32][]int32{},
		levels:     map[[4]int32]*history.Innovation{},
	}
	if cfg.Threshold > 0 {
		s.cl.Thr = fx.FromPermille(cfg.Threshold)
	}
	s.ap = history.NewApplier(s.Reg)
	n := len(s.World.Hexes)
	s.ByHex = make([]*pop.Community, n)
	s.present = make([]bool, n)
	s.prestige = make([]fx.Q16, n)

	fams := []scenario.Family{{Place: "homeland", Radius: cfg.HomelandRadius}}
	var opt lex.ProtoOptions
	if sc != nil {
		if len(sc.Families) > 0 {
			fams = sc.Families
		}
		opt = lex.ProtoOptions{Template: sc.Proto.Template, MaxSyllables: int(sc.Proto.Syllables), FinalGlottal: uint32(max(sc.Proto.FinalGlottal, 0))}
	}
	if len(fams) > 8 {
		return nil, errors.New("scenario: at most 8 families")
	}
	homes := make([]int32, len(fams))
	for f, fam := range fams {
		h, err := s.Place(fam.Place)
		if err != nil {
			return nil, err
		}
		homes[f] = h
		o := opt
		o.Family = uint8(f)
		r := rng.New(seed, rng.Key("proto"))
		if f > 0 {
			r = rng.New(seed, rng.Key("proto"), uint64(f))
		}
		p := lex.GenerateProtoWith(r, o)
		s.Protos = append(s.Protos, p)
		s.vars[p.Variety.Key] = p.Variety
	}
	s.Proto = s.Protos[0]

	// Homelands: hexes within each family's radius ("all": every hex
	// closer to that family's centre than to any other's).
	homeland := make([]bool, n)
	owner := make([]int32, n)
	for i := range owner {
		owner[i] = -1
	}
	for _, h := range s.World.Land {
		f := s.nearestFamily(homes, h)
		rad := fams[f].Radius
		if rad < 0 || s.World.Distance(homes[f], h) <= rad {
			homeland[h], owner[h] = true, int32(f)
		}
	}
	s.setupContact(homes, homeland)
	for _, h := range s.World.Land {
		if f := owner[h]; f >= 0 {
			s.settle(h, 400, s.Protos[f].Variety, nil)
		}
	}
	for f := range fams {
		s.log(history.Event{Kind: "settle", Hex: homes[f], Lang: -1, Text: s.FamilyName(uint8(f)) + " is spoken around its homeland"})
	}
	s.refreshContacts()
	s.cluster()
	if sc != nil {
		if err := s.setupPolities(sc, homes); err != nil {
			return nil, err
		}
		if err := s.scheduleEvents(sc); err != nil {
			return nil, err
		}
		if sc.Campaign != nil {
			if err := s.setupCampaign(sc.Campaign); err != nil {
				return nil, err
			}
		}
	}
	return s, nil
}

func applyWorld(w *world.Config, sw scenario.World) {
	if sw.Width > 0 {
		w.W = sw.Width
	}
	if sw.Height > 0 {
		w.H = sw.Height
	}
	if sw.Land > 0 {
		w.Land = int(sw.Land)
	}
	if sw.Rivers > 0 {
		w.Rivers = int(sw.Rivers)
	}
	switch sw.Shape {
	case "archipelago":
		w.Shape = world.Archipelago
	case "corridor":
		w.Shape = world.Corridor
	}
}

// applySetting overrides one config value by its scenario name.
func applySetting(c *Config, key string, v int64) error {
	u := uint32(max(v, 0))
	switch key {
	case "sound-rate":
		c.SoundRate = u
	case "lex-rate":
		c.LexRate = u
	case "homeland-radius":
		c.HomelandRadius = int32(v)
	case "threshold":
		c.Threshold = u
	case "gram-rate":
		c.GramRate = u
	case "analogy-rate":
		c.AnalogyRate = u
	case "order-rate":
		c.OrderRate = u
	case "sem-rate":
		c.SemRate = u
	case "loan-rate":
		c.LoanRate = u
	case "invent-rate":
		c.InventRate = u
	case "substrates":
		c.Substrates = int32(v)
	case "substrate-rate":
		c.SubstrateRate = u
	case "cross-lang":
		c.CrossLang = u
	case "society":
		c.Society = v != 0
	case "polity-rate":
		c.PolityRate = u
	case "conquest-rate":
		c.ConquestRate = u
	case "collapse-rate":
		c.CollapseRate = u
	case "script-rate":
		c.ScriptRate = u
	case "seafaring":
		c.Seafaring = v != 0
	case "small-bias":
		c.SmallBias = u
	case "large-bias":
		c.LargeBias = u
	case "candidates":
		c.Candidates = int(max(v, 1))
	case "cluster-every":
		c.ClusterEvery = int32(max(v, 1))
	case "width":
		c.World.W = int32(min(max(v, 12), 48))
	case "height":
		c.World.H = int32(min(max(v, 10), 40))
	case "land":
		c.World.Land = int(min(max(v, 40), 1400))
	case "rivers":
		c.World.Rivers = int(min(max(v, 0), 40))
	case "shape":
		c.World.Shape = world.Shape(min(max(v, 0), 2))
	default:
		return errors.New("unknown config key '" + key + "'")
	}
	return nil
}

// ApplySetting sets one config value by name, as a scenario's config
// section does (the world's size, land, rivers and shape too).
func ApplySetting(c *Config, key string, v int64) error { return applySetting(c, key, v) }

// Setting is one named config value.
type Setting struct {
	Key   string
	Value int64
}

// Settings lists every config value by name; applying them to
// DefaultConfig rebuilds c (TestSettingsRoundTrip), which is how saves
// carry a run's config.
func (c Config) Settings() []Setting {
	b := func(x bool) int64 {
		if x {
			return 1
		}
		return 0
	}
	return []Setting{
		{"sound-rate", int64(c.SoundRate)}, {"lex-rate", int64(c.LexRate)}, {"homeland-radius", int64(c.HomelandRadius)},
		{"threshold", int64(c.Threshold)}, {"gram-rate", int64(c.GramRate)}, {"analogy-rate", int64(c.AnalogyRate)},
		{"order-rate", int64(c.OrderRate)}, {"sem-rate", int64(c.SemRate)}, {"loan-rate", int64(c.LoanRate)},
		{"invent-rate", int64(c.InventRate)}, {"substrates", int64(c.Substrates)}, {"substrate-rate", int64(c.SubstrateRate)},
		{"cross-lang", int64(c.CrossLang)}, {"society", b(c.Society)}, {"polity-rate", int64(c.PolityRate)},
		{"conquest-rate", int64(c.ConquestRate)}, {"collapse-rate", int64(c.CollapseRate)}, {"script-rate", int64(c.ScriptRate)},
		{"seafaring", b(c.Seafaring)}, {"small-bias", int64(c.SmallBias)}, {"large-bias", int64(c.LargeBias)},
		{"candidates", int64(c.Candidates)}, {"cluster-every", int64(c.ClusterEvery)},
		{"width", int64(c.World.W)}, {"height", int64(c.World.H)}, {"land", int64(c.World.Land)},
		{"rivers", int64(c.World.Rivers)}, {"shape", int64(c.World.Shape)},
	}
}

func (s *Sim) nearestFamily(homes []int32, h int32) uint8 {
	best, bestD := 0, int32(1<<30)
	for f, x := range homes {
		if d := s.World.Distance(x, h); d < bestD {
			best, bestD = f, d
		}
	}
	return uint8(best)
}

// Place resolves a place name in a scenario to a land hex: homeland,
// centre, west, east, north, south, a biome (steppe, coast, forest, desert,
// mountain, plains, island), fertile, or a hex number.
func (s *Sim) Place(name string) (int32, error) {
	if alts := strings.Split(name, "|"); len(alts) > 1 {
		var err error
		for _, a := range alts {
			var h int32
			if h, err = s.Place(strings.TrimSpace(a)); err == nil {
				return h, nil
			}
		}
		return 0, err
	}
	w := s.World
	if name == "" || name == "homeland" {
		return s.homeland(), nil
	}
	if n, err := strconv.Atoi(name); err == nil {
		if n < 0 || n >= len(w.Hexes) || !w.Hexes[n].Land() {
			return 0, errors.New("scenario: hex " + name + " is not land")
		}
		return int32(n), nil
	}
	cx, cy := w.W/2, w.H/2
	dist2 := func(h int32) int32 {
		dx, dy := w.Hexes[h].Col-cx, w.Hexes[h].Row-cy
		return dx*dx + dy*dy
	}
	// best picks the land hex maximising score (ties: lower ID).
	best := func(score func(h *world.Hex) (int32, bool)) (int32, error) {
		b, bs := int32(-1), int32(-1<<30)
		for _, id := range w.Land {
			if sc, ok := score(&w.Hexes[id]); ok && sc > bs {
				b, bs = id, sc
			}
		}
		if b < 0 {
			return 0, errors.New("scenario: no land matches place '" + name + "'")
		}
		return b, nil
	}
	third := func(h *world.Hex) bool {
		switch name {
		case "west":
			return h.Col < w.W/3
		case "east":
			return h.Col >= w.W*2/3
		case "north":
			return h.Row < w.H/3
		case "south":
			return h.Row >= w.H*2/3
		}
		return false
	}
	switch name {
	case "centre", "center":
		return best(func(h *world.Hex) (int32, bool) { return -dist2(h.ID), true })
	case "west", "east", "north", "south":
		return best(func(h *world.Hex) (int32, bool) { return h.Capacity*4 - dist2(h.ID), third(h) })
	case "fertile":
		return best(func(h *world.Hex) (int32, bool) { return h.Capacity*10 - dist2(h.ID), true })
	}
	for b := world.Biome(1); b < world.NumBiomes; b++ {
		if b.String() == name {
			return best(func(h *world.Hex) (int32, bool) { return h.Capacity - dist2(h.ID)*5, h.Biome == b })
		}
	}
	return 0, errors.New("scenario: unknown place '" + name + "'")
}

// setupPolities founds the scenario's starting polities. A capital may be
// a place or "family N" (that family's homeland); covers is a radius, or
// all for every community of the capital's language.
func (s *Sim) setupPolities(sc *scenario.Scenario, homes []int32) error {
	for _, sp := range sc.Polities {
		var cap int32
		if f, ok := strings.CutPrefix(sp.Capital, "family "); ok {
			k, err := strconv.Atoi(f)
			if err != nil || k < 1 || k > len(homes) {
				return errors.New("scenario: no " + sp.Capital)
			}
			cap = homes[k-1]
		} else {
			h, err := s.Place(sp.Capital)
			if err != nil {
				return err
			}
			cap = h
		}
		c := s.ByHex[cap]
		if c == nil {
			return errors.New("scenario: the capital " + sp.Capital + " is not settled at the start")
		}
		p := s.foundPolity(c)
		for _, o := range s.ByHex {
			if o == nil || o.Polity >= 0 || o.Lang != c.Lang {
				continue
			}
			if sp.Covers < 0 || s.World.Distance(cap, o.Hex) <= sp.Covers {
				s.join(o, p)
			}
		}
		if sp.Script != "" {
			p.Script = society.NewOrthography(scriptType(sp.Script), c.Variety, 0)
		}
		if sp.Standard {
			if p.Script == nil {
				p.Script = society.NewOrthography(society.Alphabet, c.Variety, 0)
			}
			p.Standard = c.Variety
		}
		p.Seafaring = sp.Seafaring
	}
	return nil
}

func scriptType(name string) society.ScriptType {
	switch name {
	case "abjad":
		return society.Abjad
	case "syllabary":
		return society.Syllabary
	}
	return society.Alphabet
}

// scheduleEvents turns scenario events into interventions.
func (s *Sim) scheduleEvents(sc *scenario.Scenario) error {
	for _, e := range sc.Events {
		iv := Intervention{Tick: e.Tick, Kind: e.Do, Hex: -1, From: -1, Polity: e.Polity, Rule: e.What, Scripted: true}
		if e.Place != "" {
			h, err := s.Place(e.Place)
			if err != nil {
				return errors.New("scenario: line " + strconv.Itoa(e.Line) + ": " + err.Error())
			}
			iv.Hex = h
		}
		if e.From != "" {
			h, err := s.Place(e.From)
			if err != nil {
				return errors.New("scenario: line " + strconv.Itoa(e.Line) + ": " + err.Error())
			}
			iv.From = h
		}
		if err := s.validate(iv); err != nil {
			return errors.New("scenario: line " + strconv.Itoa(e.Line) + ": " + err.Error())
		}
		s.Interventions = append(s.Interventions, iv)
	}
	return nil
}
