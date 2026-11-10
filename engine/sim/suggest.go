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
// affected, few homophones, typical results, rare sounds retired, aimed at
// the community's rare sounds; the templates of its last few sound changes
// count less). Scores are
// jittered — by up to a fifth for salt 0, by up to three fifths for
// "other suggestions" — so the list is not the weight table in order, and
// no family takes more than two places. salt also varies the fillings. It
// reads the run and changes nothing.
func (s *Sim) SuggestSounds(hex int32, n int, salt uint64) []Suggestion {
	if hex < 0 || int(hex) >= len(s.ByHex) || s.ByHex[hex] == nil {
		return nil
	}
	v := s.ByHex[hex].Variety
	inv := v.Inventory()
	var out []Suggestion
	seen := map[string]bool{}
	recent := s.recentSounds(v)
	rar := rarities(inv)
	spread := uint64(200)
	if salt > 0 {
		spread = 600
	}
	for ti, t := range tmpl.Library() {
		r := rng.New(s.Seed, rng.Key("suggest"), uint64(s.Tick), uint64(hex), uint64(ti), salt)
		jr := rng.New(s.Seed, rng.Key("suggest-jitter"), uint64(s.Tick), uint64(hex), uint64(ti), salt)
		for try := 0; try < 3; try++ {
			cand, ok := t.Instantiate(r, inv)
			if !ok || seen[cand.Src] {
				continue
			}
			seen[cand.Src] = true
			score := uint64(s.evaluate(cand, v, inv)) * uint64(t.Weight) / 100 * aimFactor(t, inv, rar) / 1000
			if score == 0 {
				continue
			}
			if recent[t.Name] {
				score = score * 300 / 1000
			}
			score = score * (1000 - spread + uint64(jr.IntN(int(2*spread)+1))) / 1000
			score = max(score, 1)
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
	// Best first; at most one filling per template and two per family
	// among the first n.
	slices.SortStableFunc(out, func(a, b Suggestion) int { return int(b.Score) - int(a.Score) })
	var picked []Suggestion
	used, fam := map[string]bool{}, map[string]int{}
	for _, sg := range out {
		if len(picked) == n {
			break
		}
		if !used[sg.Name] && fam[sg.Family] < 2 {
			used[sg.Name] = true
			fam[sg.Family]++
			picked = append(picked, sg)
		}
	}
	return picked
}
