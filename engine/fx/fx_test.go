package fx

import "testing"

func TestBasics(t *testing.T) {
	if Mul(Half, Half) != One/4 {
		t.Fatal("mul")
	}
	if Div(One/4, Half) != Half {
		t.Fatal("div")
	}
	if FromPermille(500) != Half {
		t.Fatal("permille")
	}
	if Sub(Half, One) != 0 || Add(0xFFFFFFF0, One) != 0xFFFFFFFF {
		t.Fatal("saturation")
	}
}

func TestLogisticMonotoneAndBounded(t *testing.T) {
	a := FromPermille(10)
	r := FromPermille(800)
	for i := 0; i < 200; i++ {
		b := Logistic(a, r)
		if b < a || b > One {
			t.Fatalf("step %d: %d -> %d", i, a, b)
		}
		a = b
	}
	if a.Permille() < 990 {
		t.Fatalf("did not saturate: %d", a.Permille())
	}
}
