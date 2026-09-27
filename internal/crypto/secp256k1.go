// Package crypto implements the secp256k1 elliptic curve and the BIP-340
// Schnorr signature scheme used by Nostr, using only the Go standard library.
//
// This is an independent, clean-room implementation written against the
// published curve parameters and the BIP-340 specification. It performs no
// network or filesystem access.
package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"math/big"
)

// secp256k1 domain parameters.
var (
	// p = 2^256 - 2^32 - 977
	p, _  = new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F", 16)
	n, _  = new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
	gx, _ = new(big.Int).SetString("79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798", 16)
	gy, _ = new(big.Int).SetString("483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8", 16)
	big0  = big.NewInt(0)
	big1  = big.NewInt(1)
	big2  = big.NewInt(2)
	big3  = big.NewInt(3)
)

// Point is an affine point on secp256k1. A nil X denotes the point at infinity.
type Point struct{ X, Y *big.Int }

// G returns the generator point.
func G() *Point { return &Point{X: new(big.Int).Set(gx), Y: new(big.Int).Set(gy)} }

// N returns the curve order.
func N() *big.Int { return new(big.Int).Set(n) }

// P returns the field prime.
func P() *big.Int { return new(big.Int).Set(p) }

func mod(x *big.Int) *big.Int { return new(big.Int).Mod(x, p) }

// IsInfinity reports whether the point is the point at infinity.
func (pt *Point) IsInfinity() bool { return pt == nil || pt.X == nil }

// Add returns the sum of two affine points.
func (pt *Point) Add(q *Point) *Point {
	if pt.IsInfinity() {
		return q.clone()
	}
	if q.IsInfinity() {
		return pt.clone()
	}
	if pt.X.Cmp(q.X) == 0 {
		if mod(new(big.Int).Add(pt.Y, q.Y)).Sign() == 0 {
			return &Point{} // P + (-P) = O
		}
		return pt.Double()
	}
	// lambda = (qy - py) / (qx - px)
	num := mod(new(big.Int).Sub(q.Y, pt.Y))
	den := mod(new(big.Int).Sub(q.X, pt.X))
	lam := new(big.Int).Mul(num, den.ModInverse(den, p))
	lam.Mod(lam, p)
	x3 := mod(new(big.Int).Sub(new(big.Int).Sub(new(big.Int).Mul(lam, lam), pt.X), q.X))
	y3 := mod(new(big.Int).Sub(new(big.Int).Mul(lam, new(big.Int).Sub(pt.X, x3)), pt.Y))
	return &Point{X: x3, Y: y3}
}

// Double returns 2*pt.
func (pt *Point) Double() *Point {
	if pt.IsInfinity() || pt.Y.Sign() == 0 {
		return &Point{}
	}
	// lambda = 3x^2 / 2y
	x2 := new(big.Int).Mul(pt.X, pt.X)
	num := mod(new(big.Int).Mul(big3, x2))
	den := mod(new(big.Int).Mul(big2, pt.Y))
	lam := new(big.Int).Mul(num, den.ModInverse(den, p))
	lam.Mod(lam, p)
	x3 := mod(new(big.Int).Sub(new(big.Int).Mul(lam, lam), new(big.Int).Mul(big2, pt.X)))
	y3 := mod(new(big.Int).Sub(new(big.Int).Mul(lam, new(big.Int).Sub(pt.X, x3)), pt.Y))
	return &Point{X: x3, Y: y3}
}

// ScalarMult returns k*pt using Jacobian coordinates (fast, one field inversion).
func (pt *Point) ScalarMult(k *big.Int) *Point {
	if pt.IsInfinity() || k.Sign() == 0 {
		return &Point{}
	}
	kk := new(big.Int).Mod(k, n)
	j := newJacobian(pt)
	res := &jacobianPoint{z: new(big.Int).Set(big0)} // infinity
	for i := 0; i < kk.BitLen(); i++ {
		if kk.Bit(i) == 1 {
			res = res.add(j)
		}
		j = j.double()
	}
	return res.toAffine()
}

