package sim

import (
	"slices"

	"isogloss/engine/lex"
)

// Retention is the per-mille share of core concepts a variety still
// expresses with the proto root (real languages keep ~800‰ per millennium).
func Retention(v *lex.Variety) int {
	kept, n := 0, 0
	for _, c := range lex.Concepts() {
		if !c.Core() {
			continue
		}
		n++
		if v.Lex.At(c.ID).Etym == lex.ProtoRoot(c.ID) {
			kept++
		}
	}
	return kept * 1000 / n
}

// Stats summarises a run for calibration.
type Stats struct {
	Seed        uint64
	Ticks       int32
	Languages   int
	Communities int
	Retention   [3]int // min, median, max over living languages (per-mille)
	Sounds      [3]int // sound changes in each language's history: min, median, max
}

// Summarise computes calibration statistics.
func (s *Sim) Summarise() Stats {
	st := Stats{Seed: s.Seed, Ticks: s.Tick, Communities: len(s.Communities())}
	var ret, snd []int
	for _, l := range s.LivingLanguages() {
		v := s.CentreVariety(l)
		ret = append(ret, Retention(v))
		n := 0
		for _, e := range v.Adopted() {
			if e.Kind == lex.EntryInnov && (e.Lo == 1 && e.Hi == 5 || e.Lo == e.Hi && (e.Lo == 1 || e.Lo == 5)) && s.Reg.Get(e.Innov).Kind == 0 {
				n++
			}
		}
		snd = append(snd, n)
	}
	st.Languages = len(ret)
	st.Retention = spread3(ret)
	st.Sounds = spread3(snd)
	return st
}

func spread3(xs []int) [3]int {
	if len(xs) == 0 {
		return [3]int{}
	}
	xs = slices.Clone(xs)
	slices.Sort(xs)
	return [3]int{xs[0], xs[len(xs)/2], xs[len(xs)-1]}
}
