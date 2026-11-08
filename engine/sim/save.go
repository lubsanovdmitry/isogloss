package sim

import (
	"errors"
	"slices"

	"isogloss/engine/fx"
	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/pop"
	"isogloss/engine/scenario"
	"isogloss/engine/tmpl"
)

// Intervention is a player action or a scripted scenario event, applied
// at the start of its tick. Kinds: rule (a forced sound change), collapse,
// plague, famine, migrate, conquer, script, standard, route, invent,
// seafaring.
type Intervention struct {
	Tick   int32
	Kind   string
	Hex    int32  // where (origin of a rule; -1 = every community / anywhere)
	From   int32  // migrate: origin; route: the other end (-1 none)
	Polity int32  // collapse, conquer, script, standard, seafaring: polity index
	Rule   string // rule source; script type; concept label (invent)
	Innov  uint32 // encourage, suppress: the innovation
	Paid   bool   // a campaign action: costs influence when it happens
}

// Save is everything needed to rebuild a run.
type Save struct {
	Version       int
	Seed          uint64
	Config        Config
	Tick          int32
	Scenario      string // scenario source, "" for none
	Interventions []Intervention
	// Annotations are storyteller outputs (names, chronicles, myths) keyed
	// by (seed, event, prompt version). They never affect the simulation;
	// they live in the save so a replay never asks the storyteller again.
	Annotations []Annotation
}

// Annotation is one cached storyteller output (opaque to the engine).
type Annotation struct {
	Key   string
	Value string
}

// Annotate stores (or replaces) an annotation.
func (s *Sim) Annotate(key, value string) {
	for i := range s.Annotations {
		if s.Annotations[i].Key == key {
			s.Annotations[i].Value = value
			return
		}
	}
	s.Annotations = append(s.Annotations, Annotation{key, value})
}

// SaveState captures the run for replay.
func (s *Sim) SaveState() Save {
	sv := Save{Version: 1, Seed: s.Seed, Config: s.Cfg, Tick: s.Tick, Interventions: slices.Clone(s.Interventions),
		Annotations: slices.Clone(s.Annotations)}
	if s.Scenario != nil {
		sv.Scenario = s.Scenario.Src
	}
	return sv
}

// Replay rebuilds a run from a save. The scenario's own events are
// rescheduled by NewScenario; the save's list already holds them, so it
// replaces them.
func Replay(sv Save) *Sim {
	s, err := ReplayErr(sv)
	if err != nil {
		panic(err)
	}
	return s
}

// ReplayErr is Replay for saves whose scenario may not parse.
func ReplayErr(sv Save) (*Sim, error) {
	var sc *scenario.Scenario
	if sv.Scenario != "" {
		var err error
		if sc, err = scenario.Read(sv.Scenario); err != nil {
			return nil, err
		}
	}
	s, err := NewScenario(sv.Seed, sv.Config, sc)
	if err != nil {
		return nil, err
	}
	if sc != nil {
		s.Cfg = sv.Config // the save's config already has the overrides
	}
	s.Interventions = slices.Clone(sv.Interventions)
	s.Annotations = slices.Clone(sv.Annotations)
	for s.Tick < sv.Tick {
		s.Step()
	}
	return s, nil
}

// Fork returns a save of this run truncated at tick t (interventions after
// t are dropped), to explore a what-if from that point.
func (s *Sim) Fork(t int32) Save {
	sv := s.SaveState()
	sv.Tick = min(t, s.Tick)
	var kept []Intervention
	for _, iv := range sv.Interventions {
		if iv.Tick <= sv.Tick {
			kept = append(kept, iv)
		}
	}
	sv.Interventions = kept
	return sv
}

// Intervene schedules an intervention for the next tick after validating it.
func (s *Sim) Intervene(iv Intervention) error {
	if s.Camp != nil {
		if _, ok := ActionCost[iv.Kind]; !ok {
			return errors.New("sim: " + iv.Kind + " is not a campaign action")
		}
		iv.Paid = true
		if iv.Polity < 0 {
			iv.Polity = s.Camp.Polity
		}
	}
	if err := s.validate(iv); err != nil {
		return err
	}
	if iv.Hex >= 0 && iv.Kind == "rule" && (int(iv.Hex) >= len(s.ByHex) || s.ByHex[iv.Hex] == nil) {
		return errors.New("sim: hex is not settled")
	}
	iv.Tick = s.Tick + 1
	s.Interventions = append(s.Interventions, iv)
	return nil
}

