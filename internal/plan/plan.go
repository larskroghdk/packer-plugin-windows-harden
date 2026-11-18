// Package plan compares the guest state with a baseline, decides which
// settings are safe to apply without cutting off the Packer WinRM session, and
// verifies the result.
package plan

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/larskroghdk/packer-plugin-windows-harden/internal/baseline"
)

// Status is the outcome for one setting.
type Status string

const (
	StatusCompliant     Status = "compliant"
	StatusHardened      Status = "hardened"
	StatusNotApplicable Status = "not_applicable"
	StatusSkipped       Status = "skipped"
	StatusDeferred      Status = "deferred"
	StatusFailed        Status = "failed"
	StatusPending       Status = "pending"
)

// Item is the plan entry for one setting. Before is the state found by the
// first inspection and After the state after the change, when known.
type Item struct {
	Setting baseline.Setting
	Before  *State
	After   *State
	Status  Status
	Action  Action
	Reason  string
}

// Plan holds one item per baseline setting, in baseline order.
type Plan struct {
	Items []*Item
}

// Build creates the plan. Excluded settings are skipped, compliant settings
// need no change, and settings whose object does not exist are not
// applicable. Every other setting goes through Protect and ends up pending
// with ActionApply or ActionGuard, or deferred.
//
// Parameters:
//   - exclude: Setting ID patterns from the configuration (see MatchPattern).
//   - ctx: The guest facts and current state the rules decide on.
//
// Author: Lars Krogh
// Version: 1.0.0
func Build(settings []baseline.Setting, exclude []string, ctx Context) *Plan {
	p := &Plan{}
	var changes []baseline.Setting
	for _, s := range settings {
		item := &Item{Setting: s}
		p.Items = append(p.Items, item)
		if pattern, ok := matchAny(exclude, s.ID); ok {
			item.Status = StatusSkipped
			item.Reason = "excluded by configuration (" + pattern + ")"
			continue
		}
		st, ok := ctx.Current.Get(s.ID)
		if !ok {
			item.Status = StatusFailed
			item.Reason = "the guest returned no inspection result for this setting"
			continue
		}
		item.Before = &st
		if st.Error != nil {
			item.Status = StatusFailed
			item.Reason = "inspection failed: " + *st.Error
			continue
		}
		switch Compare(s, st) {
		case Match:
			item.Status = StatusCompliant
		case NotApplicable:
			item.Status = StatusNotApplicable
			item.Reason = "the configured object does not exist on this system"
		default:
			item.Status = StatusPending
			changes = append(changes, s)
		}
	}
	decisions := Protect(changes, ctx)
	for _, item := range p.Items {
		if item.Status != StatusPending {
			continue
		}
		d := decisions[item.Setting.ID]
		item.Reason = d.Reason
		if d.Action == ActionDefer {
			item.Status = StatusDeferred
			continue
		}
		item.Action = d.Action
	}
	return p
}

// Pending returns the pending items that will be applied with action a.
//
// Author: Lars Krogh
// Version: 1.0.0
func (p *Plan) Pending(a Action) []*Item {
	var out []*Item
	for _, it := range p.Items {
		if it.Status == StatusPending && it.Action == a {
			out = append(out, it)
		}
	}
	return out
}

// Count returns the number of items with status s.
//
// Author: Lars Krogh
// Version: 1.0.0
func (p *Plan) Count(s Status) int {
	n := 0
	for _, it := range p.Items {
		if it.Status == s {
			n++
		}
	}
	return n
}

// Settings returns the baseline settings of items, in the same order.
//
// Author: Lars Krogh
// Version: 1.0.0
func Settings(items []*Item) []baseline.Setting {
	out := make([]baseline.Setting, len(items))
	for i, it := range items {
		out[i] = it.Setting
	}
	return out
}

// ApplyResult is the guest's result of applying one setting.
type ApplyResult struct {
	ID      string  `json:"id"`
	OK      bool    `json:"ok"`
	Error   *string `json:"error"`
	Warning *string `json:"warning"`
}

// ApplyResults maps lower-case setting IDs to apply results.
type ApplyResults map[string]ApplyResult

