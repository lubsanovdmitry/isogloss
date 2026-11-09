package sim

import (
	"os"
	"slices"
	"strings"
	"testing"

	"isogloss/engine/history"
	"isogloss/engine/scenario"
)

// A strategy decides the player's actions at the start of each generation.
type strategy struct {
	name string
	act  func(s *Sim)
}

func afford(s *Sim, kind string) bool { return s.Camp.Influence >= ActionCost[kind] }

func (s *Sim) campPolity() bool {
	return s.Camp != nil && s.Polities[s.Camp.Polity].Alive()
}

// expandTarget picks the most populous settled hex next to the player's
// land that the player does not hold.
func expandTarget(s *Sim) int32 {
	p := s.Polities[s.Camp.Polity]
	best, bestPop := int32(-1), int32(0)
	for _, c := range s.members(p) {
		for _, nb := range s.World.Hexes[c.Hex].Nbr {
			o := s.byHex(nb)
			if o != nil && o.Polity != p.ID && o.Pop > bestPop {
				best, bestPop = nb, o.Pop
			}
		}
	}
	return best
}

func writer(s *Sim) {
	p := s.Polities[s.Camp.Polity]
	switch {
	case p.Script == nil && afford(s, "script"):
		_ = s.Intervene(Intervention{Kind: "script"})
	case p.Script != nil && p.Standard == nil && afford(s, "standard"):
		_ = s.Intervene(Intervention{Kind: "standard"})
	}
}

func expander(s *Sim) {
	for afford(s, "expand") {
		h := expandTarget(s)
		if h < 0 {
			return
		}
		if err := s.Intervene(Intervention{Kind: "expand", Hex: h}); err != nil {
			return
		}
		s.Camp.Influence -= ActionCost["expand"] // reserve: paid when it happens
		defer func() { s.Camp.Influence += ActionCost["expand"] }()
	}
}

// steerer encourages tonogenesis reaching the player's land and suppresses
// changes that would take the final glottal stops away without leaving tone
// (creaky voice, final-consonant loss).
func steerer(s *Sim) {
	ids, _, steer := s.TerritoryInnovations()
	for i, id := range ids {
		if steer[i] != 0 {
			continue
		}
		in := s.Reg.Get(id)
		if in.Kind != history.Sound {
			continue
		}
		switch {
		case strings.HasPrefix(in.Template, "tonogenesis") || in.Template == "register-to-low-tone" || in.Template == "s-debuccalization":
			if afford(s, "encourage") {
				_ = s.Intervene(Intervention{Kind: "encourage", Innov: id})
				return
			}
		case in.Template == "glottal-to-creaky" || in.Template == "final-obstruent-loss" || in.Template == "h-loss" || in.Template == "final-s-loss":
			if afford(s, "suppress") {
				_ = s.Intervene(Intervention{Kind: "suppress", Innov: id})
				return
			}
		}
	}
}

var strategies = []strategy{
	{"idle", func(*Sim) {}},
	{"writer", writer},
	{"expander", expander},
	{"steerer", steerer},
	{"balanced", func(s *Sim) {
		steerer(s)
		// A standard slows change at the capital: no use when it has
		// to become tonal.
		if !slices.ContainsFunc(s.Camp.Spec.Goals, func(g scenario.Goal) bool { return g.Kind == "tonal" }) {
			writer(s)
		}
		if s.Camp.Influence >= 200 { // keep a reserve for steering
			expander(s)
		}
	}},
}

type campaignResult struct {
	met          map[string]bool
	goals, total int
	score        int64
	earned, left int64
	survived     bool
}

func playCampaign(t *testing.T, id string, seed uint64, st strategy) campaignResult {
	sc := starter(t, id)
	s, err := NewScenario(seed, DefaultConfig(), sc)
	if err != nil {
		t.Fatal(err)
	}
	for s.Tick < int32(sc.Ticks) {
		if s.campPolity() {
			st.act(s)
		}
		s.Step()
	}
	goals, score := s.CampaignStatus()
	r := campaignResult{met: map[string]bool{}, total: len(goals), score: score, earned: s.Camp.Earned, left: s.Camp.Influence, survived: s.campPolity()}
	for _, g := range goals {
		if g.Met {
			r.goals++
			r.met[g.Goal.Kind] = true
		}
	}
	return r
}

// TestCampaignBalance plays fixed strategies over several seeds of both
// campaign starters. Strategy has to matter: the balanced player beats the
// idle one on the river, and steering makes the delta tonal more often
// (there, tone costs intelligibility, so the "spoken" goal pulls the other
// way). No strategy wins everything, and a good player finds something
// worth spending most of its influence on. Set ISOGLOSS_BALANCE=1 for the
// full table with the single-minded strategies too.
func TestCampaignBalance(t *testing.T) {
	heavy(t)
	seeds := []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	for _, id := range []string{"campaign-river-people", "campaign-song-of-the-delta"} {
		avg := map[string]float64{}
		goalRate := map[string]map[string]int{}
		for _, st := range strategies {
			if os.Getenv("ISOGLOSS_BALANCE") == "" && (st.name == "writer" || st.name == "expander") {
				continue // the full report: ISOGLOSS_BALANCE=1
			}
			var goals, total int
			var score, earned, left int64
			survived := 0
			goalRate[st.name] = map[string]int{}
			for _, seed := range seeds {
				r := playCampaign(t, id, seed, st)
				goals += r.goals
				total += r.total
				score += r.score
				earned += r.earned
				left += r.left
				if r.survived {
					survived++
				}
				for k := range r.met { //lint:ordered counting
					goalRate[st.name][k]++
				}
			}
			n := int64(len(seeds))
			avg[st.name] = float64(goals) / float64(total)
			t.Logf("%-26s %-9s goals %2d/%2d %v  score %5d  survived %d/%d  earned %5d  unspent %3d%%",
				id, st.name, goals, total, goalRate[st.name], score/n, survived, len(seeds), earned/n, left*100/max(earned, 1))
			if avg[st.name] == 1 {
				t.Errorf("%s: %s meets every goal in every run", id, st.name)
			}
		}
		switch id {
		case "campaign-river-people":
			if avg["balanced"] <= avg["idle"] {
				t.Errorf("%s: playing does not help (balanced %.2f, idle %.2f)", id, avg["balanced"], avg["idle"])
			}
		case "campaign-song-of-the-delta":
			if goalRate["steerer"]["tonal"] <= goalRate["idle"]["tonal"] {
				t.Errorf("%s: steering does not make the capital tonal more often (%d vs %d)", id, goalRate["steerer"]["tonal"], goalRate["idle"]["tonal"])
			}
		}
	}
}
