package baseline

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"sort"
)

//go:embed data/*.json
var dataFS embed.FS

// Catalog holds the embedded baselines, sorted by ID.
type Catalog struct {
	baselines []*Baseline
}

// Load decodes, validates and indexes every embedded baseline. Unknown JSON
// fields are rejected.
//
// Returns:
//   - *Catalog: The catalog.
//   - error: A baseline cannot be decoded, is invalid, or applies to a build
//     that another baseline already covers.
//
// Author: Lars Krogh
// Version: 1.0.0
func Load() (*Catalog, error) {
	entries, err := dataFS.ReadDir("data")
	if err != nil {
		return nil, err
	}
	c := &Catalog{}
	for _, e := range entries {
		data, err := dataFS.ReadFile(path.Join("data", e.Name()))
		if err != nil {
			return nil, err
		}
		var b Baseline
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&b); err != nil {
			return nil, fmt.Errorf("baseline %s: %w", e.Name(), err)
		}
		if err := b.Validate(); err != nil {
			return nil, err
		}
		c.baselines = append(c.baselines, &b)
	}
	sort.Slice(c.baselines, func(i, j int) bool { return c.baselines[i].ID < c.baselines[j].ID })
	if err := c.checkOverlap(); err != nil {
		return nil, err
	}
	return c, nil
}

// checkOverlap ensures that no product type and build pair is covered by more
// than one baseline.
//
// Returns:
//   - error: Two baselines overlap.
//
// Author: Lars Krogh
// Version: 1.0.0
func (c *Catalog) checkOverlap() error {
	owner := map[string]string{}
	for _, b := range c.baselines {
		for _, build := range b.AppliesTo.Builds {
			key := fmt.Sprintf("%s/%d", b.AppliesTo.ProductType, build)
			if other, ok := owner[key]; ok {
				return fmt.Errorf("baselines %s and %s both apply to %s", other, b.ID, key)
			}
			owner[key] = b.ID
		}
	}
	return nil
}

// Baselines returns the baselines in ID order. The slice must not be modified.
//
// Author: Lars Krogh
// Version: 1.0.0
func (c *Catalog) Baselines() []*Baseline { return c.baselines }

// Get returns the baseline with the given ID.
//
// Returns:
//   - *Baseline: The baseline, or nil.
//   - bool: Whether the ID exists.
//
// Author: Lars Krogh
// Version: 1.0.0
func (c *Catalog) Get(id string) (*Baseline, bool) {
	for _, b := range c.baselines {
		if b.ID == id {
			return b, true
		}
	}
	return nil, false
}

// Select returns the baseline for a product type ("server" or "workstation")
// and build number.
//
// Returns:
//   - *Baseline: The matching baseline.
//   - error: No baseline is implemented for the combination.
//
// Author: Lars Krogh
// Version: 1.0.0
func (c *Catalog) Select(productType string, build int) (*Baseline, error) {
	for _, b := range c.baselines {
		if b.AppliesTo.ProductType == productType && slices.Contains(b.AppliesTo.Builds, build) {
			return b, nil
		}
	}
	return nil, fmt.Errorf("no Microsoft security baseline is implemented for %s build %d", productType, build)
}
