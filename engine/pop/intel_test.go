package pop

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"isogloss/engine/data"
	"isogloss/engine/fx"
	"isogloss/engine/lex"
	"isogloss/engine/phon"
	"isogloss/engine/rng"
)

func TestWordSim(t *testing.T) {
	w := func(s string) []phon.Segment { return phon.MustParseWord(s).Segs }
	if WordSim(w("pata"), w("pata")) != fx.One {
		t.Error("identical")
	}
	pb := WordSim(w("pata"), w("bata"))
	pa := WordSim(w("pata"), w("aata"))
	if pb <= pa {
		t.Errorf("p~b (%d) should be closer than p~a (%d)", pb.Permille(), pa.Permille())
	}
}

// romance loads IE-CoR forms: concepticon → lang → segments.
func romance(t *testing.T) map[int]map[string][]phon.Segment {
	out := map[int]map[string][]phon.Segment{}
	bad := 0
	for _, f := range data.Lines(data.Romance) {
		cid, _ := strconv.Atoi(f[0])
		w, err := phon.ParseWord(strings.ReplaceAll(f[2], " ", ""))
		if err != nil {
			bad++
			continue
		}
		if out[cid] == nil {
			out[cid] = map[string][]phon.Segment{}
		}
		out[cid][f[1]] = w.Segs
	}
	if bad > 60 {
		t.Errorf("%d Romance forms failed to parse", bad)
	}
	return out
}

func pairSim(forms map[int]map[string][]phon.Segment, a, b string) (fx.Q16, int) {
	var sum uint64
	n := 0
	for _, langs := range forms { //lint:ordered summing is order-independent
		fa, okA := langs[a]
		fb, okB := langs[b]
		if okA && okB {
			sum += uint64(WordSim(fa, fb))
			n++
		}
	}
	return fx.Q16(sum / uint64(max(n, 1))), n
}

// TestRomanceCalibration checks the metric against Romance pairs with known
// intelligibility: Spanish–Portuguese high, Spanish–Italian lower,
// Spanish–Romanian and Spanish–French lower still. The clustering threshold
// must group es–pt and separate es–ro, as accepted classifications do.
func TestRomanceCalibration(t *testing.T) {
	forms := romance(t)
	m := DefaultMetric
	score := map[string]fx.Q16{}
	inv := inventories(forms)
	for _, p := range [][2]string{{"spa", "por"}, {"spa", "ita"}, {"spa", "ron"}, {"spa", "fra"}, {"ita", "ron"}, {"por", "fra"}} {
		raw, n := pairSim(forms, p[0], p[1])
		is := InventorySim(inv[p[0]], inv[p[1]])
		intel := fx.Q16((uint64(m.Rescale(raw))*uint64(m.LexWeight) + uint64(is)*uint64(m.InvWeight)) / 1000)
		score[p[0]+"-"+p[1]] = intel
		t.Logf("%s–%s: raw %d‰, inventory %d‰, intel %d‰ over %d concepts", p[0], p[1], raw.Permille(), is.Permille(), intel.Permille(), n)
	}
	// Random baseline: shuffled pairing of Spanish and Portuguese words.
	var sum uint64
	var es, pt [][]phon.Segment
	for _, langs := range forms { //lint:ordered collected then shuffled deterministically below
		if a, ok := langs["spa"]; ok {
			if b, ok := langs["por"]; ok {
				es, pt = append(es, a), append(pt, b)
			}
		}
	}
	r := rng.New(1)
	for i := range es {
		sum += uint64(WordSim(es[i], pt[r.IntN(len(pt))]))
	}
	base := fx.Q16(sum / uint64(len(es)))
	t.Logf("unrelated baseline: %d‰ (metric base %d‰)", base.Permille(), m.Base.Permille())

	if !(score["spa-por"] > score["spa-ita"] && score["spa-ita"] > score["spa-ron"]) {
		t.Errorf("ordering wrong: es-pt %d, es-it %d, es-ro %d", score["spa-por"].Permille(), score["spa-ita"].Permille(), score["spa-ron"].Permille())
	}
	thr := DefaultThreshold
	if score["spa-por"] < thr || score["spa-ita"] >= thr || score["spa-ron"] >= thr || score["spa-fra"] >= thr {
		t.Errorf("threshold %d‰ should group es-pt (%d‰) and separate es-it (%d‰), es-ro (%d‰), es-fr (%d‰)",
			thr.Permille(), score["spa-por"].Permille(), score["spa-ita"].Permille(), score["spa-ron"].Permille(), score["spa-fra"].Permille())
	}
	d := base.Permille()
	if d+60 < m.Base.Permille() || d > m.Base.Permille()+60 {
		t.Errorf("metric base %d‰ is far from measured unrelated baseline %d‰", m.Base.Permille(), d)
	}
}

func inventories(forms map[int]map[string][]phon.Segment) map[string][]phon.FeatureSet {
	sets := map[string]map[phon.FeatureSet]bool{}
	for _, langs := range forms { //lint:ordered building sets
		for l, segs := range langs { //lint:ordered building sets
			if sets[l] == nil {
				sets[l] = map[phon.FeatureSet]bool{}
			}
			for _, s := range segs {
				sets[l][s.F] = true
			}
		}
	}
	out := map[string][]phon.FeatureSet{}
	for l, set := range sets { //lint:ordered each list is sorted below
		var fs []phon.FeatureSet
		for f := range set { //lint:ordered sorted below
			fs = append(fs, f)
		}
		slices.SortFunc(fs, func(a, b phon.FeatureSet) int {
			if a.Val != b.Val {
				return cmp.Compare(a.Val, b.Val)
			}
			return cmp.Compare(a.Mask, b.Mask)
		})
		out[l] = fs
	}
	return out
}

func TestIntelSelf(t *testing.T) {
	p := lex.GenerateProto(rng.New(3, rng.Key("proto"))).Variety
	if DefaultMetric.Intel(p, p) != fx.One {
		t.Error("self intelligibility")
	}
}
