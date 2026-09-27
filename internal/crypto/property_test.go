package crypto

import (
	"math/big"
	"testing"
)

// refMult is an independent affine reference scalar multiplication used to
// cross-check the Jacobian implementation.
func refMult(pt *Point, k *big.Int) *Point {
	res := &Point{} // infinity
	base := pt.clone()
	kk := new(big.Int).Set(k)
	for i := 0; i < kk.BitLen(); i++ {
		if kk.Bit(i) == 1 {
			res = res.Add(base)
		}
		base = base.Double()
	}
	return res
}

// TestScalarMultAgreement cross-checks Jacobian scalar multiplication against
// the independent affine reference, for both G and random points.
func TestScalarMultAgreement(t *testing.T) {
	for i := 0; i < 60; i++ {
		k, _ := GeneratePrivateKey()
		if got, want := ScalarBaseMult(k), refMult(G(), k); !got.equal(want) {
			t.Fatalf("base mult mismatch i=%d\n got %x,%x\nwant %x,%x", i, got.X, got.Y, want.X, want.Y)
		}
	}
	for i := 0; i < 40; i++ {
		k, _ := GeneratePrivateKey()
		q, _ := GeneratePrivateKey()
		base := ScalarBaseMult(q)
		if got, want := base.ScalarMult(k), refMult(base, k); !got.equal(want) {
			t.Fatalf("generic mult mismatch i=%d\n got %x,%x\nwant %x,%x", i, got.X, got.Y, want.X, want.Y)
		}
	}
}

// TestAddAgreement cross-checks Jacobian addition against affine addition.
func TestAddAgreement(t *testing.T) {
	for i := 0; i < 60; i++ {
		a, _ := GeneratePrivateKey()
		b, _ := GeneratePrivateKey()
		Pa, Pb := ScalarBaseMult(a), ScalarBaseMult(b)
		got := newJacobian(Pa).add(newJacobian(Pb)).toAffine()
		want := Pa.Add(Pb)
		if !got.equal(want) {
			t.Fatalf("add mismatch i=%d\n got %x,%x\nwant %x,%x", i, got.X, got.Y, want.X, want.Y)
		}
	}
}

func (pt *Point) equal(q *Point) bool {
	if pt.IsInfinity() || q.IsInfinity() {
		return pt.IsInfinity() && q.IsInfinity()
	}
	return pt.X.Cmp(q.X) == 0 && pt.Y.Cmp(q.Y) == 0
}
