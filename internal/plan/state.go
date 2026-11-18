package plan

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/larskroghdk/packer-plugin-windows-harden/internal/baseline"
)

// State is the guest's view of one setting. Present is false when the value or
// object does not exist, and Error is set when it could not be read.
type State struct {
	ID      string  `json:"id"`
	Present bool    `json:"present"`
	Type    *string `json:"type"`
	Value   *string `json:"value"`
	Error   *string `json:"error"`
}

// String describes the state for reports: "(error)", "(absent)", the typed
// value, or "(no one)" for an empty user right.
//
// Author: Lars Krogh
// Version: 1.0.0
func (s State) String() string {
	switch {
	case s.Error != nil:
		return "(error)"
	case !s.Present:
		return "(absent)"
	case s.Type != nil && s.Value != nil:
		return *s.Type + " " + *s.Value
	case s.Value != nil:
		if *s.Value == "" {
			return "(no one)"
		}
		return *s.Value
	}
	return "(unknown)"
}

// value returns the state's value, or "" when it has none.
//
// Author: Lars Krogh
// Version: 1.0.0
func (s State) value() string { return deref(s.Value) }

// States maps lower-case setting IDs to guest state.
type States map[string]State

// Get returns the state for a setting ID, matched case-insensitively.
//
// Returns:
//   - State: The state.
//   - bool: Whether the guest returned a state for the ID.
//
// Author: Lars Krogh
// Version: 1.0.0
func (s States) Get(id string) (State, bool) {
	st, ok := s[strings.ToLower(id)]
	return st, ok
}

// ParseStates decodes the Inspect result.
//
// Returns:
//   - States: The states by setting ID.
//   - error: The data is not valid JSON.
//
// Author: Lars Krogh
// Version: 1.0.0
func ParseStates(data []byte) (States, error) {
	var res struct {
		Items []State `json:"items"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("decode inspect result: %w", err)
	}
	out := make(States, len(res.Items))
	for _, s := range res.Items {
		out[strings.ToLower(s.ID)] = s
	}
	return out, nil
}

// Comparison is the result of comparing guest state with a setting.
type Comparison int

const (
	Mismatch Comparison = iota
	Match
	NotApplicable
)

// Compare checks state st against setting s. DWORDs are compared numerically,
// strings exactly, and user rights as normalized SID sets. A service that is
// not installed is NotApplicable.
//
// Author: Lars Krogh
// Version: 1.0.0
func Compare(s baseline.Setting, st State) Comparison {
	switch s.Kind {
	case baseline.KindRegistry:
		r := s.Registry
		if r.Type == baseline.RegAbsent {
			if !st.Present {
				return Match
			}
			return Mismatch
		}
		if !st.Present || st.Type == nil || *st.Type != r.Type {
			return Mismatch
		}
		if r.Type == baseline.RegDWORD {
			want, err1 := strconv.ParseUint(r.Data, 10, 32)
			got, err2 := strconv.ParseUint(st.value(), 10, 32)
			if err1 != nil || err2 != nil || want != got {
				return Mismatch
			}
			return Match
		}
		if st.value() == r.Data {
			return Match
		}
	case baseline.KindUserRight:
		if st.Present && normalizeSIDs(st.value()) == normalizeSIDs(strings.Join(s.UserRight.SIDs, ",")) {

			return Match
		}
	case baseline.KindAccountPolicy:
		if st.Present && strings.TrimSpace(st.value()) == s.AccountPolicy.Value {
			return Match
		}
	case baseline.KindAuditPolicy:
		if st.Present && st.value() == strconv.Itoa(s.AuditPolicy.Setting) {
			return Match
		}
	case baseline.KindService:
		if !st.Present {
			return NotApplicable
		}
		if st.value() == strconv.Itoa(s.Service.StartType) {
			return Match
		}
	}
	return Mismatch
}

// normalizeSIDs upper-cases, trims and sorts a comma-separated SID list, and
// drops empty entries.
//
// Author: Lars Krogh
// Version: 1.0.0
func normalizeSIDs(v string) string {
	var sids []string
	for _, s := range strings.Split(v, ",") {
		if s = strings.ToUpper(strings.TrimSpace(s)); s != "" {
			sids = append(sids, s)
		}
	}
	sort.Strings(sids)
	return strings.Join(sids, ",")
}
