package morph

import (
	"strings"
	"sync"

	"isogloss/engine/data"
	"isogloss/engine/rng"
)

// Param is a word-order parameter.
type Param uint8

const (
	Clause Param = iota // SOV, SVO, VSO, VOS, OVS, OSV
	Adp                 // postpositions, prepositions
	GenN                // genitive–noun, noun–genitive
	AdjN                // adjective–noun, noun–adjective
	DemN                // demonstrative–noun, noun–demonstrative
	RelN                // noun–relative clause, relative clause–noun
	NumParams
)

type paramInfo struct {
	Name   string
	Feat   string   // WALS feature
	Values []string // display names, in WALS value order (value i = code i+1)
}

var params = [NumParams]paramInfo{
	Clause: {"clause order", "81A", []string{"SOV", "SVO", "VSO", "VOS", "OVS", "OSV"}},
	Adp:    {"adpositions", "85A", []string{"postpositions", "prepositions"}},
	GenN:   {"genitive", "86A", []string{"genitive–noun", "noun–genitive"}},
	AdjN:   {"adjective", "87A", []string{"adjective–noun", "noun–adjective"}},
	DemN:   {"demonstrative", "88A", []string{"demonstrative–noun", "noun–demonstrative"}},
	RelN:   {"relative clause", "90A", []string{"noun–relative clause", "relative clause–noun"}},
}

// Name returns the parameter's name.
func (p Param) Name() string { return params[p].Name }

// Values returns the parameter's value names.
func (p Param) Values() []string { return params[p].Values }

// Order is a variety's word-order parameter values (indices into Values).
type Order [NumParams]uint8

// ValueName names the value of p.
func (o Order) ValueName(p Param) string { return params[p].Values[o[p]] }

// OV reports object-before-verb clause order.
func (o Order) OV() bool { c := o[Clause]; return c == 0 || c == 4 || c == 5 }

// SV reports subject-before-verb clause order.
func (o Order) SV() bool { c := o[Clause]; return c == 0 || c == 1 || c == 5 }

// Postpositional reports postpositions.
func (o Order) Postpositional() bool { return o[Adp] == 0 }

// walsTable holds WALS value counts and pairwise joint counts.
type walsTable struct {
	count [NumParams][]uint32
	// joint[p][q][a][b]: languages with value a of p and b of q.
	joint [NumParams][NumParams][][]uint32
	// lift[p][q][a][b] (per-mille): how much more often a and b co-occur
	// than if independent, from the joint table.
	lift [NumParams][NumParams][][]uint32
}

var (
	walsOnce sync.Once
	wals     walsTable
)

func paramOf(code string) (Param, int, bool) {
	feat, val, ok := strings.Cut(code, "-")
	if !ok {
		return 0, 0, false
	}
	for p := Param(0); p < NumParams; p++ {
		if params[p].Feat == feat {
			v := 0
			for _, c := range val {
				v = v*10 + int(c-'0')
			}
			if v >= 1 && v <= len(params[p].Values) {
				return p, v - 1, true
			}
			return 0, 0, false
		}
	}
	return 0, 0, false
}

func atou(s string) uint32 {
	n := uint32(0)
	for _, c := range s {
		n = n*10 + uint32(c-'0')
	}
	return n
}

func loadWALS() {
	t := &wals
	for p := Param(0); p < NumParams; p++ {
		n := len(params[p].Values)
		t.count[p] = make([]uint32, n)
		for q := Param(0); q < NumParams; q++ {
			m := len(params[q].Values)
			t.joint[p][q] = make([][]uint32, n)
			t.lift[p][q] = make([][]uint32, n)
			for a := range n {
				t.joint[p][q][a] = make([]uint32, m)
				t.lift[p][q][a] = make([]uint32, m)
			}
		}
	}
	for _, f := range data.Lines(data.WALS) {
		switch f[0] {
		case "V":
			if p, v, ok := paramOf(f[1]); ok {
				t.count[p][v] = atou(f[3])
			}
		case "J":
			p, a, ok1 := paramOf(f[1])
			q, b, ok2 := paramOf(f[2])
			if ok1 && ok2 && p != q {
				c := atou(f[3])
				t.joint[p][q][a][b] = c
				t.joint[q][p][b][a] = c
			}
		}
	}
	// lift = J(a,b) · N / (row(a) · col(b)), with add-one smoothing.
	for p := Param(0); p < NumParams; p++ {
		for q := Param(0); q < NumParams; q++ {
			if p == q {
				continue
			}
			j := t.joint[p][q]
			var total uint64
			row := make([]uint64, len(j))
			col := make([]uint64, len(params[q].Values))
			for a := range j {
				for b := range j[a] {
					x := uint64(j[a][b]) + 1
					row[a] += x
					col[b] += x
					total += x
				}
			}
			for a := range j {
				for b := range j[a] {
					x := uint64(j[a][b]) + 1
					t.lift[p][q][a][b] = uint32(x * total * 1000 / (row[a] * col[b]))
				}
			}
		}
	}
}

func table() *walsTable {
	walsOnce.Do(loadWALS)
	return &wals
}

// Harmony is the weight of value v for parameter p given the variety's
// other parameters: the WALS frequency of v times its mean lift (per-mille)
// with the other values. The mean, not the product, keeps disharmonic
// changes rare but possible: languages do drift out of harmony (English
// went from OV to VO while keeping genitive–noun order).
func Harmony(o Order, p Param, v uint8) uint64 {
	t := table()
	var sum uint64
	for q := Param(0); q < NumParams; q++ {
		if q != p {
			sum += uint64(max(t.lift[p][q][v][o[q]], 50))
		}
	}
	return (uint64(t.count[p][v]) + 1) * sum / uint64(NumParams-1) / 1000
}

// SampleOrder draws word-order parameters from the WALS priors: clause
// order first, then each further parameter given the ones already chosen.
func SampleOrder(r *rng.Stream) Order {
	t := table()
	var o Order
	w := make([]uint32, len(t.count[Clause]))
	for v, c := range t.count[Clause] {
		w[v] = c
	}
	o[Clause] = uint8(r.Weighted(w))
	for p := Adp; p < NumParams; p++ {
		ws := make([]uint32, len(params[p].Values))
		for v := range ws {
			x := uint64(t.count[p][v]) + 1
			for q := Clause; q < p; q++ {
				x = x * uint64(max(t.lift[p][q][v][o[q]], 50)) / 1000
			}
			ws[v] = uint32(min(x+1, 1<<31))
		}
		o[p] = uint8(r.Weighted(ws))
	}
	return o
}
