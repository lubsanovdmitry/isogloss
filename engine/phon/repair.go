package phon

// RepairMode is how a language fixes syllables its template forbids.
type RepairMode uint8

const (
	RepairEpenthesis RepairMode = iota // insert the epenthetic vowel
	RepairDeletion                     // delete the offending consonant
)

func (m RepairMode) String() string {
	if m == RepairDeletion {
		return "deletion"
	}
	return "epenthesis"
}

// violation finds the first syllable-structure violation of w under p,
// syllabifying permissively. It returns the kind ('o' onset too long,
// 'c' coda too long, 'v' no vowel) and the syllable index, or 0.
func violation(w *Word, p *Phonotactics) (byte, int) {
	hasV := false
	for _, s := range w.Segs {
		if s.IsVowel() {
			hasV = true
			break
		}
	}
	if !hasV && len(w.Segs) > 0 {
		return 'v', 0
	}
	Resyllabify(w, &DefaultPhonotactics)
	for k, s := range w.Syll {
		if s.Nucleus-s.Start > p.Template.MaxOnset {
			return 'o', k
		}
		if s.End-s.Nucleus-1 > p.Template.MaxCoda {
			return 'c', k
		}
		for i := s.Start + 1; i < s.Nucleus; i++ {
			if p.banned(w.Segs[i-1], w.Segs[i]) {
				return 'o', k
			}
		}
	}
	return 0, 0
}

// Legal reports whether w satisfies p (it resyllabifies w).
func Legal(w *Word, p *Phonotactics) bool {
	kind, _ := violation(w, p)
	Resyllabify(w, p)
	return kind == 0
}

func insertSeg(w *Word, at int, s Segment) {
	w.Segs = append(w.Segs, Segment{})
	copy(w.Segs[at+1:], w.Segs[at:])
	w.Segs[at] = s
	for i := range w.Bounds {
		if w.Bounds[i].At > at {
			w.Bounds[i].At++
		}
	}
}

func deleteSeg(w *Word, at int) {
	w.Segs = append(w.Segs[:at], w.Segs[at+1:]...)
	out := w.Bounds[:0]
	for _, b := range w.Bounds {
		if b.At > at {
			b.At--
		}
		if b.At > 0 && b.At < len(w.Segs) && (len(out) == 0 || out[len(out)-1] != b) {
			out = append(out, b)
		}
	}
	w.Bounds = out
}

// Repair fixes w until it satisfies p, by epenthesis of ep or by deletion.
// It reports whether w changed. Syllables are current afterwards.
func Repair(w *Word, p *Phonotactics, mode RepairMode, ep Segment) bool {
	changed := false
	for iter := 0; iter < max(8, len(w.Segs)); iter++ {
		kind, k := violation(w, p)
		if kind == 0 {
			break
		}
		changed = true
		if kind == 'v' {
			insertSeg(w, len(w.Segs)-1, ep)
			continue
		}
		s := w.Syll[k]
		switch {
		case kind == 'o' && mode == RepairEpenthesis:
			insertSeg(w, s.Start+1, ep)
		case kind == 'o':
			deleteSeg(w, s.Start)
		case mode == RepairEpenthesis && p.Template.MaxCoda == 0:
			insertSeg(w, s.End, ep) // no codas at all: the vowel follows (kat → kati)
		case mode == RepairEpenthesis:
			insertSeg(w, s.End-1, ep)
		default:
			deleteSeg(w, s.Nucleus+1)
		}
	}
	Resyllabify(w, p)
	return changed
}
