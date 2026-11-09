package scenario

import (
	"slices"
	"strings"
)

// Scenario is a preset: map constraints, starting languages and polities,
// config overrides and scripted events.
type Scenario struct {
	Name, Summary, Watch string
	Seed                 uint64 // 0: the player's seed
	Ticks                int32  // suggested length (0: 40)
	World                World
	Proto                Proto
	Families             []Family // starting languages; empty: one family in the homeland
	Config               []Setting
	Polities             []Polity
	Events               []Event
	Campaign             *Campaign // nil: sandbox
	Src                  string    // the source text, kept for saves
}

// Campaign makes the run a steering game: the player guides the people of
// one polity (founded at People), spending influence on actions, toward
// goals.
type Campaign struct {
	People string // capital place of the player's polity
	Income int32  // income scale, per mille (default 1000)
	Start  int32  // influence at the start
	Goals  []Goal
}

// Goal is one campaign goal: Kind is spoken (≥ N hexes speak the people's
// language), unity (the farthest hex of the polity still understands the
// capital), tonal (the capital's speech has tone), daughters (≥ N living
// languages of the people's family) or survive (the polity stands).
type Goal struct {
	Kind string
	N    int32
}

// World constrains map generation.
type World struct {
	Width, Height, Land int32  // 0: defaults
	Shape               string // continent, archipelago, corridor
	Rivers              int32
}

// Proto constrains the proto-language(s).
type Proto struct {
	Template     string
	Syllables    int32
	FinalGlottal int32  // per mille
	Language     string // a real proto-language ("latin"); the others are then ignored
}

// Family is one language family's starting area.
type Family struct {
	Name     string // optional
	Language string // a real proto-language for this family (overrides proto's)
	Place  string // where its homeland is: homeland, centre, west, east, north, south, steppe, coast, fertile, or a hex number
	Radius int32  // hexes settled around it at the start (default 1); "all" is -1
}

// Setting overrides one config value by name ("collapse-rate: 0").
type Setting struct {
	Key   string
	Value int64
	Line  int
}

// Polity exists from the start.
type Polity struct {
	Capital   string // a place (as for families) or a family's homeland ("family 1")
	Covers    int32  // radius of hexes it rules; -1 for every hex of its language
	Script    string // alphabet, abjad, syllabary or ""
	Standard  bool
	Seafaring bool
}

// Event is a scripted intervention.
type Event struct {
	Tick   int32
	Do     string // collapse, plague, famine, migrate, conquer, script, standard, route, invent, seafaring, rule
	Polity int32  // polity index (by order of founding) for collapse, conquer, script, standard, seafaring
	Place  string // where: for plague, famine, migrate (destination), conquer (target), route (one end), invent
	From   string // migrate: origin; route: the other end
	What   string // script type, concept label (invent), rule source (rule)
	Line   int
}

var (
	topKeys    = []string{"name", "summary", "watch", "seed", "ticks", "world", "proto", "families", "config", "polities", "events", "campaign"}
	campKeys   = []string{"people", "income", "start", "goals"}
	GoalKinds  = []string{"spoken", "unity", "tonal", "daughters", "survive"}
	worldKeys  = []string{"width", "height", "land", "shape", "rivers"}
	protoKeys  = []string{"template", "syllables", "final-glottal", "language"}
	familyKeys = []string{"name", "place", "radius", "language"}
	polityKeys = []string{"capital", "covers", "script", "standard", "seafaring"}
	eventKeys  = []string{"tick", "year", "do", "polity", "place", "from", "what", "rule"}
	Actions    = []string{"collapse", "plague", "famine", "migrate", "conquer", "script", "standard", "route", "invent", "seafaring", "rule"}
)

func checkKeys(n *Node, allowed []string, where string) error {
	if n == nil {
		return nil
	}
	if n.Kind != Map {
		return &Error{n.Line, where + " must be a mapping"}
	}
	for i, k := range n.Keys {
		if !slices.Contains(allowed, k) {
			return &Error{n.Vals[i].Line, "unknown key '" + k + "' in " + where + " (allowed: " + strings.Join(allowed, ", ") + ")"}
		}
	}
	return nil
}

func str(n *Node, key string) string {
	if v := n.Get(key); v != nil && v.Kind == Scalar {
		return v.Str
	}
	return ""
}

func num(n *Node, key string, def int64) (int64, error) {
	v := n.Get(key)
	if v == nil {
		return def, nil
	}
	if v.Str == "all" {
		return -1, nil
	}
	return v.Int()
}

func flag(n *Node, key string) (bool, error) {
	v := n.Get(key)
	if v == nil {
		return false, nil
	}
	return v.Bool()
}

func list(n *Node, key string) ([]*Node, error) {
	v := n.Get(key)
	if v == nil {
		return nil, nil
	}
	if v.Kind != Seq {
		return nil, &Error{v.Line, key + " must be a list"}
	}
	return v.Items, nil
}

