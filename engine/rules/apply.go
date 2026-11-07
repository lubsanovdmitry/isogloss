package rules

import "isogloss/engine/phon"

// Match reports whether segment i of w matches the pattern.
func (pat *SegPat) Match(w *phon.Word, i int) bool {
	s := w.Segs[i]
	if !s.F.Matches(pat.Feat) {
		return false
	}
	if pat.Alts != nil && pat.altIndex(s.F) < 0 {
		return false
	}
	for _, c := range pat.Classes {
		if !c.Pat.Match(w, i) {
			return false
		}
	}
	if pat.Stress >= 0 && w.StressOf(i) != pat.Stress {
		return false
	}
	if pat.Tone >= 0 && s.Tone != phon.Tone(pat.Tone) {
		return false
	}
	if pat.Phon >= 0 && s.Phon() != phon.Phonation(pat.Phon) {
		return false
	}
	return true
}

func (pat *SegPat) altIndex(f phon.FeatureSet) int {
	for k, a := range pat.Alts {
		if a == f {
			return k
		}
	}
	return -1
}

// corrIndex finds the index of segment i in the target pattern's list
// (directly or through a single list class).
func (pat *SegPat) corrIndex(f phon.FeatureSet) int {
	if pat.Alts != nil {
		return pat.altIndex(f)
	}
	for _, c := range pat.Classes {
		if k := c.Pat.corrIndex(f); k >= 0 {
			return k
		}
	}
	return -1
}

func boundAt(w *phon.Word, pos int, b byte) bool {
	n := len(w.Segs)
	switch b {
	case '#':
		if pos == 0 || pos == n {
			return true
		}
		return hasBound(w, pos, phon.BWord)
	case '+':
		return hasBound(w, pos, phon.BMorpheme)
	case '=':
		return hasBound(w, pos, phon.BClitic)
	case '.':
		if pos == 0 || pos == n {
			return true
		}
		for _, s := range w.Syll {
			if s.Start == pos {
				return true
			}
		}
	}
	return false
}

func hasBound(w *phon.Word, pos int, k phon.BoundaryKind) bool {
	for _, b := range w.Bounds {
		if b.At == pos && b.Kind == k {
			return true
		}
	}
	return false
}

// matchSeq matches elems starting at pos, moving forward (dir=+1) or
// backward (dir=-1, elems consumed from last to first), calling k with each
// possible end position until k returns true.
func matchSeq(w *phon.Word, elems []Elem, pos, dir int, k func(int) bool) bool {
	if len(elems) == 0 {
		return k(pos)
	}
	var e Elem
	var rest []Elem
	if dir > 0 {
		e, rest = elems[0], elems[1:]
	} else {
		e, rest = elems[len(elems)-1], elems[:len(elems)-1]
	}
	next := func(p int) bool { return matchSeq(w, rest, p, dir, k) }
	switch e.Kind {
	case EBound:
		return boundAt(w, pos, e.Bound) && next(pos)
	case EOpt:
		if matchSeq(w, e.Group, pos, dir, next) {
			return true
		}
		return next(pos)
	}
	step := func(p int) (int, bool) {
		i := p
		if dir < 0 {
			i = p - 1
		}
		if i < 0 || i >= len(w.Segs) || !e.Seg.Match(w, i) {
			return p, false
		}
		return p + dir, true
	}
	if !e.Star {
		p, ok := step(pos)
		return ok && next(p)
	}
	// Greedy with backtracking.
	stops := []int{pos}
	for p := pos; ; {
		np, ok := step(p)
		if !ok {
			break
		}
		stops = append(stops, np)
		p = np
	}
	for j := len(stops) - 1; j >= 0; j-- {
		if next(stops[j]) {
			return true
		}
	}
	return false
}

func envMatches(w *phon.Word, env Env, start, end int) bool {
	return matchSeq(w, env.Left, start, -1, func(int) bool { return true }) &&
		matchSeq(w, env.Right, end, +1, func(int) bool { return true })
}

func (c *Clause) contextOK(w *phon.Word, start, end int) bool {
	ok := len(c.Envs) == 0
	for _, e := range c.Envs {
		if envMatches(w, e, start, end) {
			ok = true
			break
		}
	}
	if !ok {
		return false
	}
	for _, e := range c.Except {
		if envMatches(w, e, start, end) {
			return false
		}
	}
	return true
}

type site struct {
	start, end  int
	beforeBound bool // insertion goes before a boundary at start
}

// Sites returns the non-overlapping positions where the clause applies to w,
// scanning left to right. All contexts are evaluated on the input word, so
// application is simultaneous.
func (c *Clause) sites(w *phon.Word) []site {
	var out []site
	n := len(w.Segs)
	if len(c.Target) == 0 {
		for i := 0; i <= n; i++ {
			if !c.contextOK(w, i, i) {
				continue
			}
			before := false
			for _, e := range c.Envs {
				if len(e.Right) > 0 && e.Right[0].Kind == EBound && envMatches(w, e, i, i) {
					before = true
				}
			}
			out = append(out, site{i, i, before})
		}
		return out
	}
	for i := 0; i < n; {
		end := -1
		matchSeq(w, c.Target, i, +1, func(p int) bool { end = p; return true })
		if end >= 0 && c.contextOK(w, i, end) {
			out = append(out, site{start: i, end: end})
			i = end
			continue
		}
		i++
	}
	return out
}

