package sim

import "testing"

// TestTimelineMatchesReplay: rebuilding a past tick from the timeline gives
// the same run as replaying the forked save from year 0, with snapshots
// thinned out along the way.
func TestTimelineMatchesReplay(t *testing.T) {
	s := New(42, DefaultConfig())
	tl := NewTimeline(s)
	tl.Max = 3 // force thinning within 30 ticks
	for s.Tick < 30 {
		switch s.Tick {
		case 9:
			if err := s.Intervene(Intervention{Kind: "rule", Hex: -1, Rule: "rule p-to-f:\n    p => f\n"}); err != nil {
				t.Fatal(err)
			}
		case 17:
			if err := s.Intervene(Intervention{Kind: "plague", Hex: -1}); err != nil {
				t.Fatal(err)
			}
		}
		s.Step()
		tl.Record(s)
	}
	if tl.Every <= 4 || len(tl.Ticks()) > 3 {
		t.Errorf("not thinned: every %d, snapshots at %v", tl.Every, tl.Ticks())
	}
	for _, at := range []int32{0, 3, 10, 18, 23, 30} {
		got := tl.At(s, at)
		want := Replay(s.Fork(at))
		if got.Tick != at || fingerprint(got) != fingerprint(want) {
			t.Errorf("tick %d: timeline and replay differ (snapshots %v)", at, tl.Ticks())
		}
	}
	// A fork continues like the replayed save.
	f := tl.At(s, 10)
	tl.Truncate(10)
	r := Replay(s.Fork(10))
	for f.Tick < 16 {
		f.Step()
		tl.Record(f)
		r.Step()
	}
	if fingerprint(f) != fingerprint(r) {
		t.Error("forked run diverges from the replayed fork")
	}
}

// TestForkKeepsScenarioEvents: a fork drops the player's later actions but
// keeps what the scenario has scheduled, also for saves that predate the
// Scripted flag.
func TestForkKeepsScenarioEvents(t *testing.T) {
	s := mustStarter(t, "empire-collapse", 7)
	var late int
	for _, iv := range s.Interventions {
		if iv.Tick > 2 && iv.Scripted {
			late++
		}
	}
	if late == 0 {
		t.Fatal("empire-collapse schedules no later events")
	}
	if err := s.Intervene(Intervention{Kind: "plague", Hex: -1}); err != nil {
		t.Fatal(err)
	}
	s.Step()
	s.Step()
	sv := s.Fork(0)
	n := 0
	for _, iv := range sv.Interventions {
		if iv.Kind == "plague" && !iv.Scripted {
			t.Error("the player's later plague survived the fork")
		}
		if iv.Scripted {
			n++
		}
	}
	if n < late {
		t.Errorf("fork kept %d scripted events, want %d", n, late)
	}
	// An old save without the flag: replay recognises the events.
	for i := range sv.Interventions {
		sv.Interventions[i].Scripted = false
	}
	r := Replay(sv)
	m := 0
	for _, iv := range r.Interventions {
		if iv.Scripted {
			m++
		}
	}
	if m != n {
		t.Errorf("replay marked %d events scripted, want %d", m, n)
	}
}
