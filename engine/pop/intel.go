// Package pop holds speech communities, contact weights, wave spread,
// intelligibility and clustering of varieties into languages.
package pop

import (
	"isogloss/engine/fx"
	"isogloss/engine/lex"
	"isogloss/engine/phon"
)

const (
	unit     = 10 // cost of one full mismatch or indel
	maxSubst = 20 // phon.Distance at which substitution is a full mismatch
)

func substCost(a, b phon.Segment) int {
	if a == b {
		return 0
	}
	if a.IsVowel() != b.IsVowel() {
		return unit
	}
	d := phon.Distance(a.F, b.F)
	c := min(d, maxSubst) * unit / maxSubst
	if a.Tone != b.Tone {
		c += 2
	}
	return min(c, unit)
}

// WordSim is the feature-weighted alignment similarity of two segment
// strings in [0, One]: p~b costs less than p~a. Needleman–Wunsch with
// linear gap cost, normalised by the longer length.
func WordSim(a, b []phon.Segment) fx.Q16 {
	n, m := len(a), len(b)
	if n == 0 && m == 0 {
		return fx.One
	}
	if n == 0 || m == 0 {
		return 0
	}
	var buf [2][32]int
	prev, cur := buf[0][:], buf[1][:]
	if m+1 > len(prev) {
		prev, cur = make([]int, m+1), make([]int, m+1)
	}
	for j := 0; j <= m; j++ {
		prev[j] = j * unit
	}
	for i := 1; i <= n; i++ {
		cur[0] = i * unit
		for j := 1; j <= m; j++ {
			best := prev[j-1] + substCost(a[i-1], b[j-1])
			if d := prev[j] + unit; d < best {
				best = d
			}
			if d := cur[j-1] + unit; d < best {
				best = d
			}
			cur[j] = best
		}
		prev, cur = cur, prev
	}
	cost := prev[m]
	maxCost := max(n, m) * unit
	return fx.Q16(uint64(maxCost-cost) * uint64(fx.One) / uint64(maxCost))
}

// Metric holds intelligibility parameters.
type Metric struct {
	// Base is the lexical similarity of unrelated word lists; similarity
	// at or below Base counts as zero intelligibility.
	Base fx.Q16
	// LexWeight, InvWeight, MorphWeight and OrderWeight (per-mille, summing
	// to 1000) mix the core vocabulary, phoneme inventories, inflected forms
	// and word order.
	LexWeight, InvWeight, MorphWeight, OrderWeight uint32
}

// DefaultMetric: the lexical scale is calibrated by TestRomanceCalibration;
// inflection and word order are smaller terms (speakers mostly hear words).
var DefaultMetric = Metric{Base: fx.FromPermille(390), LexWeight: 750, InvWeight: 100, MorphWeight: 100, OrderWeight: 50}

// LexicalSim averages WordSim over the core (Swadesh) concepts.
func LexicalSim(a, b *lex.Variety) fx.Q16 {
	var sum uint64
	n := 0
	for i := 0; i < a.Lex.Len(); i++ {
		if !lex.Concepts()[i].Core() {
			continue
		}
		n++
		la, lb := a.Lex.At(lex.ConceptID(i)), b.Lex.At(lex.ConceptID(i))
		if la.Hash == lb.Hash {
			sum += uint64(fx.One)
			continue
		}
		sum += uint64(WordSim(la.Form.Segs, lb.Form.Segs))
	}
	return fx.Q16(sum / uint64(n))
}

// InventorySim is the Jaccard overlap of two phoneme inventories.
func InventorySim(a, b []phon.FeatureSet) fx.Q16 {
	i, j, both := 0, 0, 0
	less := func(x, y phon.FeatureSet) bool { return x.Val < y.Val || (x.Val == y.Val && x.Mask < y.Mask) }
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			both++
			i++
			j++
		case less(a[i], b[j]):
			i++
		default:
			j++
		}
	}
	union := len(a) + len(b) - both
	if union == 0 {
		return fx.One
	}
	return fx.FromRatio(uint32(both), uint32(union))
}

// Rescale maps raw lexical similarity to intelligibility given the baseline.
func (m Metric) Rescale(l fx.Q16) fx.Q16 {
	if l <= m.Base {
		return 0
	}
	return fx.Div(l-m.Base, fx.One-m.Base)
}

// MorphSim averages WordSim over a sample of inflected forms (every third
// paradigm cell of every tracked lexeme). Varieties without paradigms count
// as identical here.
func MorphSim(a, b *lex.Variety) fx.Q16 {
	var sum uint64
	n := 0
	for i := range a.M.Para {
		pa, pb := a.M.Para[i], b.M.Para[i]
		if pa == nil || pb == nil {
			continue
		}
		if pa == pb {
			k := (len(pa.Forms) + 1) / 3
			sum += uint64(k) * uint64(fx.One)
			n += k
			continue
		}
		for k := 1; k < len(pa.Forms); k += 3 {
			n++
			if pa.Hash[k] == pb.Hash[k] {
				sum += uint64(fx.One)
				continue
			}
			sum += uint64(WordSim(pa.Forms[k].Segs, pb.Forms[k].Segs))
		}
	}
	if n == 0 {
		return fx.One
	}
	return fx.Q16(sum / uint64(n))
}

// OrderSim is the share of word-order parameters two varieties agree on.
func OrderSim(a, b *lex.Variety) fx.Q16 {
	same := 0
	for p := range a.M.Order {
		if a.M.Order[p] == b.M.Order[p] {
			same++
		}
	}
	return fx.FromRatio(uint32(same), uint32(len(a.M.Order)))
}

// Intel is mutual intelligibility of two varieties in [0, One].
func (m Metric) Intel(a, b *lex.Variety) fx.Q16 {
	if a == b || a.Key == b.Key {
		return fx.One
	}
	l := m.Rescale(LexicalSim(a, b))
	inv := InventorySim(a.Inventory(), b.Inventory())
	sum := uint64(l)*uint64(m.LexWeight) + uint64(inv)*uint64(m.InvWeight)
	if m.MorphWeight > 0 {
		sum += uint64(m.Rescale(MorphSim(a, b))) * uint64(m.MorphWeight)
	}
	if m.OrderWeight > 0 {
		sum += uint64(OrderSim(a, b)) * uint64(m.OrderWeight)
	}
	return fx.Q16(sum / 1000)
}

// Align aligns b to a with the same costs as WordSim and returns, for each
// segment of a, the index of its partner in b or -1 if it was deleted.
// Insertions in b are not reported. Ties prefer substitution, then
// deletion, so the result is deterministic.
func Align(a, b []phon.Segment) []int {
	n, m := len(a), len(b)
	d := make([]int, (n+1)*(m+1))
	at := func(i, j int) *int { return &d[i*(m+1)+j] }
	for i := 0; i <= n; i++ {
		*at(i, 0) = i * unit
	}
	for j := 0; j <= m; j++ {
		*at(0, j) = j * unit
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			best := *at(i-1, j-1) + substCost(a[i-1], b[j-1])
			best = min(best, *at(i-1, j)+unit, *at(i, j-1)+unit)
			*at(i, j) = best
		}
	}
	out := make([]int, n)
	i, j := n, m
	for i > 0 {
		switch {
		case j > 0 && *at(i, j) == *at(i-1, j-1)+substCost(a[i-1], b[j-1]):
			out[i-1] = j - 1
			i, j = i-1, j-1
		case *at(i, j) == *at(i-1, j)+unit:
			out[i-1] = -1
			i--
		default:
			j--
		}
	}
	return out
}
