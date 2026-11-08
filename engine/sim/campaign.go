package sim

import (
	"isogloss/engine/history"
	"isogloss/engine/lex"
	"isogloss/engine/phon"
	"isogloss/engine/pop"
	"isogloss/engine/scenario"
)

// Campaign is the steering game's state: the player's polity and the
// influence they have to spend.
type Campaign struct {
	Spec      *scenario.Campaign
	Polity    int32 // the player's polity
	Influence int64
	Earned    int64
	Spent     int64
	steer     map[uint32]int8 // innovation → +1 encouraged, -1 suppressed in the player's territory
}

// ActionCost is what each player action costs in influence.
var ActionCost = map[string]int64{
	"script": 150, "standard": 200, "seafaring": 150, "route": 100,
	"encourage": 40, "suppress": 60, "expand": 30, "conquer": 120,
}

// setupCampaign founds the player's polity.
func (s *Sim) setupCampaign(sc *scenario.Campaign) error {
	h, err := s.Place(sc.People)
	if err != nil {
		return err
	}
	c := s.ByHex[h]
	if c == nil {
		return errNotSettled(sc.People)
	}
	p := c.Polity
	if p < 0 || !s.Polities[p].Alive() {
		p = s.foundPolity(c).ID
	}
	s.Camp = &Campaign{Spec: sc, Polity: p, Influence: int64(sc.Start), steer: map[uint32]int8{}}
	return nil
}

type errNotSettled string

func (e errNotSettled) Error() string {
	return "scenario: " + string(e) + " is not settled at the start"
}

// income: influence grows with the polity's people and the capital's
// standing (writing and a standard add to it).
func (s *Sim) income() {
	cp := s.Camp
	if cp == nil {
		return
	}
	p := s.Polities[cp.Polity]
	if !p.Alive() {
		return
	}
	var people int64
	for _, c := range s.members(p) {
		people += int64(c.Pop)
	}
	gain := people/250 + 5
	if p.Script != nil {
		gain += 5
	}
	if p.Standard != nil {
		gain += 5
	}
	gain = gain * int64(cp.Spec.Income) / 1000
	cp.Influence += gain
	cp.Earned += gain
}

// pay charges a player action; false (and a chronicle note) when the
// player cannot afford it.
func (s *Sim) pay(iv Intervention) bool {
	cp := s.Camp
	if cp == nil || !iv.Paid {
		return true
	}
	cost := ActionCost[iv.Kind]
	if cp.Influence < cost {
		s.log(history.Event{Kind: "campaign", Hex: iv.Hex, Lang: -1, Text: "not enough influence to " + iv.Kind})
		return false
	}
	cp.Influence -= cost
	cp.Spent += cost
	return true
}

// steering returns the player's steer on an innovation in hex h: ×1000
// factor on the local rate.
func (s *Sim) steering(id uint32, h int) int64 {
	cp := s.Camp
	if cp == nil || len(cp.steer) == 0 {
		return 1000
	}
	d := cp.steer[id]
	if d == 0 || s.ByHex[h] == nil || s.ByHex[h].Polity != cp.Polity {
		return 1000
	}
	if d > 0 {
		return 2500
	}
	return 150
}

// expand: the player's polity takes a neighbouring hex.
func (s *Sim) expand(iv Intervention) {
	cp := s.Camp
	t := s.byHex(iv.Hex)
	if cp == nil || t == nil || t.Polity == cp.Polity {
		return
	}
	p := s.Polities[cp.Polity]
	if !p.Alive() {
		return
	}
	adjacent := false
	for _, nb := range s.World.Hexes[t.Hex].Nbr {
		if o := s.byHex(nb); o != nil && o.Polity == p.ID {
			adjacent = true
		}
	}
	if !adjacent {
		s.log(history.Event{Kind: "campaign", Hex: t.Hex, Lang: t.Lang, Text: p.Name + " cannot reach that land"})
		return
	}
	if op := s.polityOf(t); op != nil && t.Hex == op.Capital {
		s.fall(op, "its capital falls to "+p.Name)
	}
	s.join(t, p)
	s.log(history.Event{Kind: "conquest", Hex: t.Hex, Lang: t.Lang, Text: p.Name + " takes in a neighbouring community"})
}