// Read parses and validates a scenario file.
func Read(src string) (*Scenario, error) {
	root, err := Parse(src)
	if err != nil {
		return nil, err
	}
	if err := checkKeys(root, topKeys, "the scenario"); err != nil {
		return nil, err
	}
	sc := &Scenario{Name: str(root, "name"), Summary: str(root, "summary"), Watch: str(root, "watch"), Src: src}
	if sc.Name == "" {
		return nil, &Error{1, "a scenario needs a name"}
	}
	seed, err := num(root, "seed", 0)
	if err != nil {
		return nil, err
	}
	ticks, err := num(root, "ticks", 40)
	if err != nil {
		return nil, err
	}
	sc.Seed, sc.Ticks = uint64(seed), int32(ticks)

	if w := root.Get("world"); w != nil {
		if err := checkKeys(w, worldKeys, "world"); err != nil {
			return nil, err
		}
		vals := make([]int64, 4)
		for i, k := range []string{"width", "height", "land", "rivers"} {
			if vals[i], err = num(w, k, 0); err != nil {
				return nil, err
			}
		}
		sc.World = World{Width: int32(vals[0]), Height: int32(vals[1]), Land: int32(vals[2]), Rivers: int32(vals[3]), Shape: str(w, "shape")}
		if s := sc.World.Shape; s != "" && s != "continent" && s != "archipelago" && s != "corridor" {
			return nil, &Error{w.Get("shape").Line, "shape must be continent, archipelago or corridor"}
		}
	}
	if p := root.Get("proto"); p != nil {
		if err := checkKeys(p, protoKeys, "proto"); err != nil {
			return nil, err
		}
		syl, err := num(p, "syllables", 0)
		if err != nil {
			return nil, err
		}
		fg, err := num(p, "final-glottal", 0)
		if err != nil {
			return nil, err
		}
		sc.Proto = Proto{Template: str(p, "template"), Syllables: int32(syl), FinalGlottal: int32(fg), Language: str(p, "language")}
	}
	fams, err := list(root, "families")
	if err != nil {
		return nil, err
	}
	for _, f := range fams {
		if err := checkKeys(f, familyKeys, "a family"); err != nil {
			return nil, err
		}
		rad, err := num(f, "radius", 1)
		if err != nil {
			return nil, err
		}
		sc.Families = append(sc.Families, Family{Name: str(f, "name"), Place: str(f, "place"), Radius: int32(rad), Language: str(f, "language")})
	}
	if c := root.Get("config"); c != nil {
		if c.Kind != Map {
			return nil, &Error{c.Line, "config must be a mapping"}
		}
		for i, k := range c.Keys {
			v, err := c.Vals[i].Int()
			if err != nil {
				b, berr := c.Vals[i].Bool()
				if berr != nil {
					return nil, err
				}
				v = 0
				if b {
					v = 1
				}
			}
			sc.Config = append(sc.Config, Setting{k, v, c.Vals[i].Line})
		}
	}
	pols, err := list(root, "polities")
	if err != nil {
		return nil, err
	}
	for _, p := range pols {
		if err := checkKeys(p, polityKeys, "a polity"); err != nil {
			return nil, err
		}
		cov, err := num(p, "covers", 1)
		if err != nil {
			return nil, err
		}
		std, err := flag(p, "standard")
		if err != nil {
			return nil, err
		}
		sea, err := flag(p, "seafaring")
		if err != nil {
			return nil, err
		}
		pol := Polity{Capital: str(p, "capital"), Covers: int32(cov), Script: str(p, "script"), Standard: std, Seafaring: sea}
		if pol.Script != "" && pol.Script != "alphabet" && pol.Script != "abjad" && pol.Script != "syllabary" {
			return nil, &Error{p.Get("script").Line, "script must be alphabet, abjad or syllabary"}
		}
		sc.Polities = append(sc.Polities, pol)
	}
	evs, err := list(root, "events")
	if err != nil {
		return nil, err
	}
	for _, e := range evs {
		if err := checkKeys(e, eventKeys, "an event"); err != nil {
			return nil, err
		}
		ev := Event{Do: str(e, "do"), Place: str(e, "place"), From: str(e, "from"), What: str(e, "what"), Line: e.Line}
		if r := str(e, "rule"); r != "" {
			ev.What = r
		}
		t, err := num(e, "tick", -1)
		if err != nil {
			return nil, err
		}
		if y, err := num(e, "year", -1); err != nil {
			return nil, err
		} else if y >= 0 {
			t = y / 25
		}
		if t < 1 {
			return nil, &Error{e.Line, "an event needs a tick or year after the start"}
		}
		pol, err := num(e, "polity", 0)
		if err != nil {
			return nil, err
		}
		ev.Tick, ev.Polity = int32(t), int32(pol)
		if !slices.Contains(Actions, ev.Do) {
			return nil, &Error{e.Line, "unknown action '" + ev.Do + "' (one of: " + strings.Join(Actions, ", ") + ")"}
		}
		sc.Events = append(sc.Events, ev)
	}
	slices.SortStableFunc(sc.Events, func(a, b Event) int { return int(a.Tick - b.Tick) })
	if c := root.Get("campaign"); c != nil {
		if err := checkKeys(c, campKeys, "campaign"); err != nil {
			return nil, err
		}
		inc, err := num(c, "income", 1000)
		if err != nil {
			return nil, err
		}
		start, err := num(c, "start", 100)
		if err != nil {
			return nil, err
		}
		camp := &Campaign{People: str(c, "people"), Income: int32(inc), Start: int32(start)}
		goals, err := list(c, "goals")
		if err != nil {
			return nil, err
		}
		for _, g := range goals {
			if g.Kind != Map || len(g.Keys) != 1 || !slices.Contains(GoalKinds, g.Keys[0]) {
				return nil, &Error{g.Line, "a goal is one of: " + strings.Join(GoalKinds, ", ") + " (e.g. 'spoken: 40')"}
			}
			v := g.Vals[0]
			n := int64(1)
			if v.Str != "true" && v.Str != "yes" {
				if n, err = v.Int(); err != nil {
					return nil, err
				}
			}
			camp.Goals = append(camp.Goals, Goal{g.Keys[0], int32(n)})
		}
		sc.Campaign = camp
	}
	return sc, nil
}
