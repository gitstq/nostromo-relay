package protocol

import (
	"encoding/json"
	"strings"
)

// Filter is a Nostr subscription filter (NIP-01), with NIP-12 generic tag
// queries and a NIP-50 search field.
type Filter struct {
	IDs     []string            `json:"ids,omitempty"`
	Authors []string            `json:"authors,omitempty"`
	Kinds   []int               `json:"kinds,omitempty"`
	Since   *int64              `json:"since,omitempty"`
	Until   *int64              `json:"until,omitempty"`
	Limit   int                 `json:"limit,omitempty"`
	Search  string              `json:"search,omitempty"`
	Tags    map[string][]string `json:"-"` // keys are single tag letters
}

// Match reports whether the event satisfies the filter.
func (f *Filter) Match(e *Event) bool {
	if len(f.IDs) > 0 && !containsStr(f.IDs, e.ID) {
		return false
	}
	if len(f.Authors) > 0 && !containsStr(f.Authors, e.PubKey) {
		return false
	}
	if len(f.Kinds) > 0 && !containsInt(f.Kinds, e.Kind) {
		return false
	}
	if f.Since != nil && e.CreatedAt < *f.Since {
		return false
	}
	if f.Until != nil && e.CreatedAt > *f.Until {
		return false
	}
	for letter, vals := range f.Tags {
		if !eventHasTag(e, letter, vals) {
			return false
		}
	}
	if f.Search != "" {
		content := strings.ToLower(e.Content)
		for _, term := range strings.Fields(strings.ToLower(f.Search)) {
			if !strings.Contains(content, term) {
				return false
			}
		}
	}
	return true
}

func eventHasTag(e *Event, letter string, vals []string) bool {
	for _, tag := range e.Tags {
		if len(tag) >= 2 && tag[0] == letter {
			for _, v := range vals {
				if tag[1] == v {
					return true
				}
			}
		}
	}
	return false
}

func containsStr(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// EffectiveLimit returns the client limit, falling back to def when unset (<=0).
func (f *Filter) EffectiveLimit(def int) int {
	if f.Limit <= 0 {
		return def
	}
	return f.Limit
}

type filterAlias Filter

// MarshalJSON encodes the filter including "#<letter>" tag queries.
func (f Filter) MarshalJSON() ([]byte, error) {
	var raw map[string]json.RawMessage
	b, err := json.Marshal(filterAlias(f))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		raw = map[string]json.RawMessage{}
	}
	for letter, vals := range f.Tags {
		vb, err := json.Marshal(vals)
		if err != nil {
			return nil, err
		}
		raw["#"+letter] = vb
	}
	return json.Marshal(raw)
}

// UnmarshalJSON decodes the filter, collecting "#<letter>" tag queries.
func (f *Filter) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	tags := map[string][]string{}
	for key, val := range raw {
		if len(key) == 2 && key[0] == '#' && key[1] != '_' {
			var vals []string
			if err := json.Unmarshal(val, &vals); err != nil {
				return err
			}
			tags[string(key[1])] = vals
		}
	}
	delete(raw, "")
	// Re-marshal without the tag keys, then decode into the struct.
	for key := range raw {
		if len(key) == 2 && key[0] == '#' {
			delete(raw, key)
		}
	}
	clean, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	var alias filterAlias
	if err := json.Unmarshal(clean, &alias); err != nil {
		return err
	}
	*f = Filter(alias)
	f.Tags = tags
	return nil
}