// GoalStatus is one goal's progress.
type GoalStatus struct {
	Goal     scenario.Goal
	Value    int32 // current value (hexes, languages, 0/1)
	Met      bool
	Describe string
}

// CampaignStatus evaluates the goals and the score: speakers of the
// people's language (per hundred), 50 per daughter language, 300 per goal
// met and 200 while the polity stands.
func (s *Sim) CampaignStatus() (goals []GoalStatus, score int64) {
	cp := s.Camp
	if cp == nil {
		return nil, 0
	}
	p := s.Polities[cp.Polity]
	capC := s.ByHex[p.Capital]
	var speakers int64
	spoken := int32(0)
	var speaking []*pop.Community
	if capC != nil {
		for _, c := range s.Communities() {
			if s.Intel(capC.Variety, c.Variety) >= s.Threshold() {
				spoken++
				speakers += int64(c.Pop)
				speaking = append(speaking, c)
			}
		}
	}
	daughters := int32(0)
	if capC != nil {
		for _, l := range s.LivingLanguages() {
			if s.CentreVariety(l).Family == capC.Variety.Family {
				daughters++
			}
		}
	}
	for _, g := range cp.Spec.Goals {
		st := GoalStatus{Goal: g}
		switch g.Kind {
		case "spoken":
			st.Value, st.Met = spoken, spoken >= g.N
			st.Describe = "still spoken in " + itoa(int(g.N)) + " hexes"
		case "unity":
			st.Value = 0
			if p.Alive() && capC != nil {
				far := capC
				for _, c := range s.members(p) {
					if s.World.Distance(p.Capital, c.Hex) > s.World.Distance(p.Capital, far.Hex) {
						far = c
					}
				}
				if s.Intel(capC.Variety, far.Variety) >= s.Threshold() {
					st.Value = 1
				}
			}
			st.Met = st.Value == 1
			st.Describe = "the capital and the farthest frontier still understand each other"
		case "tonal":
			if capC != nil && tonal(capC.Variety) {
				st.Value = 1
			}
			st.Met = st.Value == 1
			st.Describe = "the capital's speech becomes tonal"
		case "daughters":
			st.Value, st.Met = daughters, daughters >= g.N
			st.Describe = itoa(int(g.N)) + " or more daughter languages"
		case "survive":
			if p.Alive() {
				st.Value = 1
			}
			st.Met = st.Value == 1
			st.Describe = "the polity still stands"
		}
		if st.Met {
			score += 300
		}
		goals = append(goals, st)
	}
	score += speakers/100 + 50*int64(daughters)
	if p.Alive() {
		score += 200
	}
	return goals, score
}

// tonal reports whether at least a tenth of a variety's vowels carry tone.
func tonal(v *lex.Variety) bool {
	t, n := 0, 0
	for i := 0; i < v.Lex.Len(); i++ {
		for _, sg := range v.Lex.At(lex.ConceptID(i)).Form.Segs {
			if sg.IsVowel() {
				n++
				if sg.Tone != phon.ToneNone {
					t++
				}
			}
		}
	}
	return n > 0 && t*10 >= n
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// TerritoryInnovations lists innovations still spreading that have reached
// the player's land, with how many of its communities adopted them and the
// player's steer (+1, -1, 0).
func (s *Sim) TerritoryInnovations() (ids []uint32, applied []int, steer []int8) {
	cp := s.Camp
	if cp == nil {
		return
	}
	for _, l := range s.live {
		if l.age >= l.in.Life || l.in.Kind == history.Lexical || l.in.Kind == history.Loan {
			continue
		}
		n, reach := 0, 0
		for h, c := range s.ByHex {
			if c == nil || c.Polity != cp.Polity {
				continue
			}
			if l.wave.Applied[h] {
				n++
			}
			if l.wave.Level[h] > 0 || l.wave.Applied[h] {
				reach++
			}
		}
		if reach > 0 {
			ids = append(ids, l.in.ID)
			applied = append(applied, n)
			steer = append(steer, cp.steer[l.in.ID])
		}
	}
	// Most widespread first (stable, so ties stay in innovation order).
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && applied[j] > applied[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
			applied[j], applied[j-1] = applied[j-1], applied[j]
			steer[j], steer[j-1] = steer[j-1], steer[j]
		}
	}
	return
}
