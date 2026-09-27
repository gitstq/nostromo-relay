// Command seed generates a handful of valid signed Nostr events and writes them
// as JSONL. It is handy for quickly populating a development relay, e.g.
//
//	go run ./examples/seed -out seed.jsonl
//	nostromo replay --to ws://127.0.0.1:7777 seed.jsonl
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/gitstq/nostromo-relay/internal/crypto"
	"github.com/gitstq/nostromo-relay/internal/protocol"
)

func main() {
	out := flag.String("out", "seed.jsonl", "output JSONL file")
	count := flag.Int("n", 6, "number of text notes to generate")
	flag.Parse()

	priv, err := crypto.GeneratePrivateKey()
	if err != nil {
		fatal(err)
	}
	pub := crypto.XOnlyPublicKey(priv)
	fmt.Printf("generated author pubkey: %s\n", hex(pub))

	f, err := os.Create(*out)
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	now := time.Now().Unix()

	// A replaceable metadata event (kind 0).
	meta := protocol.Event{CreatedAt: now, Kind: protocol.KindSetMetadata,
		Content: `{"name":"seed","about":"A seeded test author","picture":""}`}
	sign(&meta, priv)
	enc.Encode(meta)

	// Text notes.
	contents := []string{
		"Hello Nostromo! 🛰️",
		"Zero dependencies, one binary.",
		"Nostr events are signed logs.",
		"Developer consoles should be real-time.",
		"Replay makes tests reproducible.",
		"Ship it.",
	}
	for i := 0; i < *count && i < len(contents); i++ {
		e := protocol.Event{
			CreatedAt: now + int64(i),
			Kind:      protocol.KindTextNote,
			Tags:      protocol.Tags{{"t", "nostromo"}, {"t", "seed"}},
			Content:   contents[i],
		}
		sign(&e, priv)
		enc.Encode(e)
	}
	fmt.Printf("wrote events to %s\n", *out)
}

func sign(e *protocol.Event, priv *big.Int) {
	if err := protocol.SignEvent(e, priv); err != nil {
		fatal(err)
	}
}

func hex(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, x := range b {
		out[i*2] = digits[x>>4]
		out[i*2+1] = digits[x&0xF]
	}
	return string(out)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "seed:", err)
	os.Exit(1)
}
