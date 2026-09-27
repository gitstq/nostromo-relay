package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseEventMessage(t *testing.T) {
	raw := `["EVENT",{"id":"id","pubkey":"pk","created_at":1,"kind":1,"tags":[],"content":"hi","sig":"sg"}]`
	m, err := ParseClientMessage([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	em, ok := m.(*EventMessage)
	if !ok || em.Event.Content != "hi" {
		t.Fatal("event message parse")
	}
}

func TestParseReqMessage(t *testing.T) {
	raw := `["REQ","sub1",{"kinds":[1]},{"kinds":[3],"limit":5}]`
	m, err := ParseClientMessage([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	rm, ok := m.(*ReqMessage)
	if !ok || rm.SubID != "sub1" || len(rm.Filters) != 2 {
		t.Fatal("req message parse")
	}
	if rm.Filters[1].Limit != 5 {
		t.Fatal("second filter parse")
	}
}

func TestParseCloseAndAuth(t *testing.T) {
	if m, err := ParseClientMessage([]byte(`["CLOSE","sub1"]`)); err != nil {
		t.Fatal(err)
	} else if cm, ok := m.(*CloseMessage); !ok || cm.SubID != "sub1" {
		t.Fatal("close parse")
	}
	if m, err := ParseClientMessage([]byte(`["AUTH",{"kind":22242}]`)); err != nil {
		t.Fatal(err)
	} else if am, ok := m.(*AuthMessage); !ok || am.Event.Kind != KindAuth {
		t.Fatal("auth parse")
	}
}

func TestParseErrors(t *testing.T) {
	bad := []string{
		`not json`,
		`[]`,
		`["NOPE"]`,
		`["EVENT"]`,
		`["REQ","sub"]`,
		`["REQ","bad id","x"]`,
		`["CLOSE",""]`,
	}
	for _, b := range bad {
		if _, err := ParseClientMessage([]byte(b)); err == nil {
			t.Fatalf("expected error for %s", b)
		}
	}
	// subscription id with whitespace rejected
	if _, err := ParseClientMessage([]byte(`["REQ","bad sub",{}]`)); err == nil {
		t.Fatal("expected sub id error")
	}
}

func TestEncodeMessages(t *testing.T) {
	cases := map[string][]byte{
		`["EOSE","sub1"]`:         EncodeEOSE("sub1"),
		`["NOTICE","error"]`:      EncodeNotice("error"),
		`["AUTH","challenge123"]`: EncodeAuthChallenge("challenge123"),
		`["CLOSED","sub1","bye"]`: EncodeClosed("sub1", "bye"),
		`["OK","id1",true,""]`:    EncodeOK("id1", true, ""),
	}
	for want, got := range cases {
		var norm []interface{}
		if err := json.Unmarshal(got, &norm); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), strings.Split(want, ",")[0][2:]) {
			t.Fatalf("encode mismatch\n got %s\nwant %s", got, want)
		}
	}
	e := &Event{ID: "id", PubKey: "pk", CreatedAt: 1, Kind: 1, Tags: Tags{}, Content: "c", Sig: "s"}
	got := EncodeOutgoingEvent("sub1", e)
	if !strings.HasPrefix(string(got), `["EVENT","sub1",`) {
		t.Fatalf("event encode wrong: %s", got)
	}
}