// ScalarBaseMult returns k*G.
func ScalarBaseMult(k *big.Int) *Point { return G().ScalarMult(k) }

func (pt *Point) clone() *Point {
	if pt.IsInfinity() {
		return &Point{}
	}
	return &Point{X: new(big.Int).Set(pt.X), Y: new(big.Int).Set(pt.Y)}
}

// Neg returns -pt.
func (pt *Point) Neg() *Point {
	if pt.IsInfinity() {
		return &Point{}
	}
	return &Point{X: new(big.Int).Set(pt.X), Y: mod(new(big.Int).Neg(pt.Y))}
}

// IsOnCurve reports whether pt satisfies y^2 = x^3 + 7.
func (pt *Point) IsOnCurve() bool {
	if pt.IsInfinity() {
		return true
	}
	lhs := mod(new(big.Int).Mul(pt.Y, pt.Y))
	rhs := mod(new(big.Int).Add(new(big.Int).Mul(pt.X, new(big.Int).Mul(pt.X, pt.X)), big.NewInt(7)))
	return lhs.Cmp(rhs) == 0
}

// jacobianPoint is a point in Jacobian coordinates (X/Z^2, Y/Z^3).
type jacobianPoint struct{ x, y, z *big.Int }

func newJacobian(pt *Point) *jacobianPoint {
	if pt.IsInfinity() {
		return &jacobianPoint{x: new(big.Int), y: new(big.Int), z: new(big.Int).Set(big0)}
	}
	return &jacobianPoint{x: new(big.Int).Set(pt.X), y: new(big.Int).Set(pt.Y), z: new(big.Int).Set(big1)}
}

func (j *jacobianPoint) isInfinity() bool { return j.z.Sign() == 0 }

func (j *jacobianPoint) toAffine() *Point {
	if j.isInfinity() {
		return &Point{}
	}
	zi := new(big.Int).ModInverse(j.z, p)
	z2i := mod(new(big.Int).Mul(zi, zi))
	z3i := mod(new(big.Int).Mul(z2i, zi))
	x := mod(new(big.Int).Mul(j.x, z2i))
	y := mod(new(big.Int).Mul(j.y, z3i))
	return &Point{X: x, Y: y}
}

// double computes 2*j using standard secp256k1 Jacobian doubling (a=0).
func (j *jacobianPoint) double() *jacobianPoint {
	if j.isInfinity() || j.y.Sign() == 0 {
		return &jacobianPoint{x: new(big.Int), y: new(big.Int), z: new(big.Int).Set(big0)}
	}
	y2 := mod(new(big.Int).Mul(j.y, j.y))
	y4 := mod(new(big.Int).Mul(y2, y2))
	s := mod(new(big.Int).Mul(big.NewInt(4), new(big.Int).Mul(j.x, y2)))
	m := mod(new(big.Int).Mul(big3, new(big.Int).Mul(j.x, j.x))) // a=0
	x3 := mod(new(big.Int).Sub(new(big.Int).Mul(m, m), new(big.Int).Mul(big2, s)))
	y3 := mod(new(big.Int).Sub(new(big.Int).Mul(m, new(big.Int).Sub(s, x3)), new(big.Int).Mul(big.NewInt(8), y4)))
	z3 := mod(new(big.Int).Mul(big2, new(big.Int).Mul(j.y, j.z)))
	return &jacobianPoint{x: x3, y: y3, z: z3}
}

