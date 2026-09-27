package protocol

import (
	"encoding/json"
	"testing"
)

func TestFilterJSONAndTags(t *testing.T) {
	raw := `{"ids":["id1"],"authors":["a1"],"kinds":[1,3],"since":100,"until":200,"limit":10,"search":"hello world","#e":["e1","e2"],"#t":["tag1"]}`
	var f Filter
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		t.Fatal(err)
	}
	if len(f.IDs) != 1 || f.IDs[0] != "id1" {
		t.Fatal("ids decode")
	}
	if len(f.Kinds) != 2 || f.Kinds[0] != 1 || f.Kinds[1] != 3 {
		t.Fatal("kinds decode")
	}
	if f.Since == nil || *f.Since != 100 || f.Until == nil || *f.Until != 200 {
		t.Fatal("since/until decode")
	}
	if f.Limit != 10 || f.Search != "hello world" {
		t.Fatal("limit/search decode")
	}
	if got := f.Tags["e"]; len(got) != 2 || got[0] != "e1" || got[1] != "e2" {
		t.Fatalf("#e decode: %v", got)
	}
	if got := f.Tags["t"]; len(got) != 1 || got[0] != "tag1" {
		t.Fatal("#t decode")
	}
	// Round-trip preserves tag queries.
	out, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	var f2 Filter
	if err := json.Unmarshal(out, &f2); err != nil {
		t.Fatal(err)
	}
	if len(f2.Tags["e"]) != 2 || len(f2.Tags["t"]) != 1 {
		t.Fatalf("tag round-trip failed: %s", out)
	}
}

func TestFilterMatch(t *testing.T) {
	e := &Event{ID: "id1", PubKey: "a1", CreatedAt: 150, Kind: 1,
		Tags: Tags{{"e", "e1"}, {"t", "foo"}}, Content: "Hello Nostromo"}

	since, until := int64(100), int64(200)
	good := Filter{IDs: []string{"id1"}, Authors: []string{"a1"}, Kinds: []int{1},
		Since: &since, Until: &until, Tags: map[string][]string{"e": {"e1"}, "t": {"foo"}}}
	if !good.Match(e) {
		t.Fatal("matching filter rejected event")
	}
	// Search term must occur (case-insensitive), all terms AND.
	good.Search = "nostromo"
	if !good.Match(e) {
		t.Fatal("search match failed")
	}
	good.Search = "nostromo missing"
	if good.Match(e) {
		t.Fatal("AND search should fail on missing term")
	}
	// Wrong id.
	bad := good
	bad.IDs = []string{"other"}
	bad.Search = ""
	if bad.Match(e) {
		t.Fatal("non-matching id accepted")
	}
	// Out of time range.
	outRange := good
	early := int64(1000)
	outRange.Since = &early
	outRange.Search = ""
	if outRange.Match(e) {
		t.Fatal("since filter failed")
	}
	// Missing required tag.
	noTag := good
	noTag.Tags = map[string][]string{"p": {"p1"}}
	noTag.Search = ""
	if noTag.Match(e) {
		t.Fatal("tag filter failed")
	}
}

func TestEffectiveLimit(t *testing.T) {
	f := Filter{}
	if f.EffectiveLimit(500) != 500 {
		t.Fatal("default limit")
	}
	f.Limit = 10
	if f.EffectiveLimit(500) != 10 {
		t.Fatal("client limit")
	}
}
