package sim

import (
	"testing"

	"isogloss/engine/history"
)

// A campaign: influence accrues, actions cost it (and are refused when it
// runs short), steering changes adoption in the player's land, goals are
// evaluated, and a run with player actions replays exactly.
func TestCampaign(t *testing.T) {
	sc := starter(t, "campaign-river-people")
	s, err := NewScenario(5, DefaultConfig(), sc)
	if err != nil {
		t.Fatal(err)
	}
	if s.Camp == nil {
		t.Fatal("no campaign")
	}
	if err := s.Intervene(Intervention{Kind: "rule", Hex: -1, Rule: "rule x:\n    a => e\n"}); err == nil {
		t.Error("god-mode sound change allowed in a campaign")
	}
	s.Run(6)
	before := s.Camp.Influence
	if s.Camp.Earned == 0 {
		t.Fatal("no income")
	}
	if err := s.Intervene(Intervention{Kind: "script", Rule: "abjad"}); err != nil {
		t.Fatal(err)
	}
	// Encourage the newest live sound change in the player's land.
	var target uint32
	for _, l := range s.live {
		if l.in.Kind == history.Sound {
			target = l.in.ID
		}
	}
	if target != 0 {
		if err := s.Intervene(Intervention{Kind: "encourage", Innov: target}); err != nil {
			t.Fatal(err)
		}
	}
	s.Step()
	p := s.Polities[s.Camp.Polity]
	if p.Script == nil || p.Script.Type.String() != "abjad" {
		t.Error("script not adopted")
	}
	if s.Camp.Spent < ActionCost["script"] || s.Camp.Influence > before+s.Camp.Earned {
		t.Errorf("not charged: spent %d", s.Camp.Spent)
	}
	// Actions beyond the purse are refused at their tick.
	for k := 0; k < 12; k++ {
		if err := s.Intervene(Intervention{Kind: "seafaring"}); err != nil {
			t.Fatal(err)
		}
	}
	spent := s.Camp.Spent
	s.Step()
	if s.Camp.Influence < 0 || s.Camp.Spent-spent >= 12*ActionCost["seafaring"] {
		t.Errorf("overspent: influence %d", s.Camp.Influence)
	}
	refused := false
	for _, e := range s.Log {
		if e.Kind == "campaign" && e.Tick == s.Tick && e.Text == "not enough influence to seafaring" {
			refused = true
		}
	}
	if !refused {
		t.Error("no refusal logged")
	}
	s.Run(10)
	goals, score := s.CampaignStatus()
	if len(goals) != 4 || score <= 0 {
		t.Errorf("goals %v score %d", goals, score)
	}
	for _, g := range goals {
		t.Logf("%-9s %3d met %v (%s)", g.Goal.Kind, g.Value, g.Met, g.Describe)
	}
	t.Logf("score %d, influence %d (earned %d, spent %d)", score, s.Camp.Influence, s.Camp.Earned, s.Camp.Spent)
	r := Replay(s.SaveState())
	if r.Digest() != s.Digest() || r.Camp.Influence != s.Camp.Influence {
		t.Error("campaign replay differs")
	}
}