// add computes j + q in Jacobian coordinates.
func (j *jacobianPoint) add(q *jacobianPoint) *jacobianPoint {
	if j.isInfinity() {
		return &jacobianPoint{x: new(big.Int).Set(q.x), y: new(big.Int).Set(q.y), z: new(big.Int).Set(q.z)}
	}
	if q.isInfinity() {
		return &jacobianPoint{x: new(big.Int).Set(j.x), y: new(big.Int).Set(j.y), z: new(big.Int).Set(j.z)}
	}
	z1z1 := mod(new(big.Int).Mul(j.z, j.z))
	z2z2 := mod(new(big.Int).Mul(q.z, q.z))
	u1 := mod(new(big.Int).Mul(j.x, z2z2))
	u2 := mod(new(big.Int).Mul(q.x, z1z1))
	s1 := mod(new(big.Int).Mul(j.y, mod(new(big.Int).Mul(q.z, z2z2))))
	s2 := mod(new(big.Int).Mul(q.y, mod(new(big.Int).Mul(j.z, z1z1))))
	if u1.Cmp(u2) == 0 {
		if s1.Cmp(s2) != 0 {
			return &jacobianPoint{x: new(big.Int), y: new(big.Int), z: new(big.Int).Set(big0)}
		}
		return j.double()
	}
	h := mod(new(big.Int).Sub(u2, u1))
	i := mod(new(big.Int).Mul(big2, h))
	i = mod(new(big.Int).Mul(i, i))
	jj := mod(new(big.Int).Mul(h, i))
	r := mod(new(big.Int).Mul(big2, new(big.Int).Sub(s2, s1)))
	v := mod(new(big.Int).Mul(u1, i))
	x3 := mod(new(big.Int).Sub(new(big.Int).Sub(new(big.Int).Mul(r, r), jj), new(big.Int).Mul(big2, v)))
	y3 := mod(new(big.Int).Sub(new(big.Int).Mul(r, new(big.Int).Sub(v, x3)), new(big.Int).Mul(big2, new(big.Int).Mul(s1, jj))))
	z3 := mod(new(big.Int).Mul(new(big.Int).Mul(big2, new(big.Int).Mul(j.z, q.z)), h))
	return &jacobianPoint{x: x3, y: y3, z: z3}
}

// liftX recovers the even-y affine point from an x-only coordinate.
func liftX(x *big.Int) (*Point, error) {
	if x.Cmp(p) >= 0 {
		return nil, errors.New("x coordinate out of field")
	}
	// c = x^3 + 7
	c := mod(new(big.Int).Add(new(big.Int).Mul(x, new(big.Int).Mul(x, x)), big.NewInt(7)))
	// p % 4 == 3, so y = c^((p+1)/4)
	exp := new(big.Int).Add(p, big1)
	exp.Rsh(exp, 2)
	y := new(big.Int).Exp(c, exp, p)
	y2 := mod(new(big.Int).Mul(y, y))
	if y2.Cmp(c) != 0 {
		return nil, errors.New("no valid y for x coordinate")
	}
	if y.Bit(0) == 1 {
		y.Sub(p, y)
	}
	pt := &Point{X: new(big.Int).Set(x), Y: y}
	if !pt.IsOnCurve() {
		return nil, errors.New("point not on curve")
	}
	return pt, nil
}

// TaggedHash implements BIP-340's tagged hash:
// sha256(sha256(tag) || sha256(tag) || msg).
func TaggedHash(tag string, msg []byte) []byte {
	tagHash := sha256.Sum256([]byte(tag))
	h := sha256.New()
	h.Write(tagHash[:])
	h.Write(tagHash[:])
	h.Write(msg)
	return h.Sum(nil)
}

// GeneratePrivateKey returns a random scalar in [1, n-1].
func GeneratePrivateKey() (*big.Int, error) {
	b := make([]byte, 32)
	for {
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		d := new(big.Int).SetBytes(b)
		if d.Sign() > 0 && d.Cmp(n) < 0 {
			return d, nil
		}
	}
}

// XOnlyPublicKey returns the 32-byte x-only public key for private key d.
func XOnlyPublicKey(d *big.Int) []byte {
	pt := ScalarBaseMult(d)
	return pt.X.FillBytes(make([]byte, 32))
}

