package pop

import (
	"isogloss/engine/fx"
	"isogloss/engine/lex"
	"isogloss/engine/phon"
	"isogloss/engine/world"
)

// Community is the speech community of one settled hex.
type Community struct {
	Hex       int32
	Pop       int32
	Variety   *lex.Variety
	Lang      int32 // language ID from the last clustering, -1 before
	Founded   int32
	Diffusing []Diffusion // lexically diffusing changes still creeping through
	// Name is the hex's place name as the community took it over (from a
	// substrate, or the proto-language's own); the variety's sound changes
	// from history entry NameAt on reshape it (history.Evolve).
	Name   phon.Word
	NameAt int32
}

// Diffusion tracks one diffusible change in one community.
type Diffusion struct {
	Innov uint32
	Next  uint8 // next frequency class to reach
	Step  int8  // +1 freq-first (frequent words first), -1 rare-first
}

// Contact is a weighted link between neighbouring communities.
type Contact struct {
	To int32
	W  fx.Q16
}

// maxContact caps one link's weight; contactScale/cost gives the rest.
var (
	maxContact   = fx.FromPermille(450)
	contactScale = uint32(40)
)

// ContactWeights computes links between settled neighbouring hexes, from
// terrain crossing costs. Sea is closed (seafaring arrives with Society).
func ContactWeights(w *world.World, settled []bool) [][]Contact {
	out := make([][]Contact, len(w.Hexes))
	for i := range w.Hexes {
		if !settled[i] {
			continue
		}
		for _, nb := range w.Hexes[i].Nbr {
			if nb < 0 || !settled[nb] {
				continue
			}
			cost := w.Cost(int32(i), nb)
			if cost <= 0 {
				continue
			}
			c := fx.FromRatio(contactScale, uint32(cost))
			if c > maxContact {
				c = maxContact
			}
			out[i] = append(out[i], Contact{nb, c})
		}
	}
	return out
}

// Prestige maps population to a spread weight in [0.6, 1.4].
func Prestige(pop, maxPop int32) fx.Q16 {
	if maxPop <= 0 {
		return fx.One
	}
	return fx.FromPermille(600) + fx.FromRatio(uint32(pop)*800/uint32(maxPop), 1000)
}
