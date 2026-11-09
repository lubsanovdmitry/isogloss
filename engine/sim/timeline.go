package sim

// Timeline keeps clones of a run so that looking back and forking need
// not replay from year 0. A snapshot is taken every Every ticks; past Max
// snapshots the spacing doubles and every other one is dropped, so memory
// stays bounded (a snapshot pins that moment's varieties: ~10 MB for a
// default run, ~30 MB when the whole map is settled).
type Timeline struct {
	Every int32
	Max   int
	snaps []*Sim // ascending by tick
}

// NewTimeline starts a timeline at s (its first snapshot).
func NewTimeline(s *Sim) *Timeline {
	tl := &Timeline{Every: 4, Max: 10}
	tl.snaps = []*Sim{s.Clone()}
	return tl
}

// Record takes a snapshot of s if it is due. Call it after every step.
func (tl *Timeline) Record(s *Sim) {
	if n := len(tl.snaps); n > 0 && tl.snaps[n-1].Tick >= s.Tick {
		return
	}
	if s.Tick%tl.Every != 0 {
		return
	}
	tl.snaps = append(tl.snaps, s.Clone())
	if len(tl.snaps) > tl.Max {
		tl.Every *= 2
		kept := tl.snaps[:0]
		for i, x := range tl.snaps {
			if i == 0 || x.Tick%tl.Every == 0 {
				kept = append(kept, x)
			}
		}
		clear(tl.snaps[len(kept):])
		tl.snaps = kept
	}
}

// Ticks lists the snapshot ticks.
func (tl *Timeline) Ticks() []int32 {
	out := make([]int32, len(tl.snaps))
	for i, x := range tl.snaps {
		out[i] = x.Tick
	}
	return out
}

// At rebuilds run s as it was at tick t (t ≤ s.Tick): a clone of the
// latest snapshot at or before t, stepped forward, carrying s's
// interventions up to t, the scenario's events and s's annotations. It is
// the run Replay(s.Fork(t)) gives (TestTimelineMatchesReplay).
func (tl *Timeline) At(s *Sim, t int32) *Sim {
	t = min(t, s.Tick)
	var base *Sim
	for _, x := range tl.snaps {
		if x.Tick <= t {
			base = x
		}
	}
	if base == nil {
		return Replay(s.Fork(t))
	}
	c := base.Clone()
	c.Interventions = keptAt(s.Interventions, t)
	c.Annotations = append([]Annotation(nil), s.Annotations...)
	for c.Tick < t {
		c.Step()
	}
	return c
}

// Truncate drops snapshots after tick t (the run was forked there).
func (tl *Timeline) Truncate(t int32) {
	n := 0
	for n < len(tl.snaps) && tl.snaps[n].Tick <= t {
		n++
	}
	clear(tl.snaps[n:])
	tl.snaps = tl.snaps[:n]
}