// validate checks what can be checked before the intervention's tick.
func (s *Sim) validate(iv Intervention) error {
	land := func(h int32) bool { return h >= 0 && int(h) < len(s.World.Hexes) && s.World.Hexes[h].Land() }
	switch iv.Kind {
	case "rule":
		if _, err := tmpl.Compile(iv.Rule); err != nil {
			return err
		}
	case "collapse", "standard", "seafaring", "script":
		if iv.Polity < 0 {
			return errors.New("sim: " + iv.Kind + " needs a polity")
		}
		if iv.Kind == "script" && iv.Rule != "" && iv.Rule != "alphabet" && iv.Rule != "abjad" && iv.Rule != "syllabary" {
			return errors.New("sim: unknown script " + iv.Rule)
		}
	case "plague", "famine":
		if iv.Hex >= 0 && !land(iv.Hex) {
			return errors.New("sim: " + iv.Kind + " needs a land hex")
		}
	case "migrate", "route":
		if !land(iv.Hex) || !land(iv.From) {
			return errors.New("sim: " + iv.Kind + " needs two land hexes")
		}
	case "conquer":
		if !land(iv.Hex) || iv.Polity < 0 {
			return errors.New("sim: conquer needs a polity and a land hex")
		}
	case "encourage", "suppress":
		if iv.Innov == 0 || int(iv.Innov) > len(s.Reg.All) {
			return errors.New("sim: " + iv.Kind + " needs an innovation")
		}
	case "expand":
		if !land(iv.Hex) {
			return errors.New("sim: expand needs a land hex")
		}
	case "invent":
		if _, ok := lex.ConceptByLabel(iv.Rule); !ok || !land(iv.Hex) {
			return errors.New("sim: invent needs a known concept and a land hex")
		}
	default:
		return errors.New("sim: unknown intervention " + iv.Kind)
	}
	return nil
}

func (s *Sim) applyInterventions() {
	for _, iv := range s.Interventions {
		if iv.Tick != s.Tick {
			continue
		}
		if iv.Kind != "rule" {
			s.applyEvent(iv)
			continue
		}
		rule, err := tmpl.Compile(iv.Rule)
		if err != nil {
			continue // validated when scheduled
		}
		in := &history.Innovation{
			Kind: history.Sound, Template: "user:" + rule.Name, Family: "user",
			Src: iv.Rule, Rule: rule, Diffuse: rule.Diffuse, Stress: -1, Forced: true,
			Rate: signedQ16(600), Life: 12,
		}
		origin := iv.Hex
		if origin < 0 {
			origin = s.homeland()
		}
		in.Origin, in.Tick = origin, s.Tick
		s.Reg.Add(in)
		s.ap.Track(in.ID)
		l := &liveInnov{in: in, wave: pop.NewWave(len(s.World.Hexes))}
		s.live = append(s.live, l)
		targets := []int32{iv.Hex}
		if iv.Hex < 0 {
			targets = targets[:0]
			for _, c := range s.Communities() {
				targets = append(targets, c.Hex)
			}
		}
		for _, h := range targets {
			if s.ByHex[h] == nil {
				continue
			}
			l.wave.Level[h], l.wave.Applied[h] = fx.One, true
			s.adopt(s.ByHex[h], in)
		}
		s.log(history.Event{Kind: "intervention", Hex: iv.Hex, Innov: in.ID, Text: "forced change " + rule.Name})
	}
}

// Digest hashes the observable state of the run: every community's variety
// (history and forms), population, language, plus the language list. Golden
// tests compare it across platforms.
func (s *Sim) Digest() uint64 {
	h := uint64(0xcbf29ce484222325)
	mix := func(x uint64) {
		h ^= x
		h *= 0x100000001b3
		h ^= h >> 29
	}
	mix(uint64(s.Tick))
	for _, c := range s.Communities() {
		mix(uint64(c.Hex))
		mix(uint64(c.Pop))
		mix(uint64(uint32(c.Lang)) | uint64(uint32(c.Polity))<<32)
		mix(c.Variety.Key)
		for i := 0; i < c.Variety.Lex.Len(); i++ {
			l := c.Variety.Lex.At(lex.ConceptID(i))
			mix(l.Hash)
			mix(uint64(l.Etym) | uint64(l.Also)<<32 | uint64(l.Stratum.Kind)<<48)
			mix(uint64(uint32(l.Stratum.Donor)) | uint64(uint32(l.Stratum.Tick))<<32)
		}
		m := &c.Variety.M
		for _, e := range m.Exp {
			mix(uint64(e.Stage) | uint64(uint32(e.Source))<<8)
		}
		for _, o := range m.Order {
			mix(uint64(o))
		}
		for _, p := range m.Para {
			if p != nil {
				for _, x := range p.Hash {
					mix(x)
				}
			}
		}
	}
	for _, l := range s.Langs {
		mix(uint64(l.Centre))
		mix(uint64(uint32(l.Died)))
		for _, b := range []byte(l.Name) {
			mix(uint64(b))
		}
	}
	for _, p := range s.Polities {
		mix(uint64(p.Capital) | uint64(uint32(p.Fell))<<32)
	}
	mix(uint64(len(s.Reg.All)))
	return h
}