// Get returns the result for a setting ID, matched case-insensitively.
//
// Returns:
//   - ApplyResult: The result.
//   - bool: Whether the guest returned a result for the ID.
//
// Author: Lars Krogh
// Version: 1.0.0
func (r ApplyResults) Get(id string) (ApplyResult, bool) {
	res, ok := r[strings.ToLower(id)]
	return res, ok
}

// ParseApplyResults decodes the Apply result.
//
// Returns:
//   - ApplyResults: The results by setting ID.
//   - error: The data is not valid JSON.
//
// Author: Lars Krogh
// Version: 1.0.0
func ParseApplyResults(data []byte) (ApplyResults, error) {
	var res struct {
		Items []ApplyResult `json:"items"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("decode apply result: %w", err)
	}
	out := make(ApplyResults, len(res.Items))
	for _, r := range res.Items {
		out[strings.ToLower(r.ID)] = r
	}
	return out, nil
}

// Reverify re-checks compliant and hardened items against current and marks
// those that no longer match as failed. when is appended to the reason, for
// example "after restart".
//
// Author: Lars Krogh
// Version: 1.0.0
func Reverify(items []*Item, current States, when string) {
	for _, it := range items {
		if it.Status != StatusCompliant && it.Status != StatusHardened {
			continue
		}
		st, ok := current.Get(it.Setting.ID)
		switch {
		case !ok:
			it.Status, it.Reason = StatusFailed, "could not be re-inspected "+when
		case st.Error != nil:
			it.Status, it.Reason = StatusFailed, "re-inspection "+when+" failed: "+*st.Error
		case Compare(it.Setting, st) != Match:
			it.After = &st
			it.Status, it.Reason = StatusFailed, "no longer compliant "+when+": the value is "+st.String()
		}
	}
}

// Verify sets the final status of applied items. An item is hardened only when
// the guest reported success and the re-inspected value matches the baseline.
// Anything else is failed.
//
// Author: Lars Krogh
// Version: 1.0.0
func Verify(items []*Item, applied ApplyResults, after States) {
	for _, it := range items {
		res, ok := applied.Get(it.Setting.ID)
		st, inspected := after.Get(it.Setting.ID)

		if inspected {
			it.After = &st
		}
		switch {
		case !ok:
			it.Status, it.Reason = StatusFailed, "the guest returned no apply result for this setting"
		case !res.OK:
			it.Status, it.Reason = StatusFailed, "apply failed: "+deref(res.Error)
		case !inspected:
			it.Status, it.Reason = StatusFailed, "the setting was applied but could not be re-inspected"
		case st.Error != nil:
			it.Status, it.Reason = StatusFailed, "verification failed: "+*st.Error
		case Compare(it.Setting, st) != Match:
			it.Status, it.Reason = StatusFailed, "verification failed: the value after applying is "+st.String()
		default:
			it.Status = StatusHardened
			it.Reason = joinReason(it.Reason, deref(res.Warning))
		}
	}
}

// deref returns the string s points to, or "" for nil.
//
// Author: Lars Krogh
// Version: 1.0.0
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// joinReason joins two reasons with "; ", omitting empty ones.
//
// Author: Lars Krogh
// Version: 1.0.0
func joinReason(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + "; " + b
}

// matchAny returns the first pattern that matches id.
//
// Returns:
//   - string: The matching pattern.
//   - bool: Whether any pattern matched.
//
// Author: Lars Krogh
// Version: 1.0.0
func matchAny(patterns []string, id string) (string, bool) {
	for _, p := range patterns {
		if MatchPattern(p, id) {
			return p, true
		}
	}
	return "", false
}

// MatchPattern reports whether a setting ID matches an exclude pattern. The
// match is case-insensitive and covers the whole ID, '*' matches any sequence,
// and every other character, including '\', is literal.
//
// Author: Lars Krogh
// Version: 1.0.0
func MatchPattern(pattern, id string) bool {
	pattern, id = strings.ToLower(pattern), strings.ToLower(id)
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == id
	}
	if !strings.HasPrefix(id, parts[0]) {
		return false
	}
	id = id[len(parts[0]):]
	for _, mid := range parts[1 : len(parts)-1] {
		i := strings.Index(id, mid)
		if i < 0 {
			return false
		}
		id = id[i+len(mid):]
	}
	return strings.HasSuffix(id, parts[len(parts)-1])
}