// transfer copies stress and tone from a matched nucleus onto the first
// syllabic segment of a literal replacement.
func transfer(src []phon.Segment, lit []phon.Segment) []phon.Segment {
	out := append([]phon.Segment(nil), lit...)
	var stress int8
	var tone phon.Tone
	for _, s := range src {
		if s.IsVowel() {
			if s.Stress > stress {
				stress = s.Stress
			}
			if tone == phon.ToneNone {
				tone = s.Tone
			}
		}
	}
	for i := range out {
		if out[i].IsVowel() {
			if out[i].Stress == 0 {
				out[i].Stress = stress
			}
			if out[i].Tone == phon.ToneNone {
				out[i].Tone = tone
			}
			break
		}
	}
	return out
}

func (c *Clause) replace(w *phon.Word, s site) []phon.Segment {
	matched := w.Segs[s.start:s.end]
	if len(c.Target) == 0 {
		var out []phon.Segment
		for _, o := range c.Repl {
			out = append(out, o.Lit...)
		}
		return out
	}
	if len(c.Repl) != len(c.Target) {
		if len(c.Repl) == 1 && c.Repl[0].Kind == ODelete {
			return nil
		}
		var lit []phon.Segment
		for _, o := range c.Repl {
			lit = append(lit, o.Lit...)
		}
		return transfer(matched, lit)
	}
	var out []phon.Segment
	for k, o := range c.Repl {
		seg := matched[k]
		switch o.Kind {
		case ODelete:
		case OLit:
			out = append(out, transfer([]phon.Segment{seg}, o.Lit)...)
		case OMatrix:
			seg.F = seg.F.Apply(o.Delta)
			if o.Stress >= 0 {
				seg.Stress = o.Stress
			}
			if o.Tone >= 0 {
				seg.Tone = phon.Tone(o.Tone)
			}
			if o.Phon >= 0 {
				seg = seg.WithPhon(phon.Phonation(o.Phon))
			}
			out = append(out, seg)
		case OList:
			idx := c.Target[k].Seg.corrIndex(seg.F)
			if len(o.List) == 1 {
				idx = 0
			}
			if idx < 0 || idx >= len(o.List) {
				out = append(out, seg) // no correspondent: unchanged
				continue
			}
			out = append(out, transfer([]phon.Segment{seg}, o.List[idx])...)
		}
	}
	return out
}

// ApplyClause applies one clause to w, returning the new word and whether it
// changed. Syllables are recomputed with ph (nil = default phonotactics).
func (c *Clause) Apply(w phon.Word, ph *phon.Phonotactics) (phon.Word, bool) {
	sites := c.sites(&w)
	if len(sites) == 0 {
		return w, false
	}
	n := len(w.Segs)
	out := phon.Word{Segs: make([]phon.Segment, 0, n+2)}
	newPos := make([]int, n+1)
	si := 0
	for i := 0; i <= n; {
		var s *site
		if si < len(sites) && sites[si].start == i {
			s = &sites[si]
		}
		if s != nil && s.start == s.end && s.beforeBound {
			out.Segs = append(out.Segs, c.replace(&w, *s)...)
		}
		newPos[i] = len(out.Segs)
		if s != nil && s.start == s.end && !s.beforeBound {
			out.Segs = append(out.Segs, c.replace(&w, *s)...)
		}
		if s != nil {
			si++
		}
		if i == n {
			break
		}
		if s != nil && s.end > s.start {
			out.Segs = append(out.Segs, c.replace(&w, *s)...)
			for j := i + 1; j < s.end; j++ {
				newPos[j] = len(out.Segs)
			}
			i = s.end
			continue
		}
		out.Segs = append(out.Segs, w.Segs[i])
		i++
	}
	for _, b := range w.Bounds {
		at := newPos[b.At]
		if at == 0 || at >= len(out.Segs) {
			continue
		}
		if k := len(out.Bounds); k > 0 && out.Bounds[k-1] == (phon.Boundary{At: at, Kind: b.Kind}) {
			continue
		}
		out.Bounds = append(out.Bounds, phon.Boundary{At: at, Kind: b.Kind})
	}
	phon.CleanStress(&out, ph)
	if out.Equal(w) { // compares segments including stress and tone
		return w, false
	}
	return out, true
}

// Apply applies the rule's clauses in order. It reports a change only if the
// final word differs from the input (clauses may undo each other).
func (r *Rule) Apply(w phon.Word, ph *phon.Phonotactics) (phon.Word, bool) {
	out := w
	for _, c := range r.Clauses {
		out, _ = c.Apply(out, ph)
	}
	if out.Equal(w) {
		return w, false
	}
	return out, true
}

// Step records one rule's effect on a word.
type Step struct {
	Rule          *Rule
	Before, After phon.Word
}

// ApplyAll applies every rule in the file in order and returns the result
// and the steps that changed the word.
func (f *File) ApplyAll(w phon.Word, ph *phon.Phonotactics) (phon.Word, []Step) {
	phon.CleanStress(&w, ph)
	var steps []Step
	for _, r := range f.Rules {
		nw, ch := r.Apply(w, ph)
		if ch {
			steps = append(steps, Step{Rule: r, Before: w, After: nw})
			w = nw
		}
	}
	return w, steps
}
