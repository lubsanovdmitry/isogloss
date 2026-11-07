// Package rng provides keyed, deterministic random streams. Each subsystem and
// entity draws from its own stream, so adding a subsystem never reshuffles
// the others.
package rng

import "math/rand/v2"

// Stream is a deterministic PCG stream.
type Stream struct{ r *rand.Rand }

func splitmix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// Key hashes a string into a stream key (FNV-1a, 64 bit).
func Key(s string) uint64 {
	h := uint64(0xcbf29ce484222325)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 0x100000001b3
	}
	return h
}

// New derives a stream from the run seed and a key path, e.g.
// New(seed, Key("pop"), uint64(communityID)).
func New(seed uint64, keys ...uint64) *Stream {
	a := splitmix(seed)
	b := splitmix(a ^ 0x5851f42d4c957f2d)
	for _, k := range keys {
		a = splitmix(a ^ k)
		b = splitmix(b ^ a)
	}
	return &Stream{r: rand.New(rand.NewPCG(a, b))}
}

// Uint64 returns a uniformly random uint64.
func (s *Stream) Uint64() uint64 { return s.r.Uint64() }

// IntN returns a uniform int in [0, n). n must be > 0.
func (s *Stream) IntN(n int) int { return s.r.IntN(n) }

// Permille returns true with probability pm/1000.
func (s *Stream) Permille(pm uint32) bool { return uint32(s.r.IntN(1000)) < pm }

// Weighted picks an index with probability proportional to w[i]. It returns
// -1 if all weights are zero.
func (s *Stream) Weighted(w []uint32) int {
	var total uint64
	for _, x := range w {
		total += uint64(x)
	}
	if total == 0 {
		return -1
	}
	r := s.r.Uint64N(total)
	for i, x := range w {
		if r < uint64(x) {
			return i
		}
		r -= uint64(x)
	}
	return len(w) - 1
}
