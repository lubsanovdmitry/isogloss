package sim

import (
	"errors"
	"strings"

	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/morph"
	"isogloss/engine/rng"
)

// Structural interventions: the player starts a grammatical construction,
// pushes one a stage further, sets a word-order parameter or changes the
// syllable template. Each becomes an ordinary innovation that starts in
// one hex (or everywhere) and spreads like any other, so it is saved with
// the run and replays exactly. Rule holds the request:
//
//	gram       "FUT go"       recruit 'go' as a future construction
//	gram       "FUT +"        push the future one stage further
//	order      "clause order=SVO"
//	syllables  "onset+1" | "onset-1" | "coda+1" | "coda-1"

// slotByGloss finds a slot by its gloss (FUT, 1SG) or name (future).
func slotByGloss(g string) (morph.Slot, bool) {
	for s := morph.Slot(0); s < morph.NumSlots; s++ {
		if strings.EqualFold(s.String(), g) || strings.EqualFold(s.Info().Name, g) {
			return s, true
		}
	}
	return 0, false
}

func parseGram(req string) (slot morph.Slot, src lex.ConceptID, push bool, err error) {
	f := strings.Fields(req)
	if len(f) != 2 {
		return 0, 0, false, errors.New("sim: gram wants \"SLOT word\" or \"SLOT +\"")
	}
	slot, ok := slotByGloss(f[0])
	if !ok {
		return 0, 0, false, errors.New("sim: unknown grammatical slot " + f[0])
	}
	if f[1] == "+" {
		return slot, 0, true, nil
	}
	src, ok = lex.ConceptByLabel(f[1])
	if !ok {
		return 0, 0, false, errors.New("sim: unknown word " + f[1])
	}
	return slot, src, false, nil
}

func parseOrder(req string) (*history.OrderChange, error) {
	name, val, ok := strings.Cut(req, "=")
	if !ok {
		return nil, errors.New("sim: order wants \"parameter=value\"")
	}
	for p := morph.Param(0); p < morph.NumParams; p++ {
		if !strings.EqualFold(p.Name(), strings.TrimSpace(name)) {
			continue
		}
		for i, v := range p.Values() {
			if strings.EqualFold(v, strings.TrimSpace(val)) {
				return &history.OrderChange{Param: p, Value: uint8(i)}, nil
			}
		}
		return nil, errors.New("sim: " + p.Name() + " has no value " + val)
	}
	return nil, errors.New("sim: unknown word-order parameter " + name)
}

func parseSyll(req string) (*history.SyllChange, error) {
	switch req {
	case "onset+1":
		return &history.SyllChange{Onset: 1}, nil
	case "onset-1":
		return &history.SyllChange{Onset: -1}, nil
	case "coda+1":
		return &history.SyllChange{Coda: 1}, nil
	case "coda-1":
		return &history.SyllChange{Coda: -1}, nil
	}
	return nil, errors.New("sim: syllables wants onset+1, onset-1, coda+1 or coda-1")
}

// validateStructural checks a structural request when it is scheduled.
func validateStructural(iv Intervention) error {
	var err error
	switch iv.Kind {
	case "gram":
		_, _, _, err = parseGram(iv.Rule)
	case "order":
		_, err = parseOrder(iv.Rule)
	case "syllables":
		_, err = parseSyll(iv.Rule)
	}
	return err
}

// structuralInnov builds the innovation for a structural request, judged
// against the variety where it starts (nil when there is nothing to do:
// a slot that is already bound cannot take a new construction, one with
// no construction cannot be pushed).
func (s *Sim) structuralInnov(iv Intervention, origin int32) (*history.Innovation, string) {
	c := s.ByHex[origin]
	if c == nil {
		return nil, "nobody lives there"
	}
	v := c.Variety
	in := &history.Innovation{Stress: -1}
	switch iv.Kind {
	case "gram":
		slot, src, push, _ := parseGram(iv.Rule)
		e := v.M.Exp[slot]
		g := &history.GramChange{Slot: slot, Source: e.Source, Prefix: e.Prefix}
		if push {
			switch e.Stage {
			case morph.Periphrasis:
				g.To = morph.Fixed
			case morph.Fixed:
				g.To = morph.Clitic
			case morph.Clitic:
				g.To = morph.Affix
			default:
				return nil, "the " + slot.Info().Name + " is " + e.Stage.String() + ": nothing to push"
			}
		} else {
			if e.Stage.Bound() {
				return nil, "the " + slot.Info().Name + " is already a " + e.Stage.String() + "; push it or wait for it to wear away"
			}
			r := rng.New(s.Seed, rng.Key("intervene"), uint64(s.Tick), uint64(origin), uint64(slot))
			g.To, g.Source, g.Prefix = morph.Periphrasis, int32(src), prefixFor(r, slot, v.M.Order)
			if e.Stage != morph.None && e.Stage != morph.Lost {
				g.Renew = true // replaces the free construction in use
			}
		}
		in.Kind, in.Gram = history.Gram, g
	case "order":
		oc, _ := parseOrder(iv.Rule)
		in.Kind, in.Order = history.Order, oc
	case "syllables":
		sc, _ := parseSyll(iv.Rule)
		in.Kind, in.Syll = history.Syllable, sc
	}
	return in, ""
}
