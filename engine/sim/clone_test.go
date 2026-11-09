package sim

import (
	"fmt"
	"hash/fnv"
	"testing"
)

// fingerprint covers more than Digest: polities and their spellings, the
// campaign, adoption curves, routes and the chronicle.
func fingerprint(s *Sim) string {
	h := fnv.New64a()
	fmt.Fprint(h, s.Digest(), len(s.Log), len(s.Hist), len(s.Tree), len(s.Routes), len(s.FixedRoutes))
	for _, e := range s.Log {
		fmt.Fprint(h, e.Tick, e.Kind, e.Text)
	}
	for _, p := range s.Polities {
		fmt.Fprint(h, p.Capital, p.Fell, p.Seafaring, p.Standard != nil, p.StandardTick, p.Classical)
		if p.Script != nil {
			fmt.Fprint(h, p.Script.Reformed, p.Script.Graph, p.Script.Spell)
		}
	}
	for _, in := range s.Reg.All {
		fmt.Fprint(h, in.Life, in.Curve)
	}
	if s.Camp != nil {
		fmt.Fprint(h, s.Camp.Influence, s.Camp.Earned, s.Camp.Spent, len(s.Camp.steer))
	}
	for _, c := range s.Communities() {
		fmt.Fprint(h, c.Polity, c.Outcome, len(c.Diffusing))
	}
	return fmt.Sprintf("%x", h.Sum64())
}

// TestCloneContinuesExactly: a clone taken mid-run continues exactly like
// the original, and running either leaves the other untouched (no shared
// mutable state): the clone runs ahead, then the original catches up, then
// they alternate tick by tick.
func TestCloneContinuesExactly(t *testing.T) {
	for _, tc := range []struct {
		name  string
		mk    func() *Sim
		at    int
		steer bool
	}{
		{"default", func() *Sim { return New(42, DefaultConfig()) }, 14, false},
		{"empire", func() *Sim { return mustStarter(t, "empire-collapse", 3) }, 30, false},
		{"campaign", func() *Sim { return mustStarter(t, "campaign-river-people", 5) }, 6, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "empire" {
				heavy(t)
			}
			const ahead, alt = 8, 8
			act := func(s *Sim) {
				if tc.steer && s.Tick == int32(tc.at) {
					// A paid action and a steered change, scheduled before the clone.
					_ = s.Intervene(Intervention{Kind: "script", Paid: true})
				}
			}
			diverge := func(d *Sim) {
				_ = d.Intervene(Intervention{Kind: "rule", Hex: -1, Rule: "rule all-a-to-o:\n    a => o\n"})
				if d.Camp != nil {
					_ = d.Intervene(Intervention{Kind: "standard"})
					for _, l := range d.live {
						_ = d.Intervene(Intervention{Kind: "suppress", Innov: l.in.ID})
						break
					}
				}
			}
			// References: the plain run, and a run that diverges at tc.at,
			// both without any cloning.
			track := func(div bool) []string {
				r := tc.mk()
				var fp []string
				for r.Tick < int32(tc.at+ahead+alt) {
					act(r)
					if div && r.Tick == int32(tc.at) {
						diverge(r)
					}
					r.Step()
					fp = append(fp, fingerprint(r))
				}
				return fp
			}
			want, wantD := track(false), track(true)
			if want[len(want)-1] == wantD[len(wantD)-1] {
				t.Fatal("the diverging run did not diverge")
			}
			check := func(who string, s *Sim, want []string) {
				t.Helper()
				if got := fingerprint(s); got != want[s.Tick-1] {
					t.Fatalf("%s at tick %d: %s, want %s", who, s.Tick, got, want[s.Tick-1])
				}
			}
			s := tc.mk()
			for s.Tick < int32(tc.at) {
				act(s)
				s.Step()
			}
			act(s)
			c, d := s.Clone(), s.Clone()
			diverge(d)
			for range ahead {
				c.Step()
				check("clone", c, want)
				d.Step()
				check("diverging clone", d, wantD)
			}
			for range ahead {
				s.Step()
				check("original", s, want)
			}
			for range alt {
				s.Step()
				check("original", s, want)
				d.Step()
				check("diverging clone", d, wantD)
				c.Step()
				check("clone", c, want)
			}
		})
	}
}

func mustStarter(t *testing.T, id string, seed uint64) *Sim {
	s, err := NewScenario(seed, DefaultConfig(), starter(t, id))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
