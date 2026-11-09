package sim

import (
	"slices"

	"isogloss/engine/lex"
	"isogloss/engine/phon"
	"isogloss/engine/rng"
	"isogloss/engine/tmpl"
)

// Suggestion is a sound change the engine finds plausible for a community:
// one of its templates filled in from the community's sounds, with the
// words it would change.
type Suggestion struct {
	Name, Family, Src string
	Score             uint32 // template weight × the simulation's own evaluation
	Count             int    // words changed
	Examples          []WordChange
}

// WordChange is one word before and after a change.
type WordChange struct {
	Concept       string
	Before, After string
}

// SuggestSounds offers up to n sound changes for the community at hex,
// best first: every template is filled in a few times from its inventory
// and scored as the simulation scores its own proposals (enough words
// affected, few homophones, typical results). salt varies the fillings.
// It reads the run and changes nothing.
func (s *Sim) SuggestSounds(hex int32, n int, salt uint64) []Suggestion {
	if hex < 0 || int(hex) >= len(s.ByHex) || s.ByHex[hex] == nil {
		return nil
	}
	v := s.ByHex[hex].Variety
	inv := v.Inventory()
	var out []Suggestion
	seen := map[string]bool{}
	for ti, t := range tmpl.Library() {
		r := rng.New(s.Seed, rng.Key("suggest"), uint64(s.Tick), uint64(hex), uint64(ti), salt)
		for try := 0; try < 3; try++ {
			cand, ok := t.Instantiate(r, inv)
			if !ok || seen[cand.Src] {
				continue
			}
			seen[cand.Src] = true
			score := uint64(s.evaluate(cand, v, inv)) * uint64(t.Weight) / 100
			if score == 0 {
				continue
			}
			sg := Suggestion{Name: t.Name, Family: t.Family, Src: cand.Src, Score: uint32(score)}
			for _, c := range lex.Concepts() {
				before := v.Lex.At(c.ID).Form
				after, ch := cand.Rule.Apply(before, &v.Phonotax)
				if !ch {
					continue
				}
				sg.Count++
				if len(sg.Examples) < 4 && c.Core() {
					sg.Examples = append(sg.Examples, WordChange{c.Label, phon.RenderWord(before), phon.RenderWord(after)})
				}
			}
			out = append(out, sg)
		}
	}
	// Best first; at most one filling per template among the first n.
	slices.SortStableFunc(out, func(a, b Suggestion) int { return int(b.Score) - int(a.Score) })
	var picked []Suggestion
	used := map[string]bool{}
	for _, sg := range out {
		if len(picked) == n {
			break
		}
		if !used[sg.Name] {
			used[sg.Name] = true
			picked = append(picked, sg)
		}
	}
	return picked
}
