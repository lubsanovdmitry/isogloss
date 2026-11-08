package sim

import (
	"testing"

	"isogloss/engine/society"
)

// Society shapes who talks to whom: within 1000 years polities form and some
// fall, trade routes link hubs, and foreign rule ends in each of its
// outcomes somewhere across a few seeds.
func TestSocietyRuns(t *testing.T) {
	outcomes := map[string]int{}
	var polities, routes, shifts int
	for seed := uint64(1); seed <= 4; seed++ {
		s := New(seed, DefaultConfig())
		s.Run(40)
		polities += len(s.Polities)
		routes += len(s.Routes)
		for _, e := range s.Log {
			switch e.Kind {
			case "rule":
				outcomes[e.Text[:min(len(e.Text), 12)]]++
			case "shift":
				shifts++
			}
		}
		for _, p := range s.Polities {
			if p.Capital < 0 || s.World.Hexes[p.Capital].Biome == 0 {
				t.Fatalf("polity %s has a bad capital", p.Name)
			}
			if p.Alive() {
				for _, c := range s.members(p) {
					if s.World.Distance(p.Capital, c.Hex) > maxReach {
						t.Errorf("%s reaches beyond %d hexes", p.Name, maxReach)
					}
				}
			}
		}
		for _, rt := range s.Routes {
			for i := 0; i+1 < len(rt.Path); i++ {
				if s.World.Distance(rt.Path[i], rt.Path[i+1]) != 1 {
					t.Fatalf("route path is not contiguous")
				}
			}
		}
	}
	t.Logf("polities %d, routes %d, shifts %d, rule outcomes %v", polities, routes, shifts, outcomes)
	if polities < 8 || routes < 8 {
		t.Errorf("too little society: %d polities, %d routes", polities, routes)
	}
	if shifts == 0 {
		t.Error("no language shift in four runs")
	}
}

func TestPath(t *testing.T) {
	s := New(3, DefaultConfig())
	a, b := s.World.Land[0], s.World.Land[len(s.World.Land)-1]
	p := society.Path(s.World, a, b, true, 1<<30)
	if p == nil || p[0] != a || p[len(p)-1] != b {
		t.Fatalf("no path from %d to %d across the sea: %v", a, b, p)
	}
}