// SchnorrSign produces a BIP-340 signature. aux should be 32 random bytes
// (it may be zeros). Returns the 64-byte signature (rx || s).
func SchnorrSign(d *big.Int, msg, aux []byte) ([]byte, error) {
	if d.Sign() <= 0 || d.Cmp(n) >= 0 {
		return nil, errors.New("invalid private key")
	}
	if len(msg) != 32 || len(aux) != 32 {
		return nil, errors.New("msg and aux must be 32 bytes")
	}
	d0 := new(big.Int).Set(d)
	P := ScalarBaseMult(d0)
	px := P.X.FillBytes(make([]byte, 32))
	dd := new(big.Int).Set(d0)
	if P.Y.Bit(0) == 1 {
		dd.Sub(n, d0)
	}
	// t = bytes(d) XOR tagged_hash("BIP0340/aux", aux)
	tHash := TaggedHash("BIP0340/aux", aux)
	dBytes := dd.FillBytes(make([]byte, 32))
	t := make([]byte, 32)
	for i := range t {
		t[i] = dBytes[i] ^ tHash[i]
	}
	// rand = int(tagged_hash("BIP0340/nonce", t || px || msg))
	nonceInput := make([]byte, 0, 96)
	nonceInput = append(nonceInput, t...)
	nonceInput = append(nonceInput, px...)
	nonceInput = append(nonceInput, msg...)
	k := new(big.Int).SetBytes(TaggedHash("BIP0340/nonce", nonceInput))
	k.Mod(k, n)
	if k.Sign() == 0 {
		return nil, errors.New("derived nonce is zero")
	}
	R := ScalarBaseMult(k)
	if R.Y.Bit(0) == 1 {
		k.Sub(n, k)
	}
	rx := R.X.FillBytes(make([]byte, 32))
	// e = int(tagged_hash("BIP0340/challenge", rx || px || msg)) mod n
	chInput := make([]byte, 0, 96)
	chInput = append(chInput, rx...)
	chInput = append(chInput, px...)
	chInput = append(chInput, msg...)
	e := new(big.Int).SetBytes(TaggedHash("BIP0340/challenge", chInput))
	e.Mod(e, n)
	// s = k + e*d mod n
	s := new(big.Int).Mod(new(big.Int).Add(k, new(big.Int).Mul(e, dd)), n)
	sig := make([]byte, 64)
	copy(sig[0:32], rx)
	copy(sig[32:64], s.FillBytes(make([]byte, 32)))
	return sig, nil
}

// SchnorrVerify validates a BIP-340 signature. pubX is the 32-byte x-only
// public key, msg the 32-byte message, and sig the 64-byte signature.
func SchnorrVerify(pubX, msg, sig []byte) bool {
	if len(pubX) != 32 || len(msg) != 32 || len(sig) != 64 {
		return false
	}
	rx := new(big.Int).SetBytes(sig[0:32])
	s := new(big.Int).SetBytes(sig[32:64])
	if rx.Cmp(p) >= 0 || s.Cmp(n) >= 0 {
		return false
	}
	P, err := liftX(new(big.Int).SetBytes(pubX))
	if err != nil {
		return false
	}
	R0, err := liftX(new(big.Int).Set(rx))
	if err != nil {
		return false
	}
	chInput := make([]byte, 0, 96)
	chInput = append(chInput, sig[0:32]...)
	chInput = append(chInput, pubX...)
	chInput = append(chInput, msg...)
	e := new(big.Int).SetBytes(TaggedHash("BIP0340/challenge", chInput))
	e.Mod(e, n)
	// R = s*G - e*P
	sg := ScalarBaseMult(s)
	ep := P.ScalarMult(e).Neg()
	R := sg.Add(ep)
	if R.IsInfinity() || R.X.Cmp(rx) != 0 {
		return false
	}
	_ = R0 // R0 used to enforce even-y via liftX(rx); recomputed R must match
	return R.Y.Bit(0) == 0
}
