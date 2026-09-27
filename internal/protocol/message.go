package protocol

import (
	"encoding/json"
	"fmt"
)

// ClientMessage is a message sent from a client to the relay.
type ClientMessage interface{ clientType() string }

// EventMessage publishes an event: ["EVENT", event].
type EventMessage struct{ Event Event }

func (EventMessage) clientType() string { return "EVENT" }

// ReqMessage opens or updates a subscription: ["REQ", subID, ...filters].
type ReqMessage struct {
	SubID   string
	Filters []Filter
}

func (ReqMessage) clientType() string { return "REQ" }

// CloseMessage ends a subscription: ["CLOSE", subID].
type CloseMessage struct{ SubID string }

func (CloseMessage) clientType() string { return "CLOSE" }

// AuthMessage responds to an authentication challenge: ["AUTH", event].
type AuthMessage struct{ Event Event }

func (AuthMessage) clientType() string { return "AUTH" }

const maxSubIDLen = 64

func validSubID(s string) bool {
	if len(s) == 0 || len(s) > maxSubIDLen {
		return false
	}
	for _, r := range s {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return false
		}
	}
	return true
}

// ParseClientMessage decodes a single client message.
func ParseClientMessage(raw []byte) (ClientMessage, error) {
	var head []json.RawMessage
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, fmt.Errorf("message is not a JSON array")
	}
	if len(head) == 0 {
		return nil, fmt.Errorf("empty message")
	}
	var typ string
	if err := json.Unmarshal(head[0], &typ); err != nil {
		return nil, fmt.Errorf("missing message type")
	}
	switch typ {
	case "EVENT", "AUTH":
		if len(head) < 2 {
			return nil, fmt.Errorf("%s missing event", typ)
		}
		var ev Event
		if err := json.Unmarshal(head[1], &ev); err != nil {
			return nil, fmt.Errorf("%s invalid event: %w", typ, err)
		}
		if typ == "EVENT" {
			return &EventMessage{Event: ev}, nil
		}
		return &AuthMessage{Event: ev}, nil
	case "REQ":
		if len(head) < 3 {
			return nil, fmt.Errorf("REQ requires a subscription id and at least one filter")
		}
		var subID string
		if err := json.Unmarshal(head[1], &subID); err != nil || !validSubID(subID) {
			return nil, fmt.Errorf("REQ has invalid subscription id")
		}
		filters := make([]Filter, 0, len(head)-2)
		for _, fb := range head[2:] {
			var f Filter
			if err := json.Unmarshal(fb, &f); err != nil {
				return nil, fmt.Errorf("REQ invalid filter: %w", err)
			}
			filters = append(filters, f)
		}
		return &ReqMessage{SubID: subID, Filters: filters}, nil
	case "CLOSE":
		if len(head) < 2 {
			return nil, fmt.Errorf("CLOSE missing subscription id")
		}
		var subID string
		if err := json.Unmarshal(head[1], &subID); err != nil || !validSubID(subID) {
			return nil, fmt.Errorf("CLOSE has invalid subscription id")
		}
		return &CloseMessage{SubID: subID}, nil
	default:
		return nil, fmt.Errorf("unknown message type %q", typ)
	}
}

// EncodeOutgoingEvent builds ["EVENT", subID, event].
func EncodeOutgoingEvent(subID string, e *Event) []byte {
	return mustMarshal([]interface{}{"EVENT", subID, e})
}

// EncodeOK builds ["OK", eventID, saved, message].
func EncodeOK(id string, saved bool, message string) []byte {
	return mustMarshal([]interface{}{"OK", id, saved, message})
}

// EncodeEOSE builds ["EOSE", subID].
func EncodeEOSE(subID string) []byte { return mustMarshal([]interface{}{"EOSE", subID}) }

// EncodeClosed builds ["CLOSED", subID, message].
func EncodeClosed(subID, message string) []byte {
	return mustMarshal([]interface{}{"CLOSED", subID, message})
}

// EncodeNotice builds ["NOTICE", message].
func EncodeNotice(message string) []byte { return mustMarshal([]interface{}{"NOTICE", message}) }

// EncodeAuthChallenge builds ["AUTH", challenge].
func EncodeAuthChallenge(challenge string) []byte {
	return mustMarshal([]interface{}{"AUTH", challenge})
}

func mustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("protocol: failed to marshal: %v", err))
	}
	return b
}
