package crypto

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex: %v", err)
	}
	return b
}

// Official BIP-340 test vectors (bip-0340/test-vectors.csv), first two
// valid sign+verify cases.
type vector struct {
	priv, pub, aux, msg, sig string
}

var vectors = []vector{
	{
		priv: "0000000000000000000000000000000000000000000000000000000000000003",
		pub:  "F9308A019258C31049344F85F89D5229B531C845836F99B08601F113BCE036F9",
		aux:  "0000000000000000000000000000000000000000000000000000000000000000",
		msg:  "0000000000000000000000000000000000000000000000000000000000000000",
		sig:  "E907831F80848D1069A5371B402410364BDF1C5F8307B0084C55F1CE2DCA821525F66A4A85EA8B71E482A74F382D2CE5EBEEE8FDB2172F477DF4900D310536C0",
	},
}

func TestSchnorrOfficialVectors(t *testing.T) {
	for i, v := range vectors {
		priv := new(big.Int).SetBytes(mustHex(t, v.priv))
		pub := mustHex(t, v.pub)
		aux := mustHex(t, v.aux)
		msg := mustHex(t, v.msg)
		wantSig := mustHex(t, v.sig)

		if got := XOnlyPublicKey(priv); hex.EncodeToString(got) != strings.ToLower(v.pub) {
			t.Fatalf("vector %d: pubkey mismatch\n got %x\nwant %s", i, got, v.pub)
		}
		gotSig, err := SchnorrSign(priv, msg, aux)
		if err != nil {
			t.Fatalf("vector %d: sign error: %v", i, err)
		}
		if hex.EncodeToString(gotSig) != strings.ToLower(v.sig) {
			t.Fatalf("vector %d: signature mismatch\n got %x\nwant %s", i, gotSig, v.sig)
		}
		if !SchnorrVerify(pub, msg, wantSig) {
			t.Fatalf("vector %d: official signature failed verification", i)
		}
	}
}

func TestSchnorrTamper(t *testing.T) {
	v := vectors[0]
	priv := new(big.Int).SetBytes(mustHex(t, v.priv))
	pub := mustHex(t, v.pub)
	msg := mustHex(t, v.msg)
	sig, err := SchnorrSign(priv, msg, mustHex(t, v.aux))
	if err != nil {
		t.Fatal(err)
	}
	// Flip a bit in the message.
	badMsg := append([]byte{}, msg...)
	badMsg[0] ^= 1
	if SchnorrVerify(pub, badMsg, sig) {
		t.Fatal("signature verified against tampered message")
	}
	// Flip a bit in the signature.
	badSig := append([]byte{}, sig...)
	badSig[5] ^= 1
	if SchnorrVerify(pub, msg, badSig) {
		t.Fatal("tampered signature verified")
	}
	// Wrong pubkey.
	badPub := append([]byte{}, pub...)
	badPub[0] ^= 1
	if SchnorrVerify(badPub, msg, sig) {
		t.Fatal("signature verified against wrong pubkey")
	}
}

func TestGenerateAndRoundTrip(t *testing.T) {
	for i := 0; i < 8; i++ {
		d, err := GeneratePrivateKey()
		if err != nil {
			t.Fatal(err)
		}
		pub := XOnlyPublicKey(d)
		msg := make([]byte, 32)
		for j := range msg {
			msg[j] = byte(i*7 + j)
		}
		aux := make([]byte, 32)
		sig, err := SchnorrSign(d, msg, aux)
		if err != nil {
			t.Fatal(err)
		}
		if !SchnorrVerify(pub, msg, sig) {
			t.Fatalf("round-trip verification failed at iter %d", i)
		}
	}
}

func TestCurveOps(t *testing.T) {
	g := G()
	if !g.IsOnCurve() {
		t.Fatal("generator not on curve")
	}
	// 3*G must equal the vector-0 public key via liftX x coordinate.
	p3 := ScalarBaseMult(big.NewInt(3))
	if hex.EncodeToString(p3.X.FillBytes(make([]byte, 32))) != strings.ToLower(vectors[0].pub) {
		t.Fatal("3*G mismatch")
	}
	// G + G == 2*G.
	if g.Add(g).X.Cmp(g.Double().X) != 0 {
		t.Fatal("add vs double mismatch")
	}
	// n*G == infinity.
	if !ScalarBaseMult(n).IsInfinity() {
		t.Fatal("n*G should be infinity")
	}
}
