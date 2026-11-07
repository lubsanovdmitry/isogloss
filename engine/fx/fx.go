// Package fx provides fixed-point arithmetic for every value that feeds a
// simulation decision. Floats never enter simulation state (see engine/README.md).
package fx

// Q16 is an unsigned fixed-point number with 16 fractional bits: One == 1.0.
type Q16 uint32

const (
	Zero Q16 = 0
	One  Q16 = 1 << 16
	Half Q16 = 1 << 15
)

// FromRatio returns num/den as Q16, rounded down. den must be non-zero.
func FromRatio(num, den uint32) Q16 {
	return Q16((uint64(num) << 16) / uint64(den))
}

// FromPermille converts a per-mille value (0..1000) to Q16.
func FromPermille(pm uint32) Q16 { return FromRatio(pm, 1000) }

// Mul returns a*b, rounded down.
func Mul(a, b Q16) Q16 { return Q16((uint64(a) * uint64(b)) >> 16) }

// Div returns a/b, rounded down. b must be non-zero.
func Div(a, b Q16) Q16 { return Q16((uint64(a) << 16) / uint64(b)) }

// Add returns a+b, saturating at the uint32 maximum.
func Add(a, b Q16) Q16 {
	s := uint64(a) + uint64(b)
	if s > 0xFFFFFFFF {
		return 0xFFFFFFFF
	}
	return Q16(s)
}

// Sub returns a-b, clamped at zero.
func Sub(a, b Q16) Q16 {
	if b >= a {
		return 0
	}
	return a - b
}

// Clamp01 clamps a to [0, One].
func Clamp01(a Q16) Q16 {
	if a > One {
		return One
	}
	return a
}

// Logistic returns one logistic growth step: a + r*a*(1-a), clamped to [0,1].
func Logistic(a, r Q16) Q16 {
	a = Clamp01(a)
	return Clamp01(Add(a, Mul(r, Mul(a, One-a))))
}

// Permille returns a rounded to the nearest per-mille (for display and tests).
func (a Q16) Permille() uint32 { return uint32((uint64(a)*1000 + uint64(Half)) >> 16) }
