package world

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	w := Generate(42, DefaultConfig)
	if n := len(w.Land); n < 280 || n > 320 {
		t.Errorf("land hexes = %d", n)
	}
	counts := map[Biome]int{}
	rivers := 0
	for _, h := range w.Hexes {
		counts[h.Biome]++
		if h.River {
			rivers++
		}
	}
	if counts[Mountain] == 0 || counts[Coast] == 0 || rivers == 0 {
		t.Errorf("biomes %v rivers %d", counts, rivers)
	}
	// Neighbour symmetry.
	for _, h := range w.Hexes {
		for _, nb := range h.Nbr {
			if nb < 0 {
				continue
			}
			found := false
			for _, back := range w.Hexes[nb].Nbr {
				if back == h.ID {
					found = true
				}
			}
			if !found {
				t.Fatalf("neighbours of %d and %d not symmetric", h.ID, nb)
			}
			if w.Distance(h.ID, nb) != 1 {
				t.Fatalf("distance %d-%d = %d", h.ID, nb, w.Distance(h.ID, nb))
			}
		}
	}
	if testing.Verbose() {
		t.Log("\n" + ascii(w))
	}
}

func TestDeterministic(t *testing.T) {
	a, b := Generate(7, DefaultConfig), Generate(7, DefaultConfig)
	for i := range a.Hexes {
		if a.Hexes[i] != b.Hexes[i] {
			t.Fatal("not deterministic")
		}
	}
	c := Generate(8, DefaultConfig)
	same := 0
	for i := range a.Hexes {
		if a.Hexes[i].Biome == c.Hexes[i].Biome {
			same++
		}
	}
	if same == len(a.Hexes) {
		t.Error("different seeds give identical maps")
	}
}

func ascii(w *World) string {
	sym := map[Biome]byte{Sea: '.', Coast: 'c', Plains: 'p', Forest: 'f', Steppe: 's', Desert: 'd', Mountain: 'M', Island: 'i'}
	var b strings.Builder
	for r := int32(0); r < w.H; r++ {
		if r&1 == 1 {
			b.WriteByte(' ')
		}
		for c := int32(0); c < w.W; c++ {
			h := w.Hexes[r*w.W+c]
			ch := sym[h.Biome]
			if h.River {
				ch = '~'
			}
			b.WriteByte(ch)
			b.WriteByte(' ')
		}
		b.WriteByte('\n')
	}
	return b.String()
}
