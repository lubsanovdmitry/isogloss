package sim

import (
	"errors"
	"slices"

	"isogloss/engine/fx"
	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/pop"
	"isogloss/engine/tmpl"
)

// Intervention is a player action, applied at the start of its tick.
type Intervention struct {
	Tick int32
	Kind string // "rule"
	Hex  int32  // origin hex; -1 = every community at once
	Rule string // rule source (Prelude classes available)
}

// Save is everything needed to rebuild a run.
type Save struct {
	Version       int
	Seed          uint64
	Config        Config
	Tick          int32
	Interventions []Intervention
}

// SaveState captures the run for replay.
func (s *Sim) SaveState() Save {
	return Save{Version: 1, Seed: s.Seed, Config: s.Cfg, Tick: s.Tick, Interventions: slices.Clone(s.Interventions)}
}

// Replay rebuilds a run from a save.
func Replay(sv Save) *Sim {
	s := New(sv.Seed, sv.Config)
	s.Interventions = slices.Clone(sv.Interventions)
	for s.Tick < sv.Tick {
		s.Step()
	}
	return s
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
	switch iv.Kind {
	case "rule":
		if _, err := tmpl.Compile(iv.Rule); err != nil {
			return err
		}
		if iv.Hex >= 0 && (int(iv.Hex) >= len(s.ByHex) || s.ByHex[iv.Hex] == nil) {
			return errors.New("sim: hex is not settled")
		}
	default:
		return errors.New("sim: unknown intervention " + iv.Kind)
	}
	iv.Tick = s.Tick + 1
	s.Interventions = append(s.Interventions, iv)
	return nil
}

func (s *Sim) applyInterventions() {
	for _, iv := range s.Interventions {
		if iv.Tick != s.Tick || iv.Kind != "rule" {
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
		mix(uint64(uint32(c.Lang)))
		mix(c.Variety.Key)
		for i := 0; i < c.Variety.Lex.Len(); i++ {
			l := c.Variety.Lex.At(lex.ConceptID(i))
			mix(l.Hash)
			mix(uint64(l.Etym))
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
	mix(uint64(len(s.Reg.All)))
	return h
}
