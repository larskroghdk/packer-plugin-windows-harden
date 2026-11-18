// Command sctimport converts Microsoft Security Compliance Toolkit packages into
// the baseline JSON files embedded in internal/baseline/data.
//
// Each package listed in sources.json is verified against its pinned SHA-256,
// then every GPO component in it is either imported or explicitly excluded. The
// command fails instead of dropping data it cannot represent.
//
// Usage:
//
//	go run ./cmd/sctimport -downloads <dir with SCT zips> [-sources file] [-out dir]
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"

	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/larskroghdk/packer-plugin-windows-harden/internal/baseline"
)

type sourceSpec struct {
	ID                 string               `json:"id"`
	Name               string               `json:"name"`
	Version            string               `json:"version"`
	Zip                string               `json:"zip"`
	URL                string               `json:"url"`
	SHA256             string               `json:"sha256"`
	PolicyRules        string               `json:"policy_rules"`
	Workbook           string               `json:"workbook"`
	DeltaINF           string               `json:"delta_inf"`
	DeltaText          string               `json:"delta_txt"`
	ProductType        string               `json:"product_type"`
	Builds             []int                `json:"builds"`
	Components         []string             `json:"components"`
	ExcludedComponents []baseline.Exclusion `json:"excluded_components"`
	ExcludedSettings   []baseline.Exclusion `json:"excluded_settings"`
}

// main reads the source specification, generates and validates one baseline
// per entry, and writes each as indented JSON. It stops at the first error.
//
// Author: Lars Krogh
// Version: 1.0.0
func main() {
	sources := flag.String("sources", "cmd/sctimport/sources.json", "baseline source specification")
	downloads := flag.String("downloads", "", "directory containing the downloaded SCT zip files")
	out := flag.String("out", "internal/baseline/data", "output directory")
	flag.Parse()
	if *downloads == "" {
		log.Fatal("-downloads is required")
	}
	specs, err := readSpecs(*sources)
	if err != nil {
		log.Fatal(err)
	}
	for _, spec := range specs {
		b, err := generate(spec, filepath.Join(*downloads, spec.Zip))
		if err != nil {
			log.Fatalf("%s: %v", spec.ID, err)
		}
		if err := b.Validate(); err != nil {
			log.Fatalf("%s: generated baseline is invalid: %v", spec.ID, err)
		}
		data, err := json.MarshalIndent(b, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		path := filepath.Join(*out, spec.ID+".json")
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			log.Fatal(err)
		}
		log.Printf("%s: %d settings, %d non-domain-joined overrides, %d exclusions -> %s",
			spec.ID, len(b.Settings), len(b.NonDomainJoinedOverrides), len(b.Excluded), path)
	}
}

// readSpecs decodes the source specification at path, rejecting unknown fields
// so that a misspelled key fails instead of being ignored.
//
// Returns:
//   - []sourceSpec: The entries in file order.
//   - error: The file cannot be read or is not a valid specification.
//
// Author: Lars Krogh
// Version: 1.0.0
func readSpecs(path string) ([]sourceSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var specs []sourceSpec
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&specs); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return specs, nil
}

type sctPackage struct {
	files []*zip.File
}

// read returns the contents of the one file in the package whose path ends
// with suffix, compared case-insensitively.
//
// Returns:
//   - []byte: The file contents.
//   - error: No file or more than one file matches, or the file cannot be read.
//
// Author: Lars Krogh
// Version: 1.0.0
func (p *sctPackage) read(suffix string) ([]byte, error) {
	var match *zip.File
	for _, f := range p.files {
		if strings.HasSuffix(strings.ToLower(filepath.ToSlash(f.Name)), strings.ToLower(suffix)) {
			if match != nil {
				return nil, fmt.Errorf("%q matches more than one file in the package", suffix)
			}
			match = f
		}
	}
	if match == nil {
		return nil, fmt.Errorf("%q not found in the package", suffix)
	}
	rc, err := match.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// generate builds the baseline for spec from the SCT zip at zipPath. It checks
// the zip's SHA-256, requires every GPO component to be included or excluded,
// imports the included settings, drops the excluded settings and attaches the
// non-domain-joined overrides.
//
// Returns:
//   - *baseline.Baseline: The baseline, not yet validated.
//   - error: The checksum does not match, a component or exclusion is
//     unaccounted for, or a setting cannot be represented.
//
// Author: Lars Krogh
// Version: 1.0.0
func generate(spec sourceSpec, zipPath string) (*baseline.Baseline, error) {
	raw, err := os.ReadFile(zipPath)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); !strings.EqualFold(got, spec.SHA256) {
		return nil, fmt.Errorf("sha256 mismatch for %s: got %s, want %s", spec.Zip, got, spec.SHA256)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}
	p := &sctPackage{files: zr.File}
	policyRulesData, err := p.read(spec.PolicyRules)
	if err != nil {
		return nil, err
	}
	rules, err := parsePolicyRules(policyRulesData)
	if err != nil {
		return nil, err
	}
	workbookData, err := p.read(spec.Workbook)
	if err != nil {
		return nil, err
	}
	names, err := readWorkbook(workbookData)
	if err != nil {
		return nil, err
	}
	included := map[string]bool{}
	for _, c := range spec.Components {
		included[c] = true
	}
	excluded := map[string]bool{}
	for _, e := range spec.ExcludedComponents {
		excluded[e.Item] = true
	}
	present := rules.components()
	for c := range present {
		if !included[c] && !excluded[c] {
			return nil, fmt.Errorf("component %q is neither included nor explicitly excluded", c)
		}
	}
	for c := range included {
		if !present[c] {
			return nil, fmt.Errorf("component %q is not present in %s", c, spec.PolicyRules)
		}
	}
	g := &generator{names: names, byID: map[string]int{}}
	if err := g.addRules(rules, included); err != nil {
		return nil, err
	}
	b := &baseline.Baseline{
		ID:      spec.ID,
		Name:    spec.Name,
		Version: spec.Version,
		Source: baseline.Source{
			Package:     spec.Zip,
			URL:         spec.URL,
			SHA256:      strings.ToLower(spec.SHA256),
			PolicyRules: spec.PolicyRules,
		},
		AppliesTo:  baseline.Applicability{ProductType: spec.ProductType, Builds: spec.Builds},
		Components: spec.Components,
	}
	b.Excluded = append(b.Excluded, spec.ExcludedComponents...)
	b.Excluded = append(b.Excluded, g.excluded...)
	skip := map[string]bool{}
	for _, e := range spec.ExcludedSettings {
		key := strings.ToLower(e.Item)
		if _, ok := g.byID[key]; !ok {
			return nil, fmt.Errorf("excluded setting %s is not in the baseline", e.Item)
		}
		skip[key] = true
	}
	for _, s := range g.settings {
		if !skip[strings.ToLower(s.ID)] {
			b.Settings = append(b.Settings, s)
		}
	}
	b.Excluded = append(b.Excluded, spec.ExcludedSettings...)
	overrides, err := g.deltaOverrides(p, spec)
	if err != nil {
		return nil, err
	}
	for _, s := range overrides {
		if !skip[strings.ToLower(s.ID)] {
			b.NonDomainJoinedOverrides = append(b.NonDomainJoinedOverrides, s)
		}
	}
	return b, nil
}

type generator struct {
	names    *workbookNames
	settings []baseline.Setting
	byID     map[string]int
	excluded []baseline.Exclusion
}

// add records s unless a setting with the same ID was already added. A
// duplicate with the same expected value is ignored.
//
// Returns:
//   - error: A duplicate expects a different value.
//
// Author: Lars Krogh
// Version: 1.0.0
func (g *generator) add(s baseline.Setting) error {
	key := strings.ToLower(s.ID)
	if i, ok := g.byID[key]; ok {
		prev := g.settings[i]
		if prev.ExpectedValue() != s.ExpectedValue() {
			return fmt.Errorf("conflicting values for %s: %q (%s) and %q (%s)",
				s.ID, prev.ExpectedValue(), prev.Component, s.ExpectedValue(), s.Component)
		}
		return nil
	}
	g.byID[key] = len(g.settings)
	g.settings = append(g.settings, s)
	return nil
}

// addRules imports the registry, security template and audit rules of the
// included components. Key-only markers and **delvals. markers are recorded as
// exclusions, and **del. values become settings that require the value to be
// absent.
//
// Returns:
//   - error: A rule uses an unsupported directive, type or value.
//
// Author: Lars Krogh
// Version: 1.0.0
func (g *generator) addRules(rules *policyRules, included map[string]bool) error {
	for _, e := range rules.ComputerConfig {
		if !included[e.PolicyName] {
			continue
		}
		if e.Value == "" && e.RegType == "REG_NONE" {
			g.excluded = append(g.excluded, baseline.Exclusion{
				Item:   "registry:HKLM\\" + e.Key,
				Reason: "key-only marker in the GPO (REG_NONE without a value name); there is no value to enforce",
			})
			continue
		}
		if strings.EqualFold(e.Value, "**delvals.") {
			g.excluded = append(g.excluded, baseline.Exclusion{
				Item: "registry:HKLM\\" + e.Key + "\\**delvals.",
				Reason: "list-replacement marker (delete all other values in the key); the values the GPO defines " +
					"in this key are enforced individually, but pre-existing additional values are not removed",
			})
			continue
		}
		name, regType, data := e.Value, e.RegType, e.RegData
		if strings.HasPrefix(strings.ToLower(name), "**del.") {
			name, regType, data = name[len("**del."):], baseline.RegAbsent, ""
		}
		if strings.HasPrefix(name, "**") {
			return fmt.Errorf("%s\\%s: unsupported registry.pol directive", e.Key, e.Value)
		}
		s, err := g.registrySetting(e.PolicyName, e.Key, name, regType, data)
		if err != nil {
			return err
		}
		if err := g.add(s); err != nil {
			return err
		}
	}
	for _, e := range rules.SecurityTemplate {
		if !included[e.PolicyName] {
			continue
		}
		s, err := g.templateSetting(e.PolicyName, e.Section, e.LineItem)
		if err != nil {
			return err
		}
		if err := g.add(s); err != nil {
			return err
		}
	}
	for _, e := range rules.AuditSubcategory {
		if !included[e.PolicyName] {
			continue
		}
		desc, ok := g.names.audit[strings.ToLower(e.Name)]
		if !ok {
			desc = "Advanced Audit Policy > " + e.Name
		}
		guid := strings.ToLower(strings.Trim(e.GUID, "{}"))
		s := baseline.Setting{
			ID:          baseline.AuditPolicyID(guid),
			Kind:        baseline.KindAuditPolicy,
			Description: desc,
			Component:   e.PolicyName,
			AuditPolicy: &baseline.AuditPolicy{SubcategoryGUID: guid, Subcategory: e.Name, Setting: e.Setting},
		}
		if err := g.add(s); err != nil {
			return err
		}
	}
	return nil
}

// registrySetting creates a registry setting and names it from the baseline
// workbook, falling back to the registry path when the workbook has no entry.
//
// Returns:
//   - baseline.Setting: The setting.
//   - error: The type is not REG_DWORD, REG_SZ or absent, or REG_DWORD data is
//     not a decimal uint32.
//
// Author: Lars Krogh
// Version: 1.0.0
func (g *generator) registrySetting(component, key, value, regType, data string) (baseline.Setting, error) {
	switch regType {
	case baseline.RegDWORD:
		if _, err := strconv.ParseUint(data, 10, 32); err != nil {
			return baseline.Setting{}, fmt.Errorf("%s\\%s: invalid REG_DWORD %q", key, value, data)
		}
	case baseline.RegSZ, baseline.RegAbsent:
	default:
		return baseline.Setting{}, fmt.Errorf("%s\\%s: unsupported registry type %s", key, value, regType)
	}
	desc, ok := g.names.registry[strings.ToLower(key+"!"+value)]
	if !ok {
		desc = fmt.Sprintf("Registry value HKLM\\%s\\%s (no policy name for this value in the baseline workbook)", key, value)
	}
	return baseline.Setting{
		ID:          baseline.RegistryID(key, value),
		Kind:        baseline.KindRegistry,
		Description: desc,
		Component:   component,
		Registry:    &baseline.RegistryValue{Path: key, Name: value, Type: regType, Data: data},
	}, nil
}

// templateSetting converts one security template line from section into a
// setting.
//
// Returns:
//   - baseline.Setting: The user right, account policy, registry or service
//     setting.
//   - error: The section is unsupported or the line is malformed.
//
// Author: Lars Krogh
// Version: 1.0.0
func (g *generator) templateSetting(component, section, line string) (baseline.Setting, error) {
	if section == "Service General Setting" {
		return serviceSetting(component, line)
	}
	k, v, ok := strings.Cut(line, "=")
	if !ok {
		return baseline.Setting{}, fmt.Errorf("malformed security template line %q", line)
	}
	k, v = strings.TrimSpace(k), strings.TrimSpace(v)
	switch section {
	case "Privilege Rights":
		return g.userRight(component, k, v)
	case "System Access":
		name, ok := accountPolicyNames[k]
		if !ok {
			return baseline.Setting{}, fmt.Errorf("unknown [System Access] key %q", k)
		}
		if _, err := strconv.Atoi(v); err != nil {
			return baseline.Setting{}, fmt.Errorf("[System Access] %s: non-integer value %q is not supported", k, v)
		}
		return baseline.Setting{
			ID:            baseline.AccountPolicyID(k),
			Kind:          baseline.KindAccountPolicy,
			Description:   name,
			Component:     component,
			AccountPolicy: &baseline.AccountPolicy{Name: k, Value: v},
		}, nil
	case "Registry Values":
		return g.templateRegistry(component, k, v)
	}
	return baseline.Setting{}, fmt.Errorf("unsupported security template section %q", section)
}

// userRight creates a user right setting from a template value such as
// "*S-1-5-32-544,*S-1-5-11". The SIDs are upper-cased and sorted.
//
// Returns:
//   - baseline.Setting: The setting.
//   - error: The right is unknown, its name is missing from the workbook, or an
//     entry is not a SID.
//
// Author: Lars Krogh
// Version: 1.0.0
func (g *generator) userRight(component, right, value string) (baseline.Setting, error) {
	name, ok := userRightNames[right]
	if !ok {
		return baseline.Setting{}, fmt.Errorf("unknown user right %q", right)
	}
	if !g.names.policies[strings.ToLower(name)] {
		return baseline.Setting{}, fmt.Errorf("user right %s: name %q not found in the baseline workbook", right, name)
	}
	sids := []string{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if !strings.HasPrefix(item, "*S-") {
			return baseline.Setting{}, fmt.Errorf("user right %s: %q is not a SID", right, item)
		}
		sids = append(sids, strings.ToUpper(item[1:]))
	}
	sort.Strings(sids)
	return baseline.Setting{
		ID:          baseline.UserRightID(right),
		Kind:        baseline.KindUserRight,
		Description: "User Rights Assignment > " + name,
		Component:   component,
		UserRight:   &baseline.UserRight{Right: right, SIDs: sids},
	}, nil
}

// serviceSetting creates a service start type setting from a
// [Service General Setting] line of the form "name,start,sddl".
//
// Returns:
//   - baseline.Setting: The setting.
//   - error: The line is malformed, the start type is not 2, 3 or 4, or a
//     security descriptor is set.
//
// Author: Lars Krogh
// Version: 1.0.0
func serviceSetting(component, line string) (baseline.Setting, error) {
	parts := strings.SplitN(line, ",", 3)
	if len(parts) != 3 {
		return baseline.Setting{}, fmt.Errorf("malformed service line %q", line)
	}
	name := strings.Trim(parts[0], `"`)
	start, err := strconv.Atoi(parts[1])
	if err != nil || start < 2 || start > 4 {
		return baseline.Setting{}, fmt.Errorf("service %s: unsupported start type %q", name, parts[1])
	}
	if sddl := strings.Trim(parts[2], `"`); sddl != "" {
		return baseline.Setting{}, fmt.Errorf("service %s: security descriptors are not supported", name)
	}
	return baseline.Setting{
		ID:          baseline.ServiceID(name),
		Kind:        baseline.KindService,
		Description: fmt.Sprintf("System Services > %s startup type", name),
		Component:   component,
		Service:     &baseline.Service{Name: name, StartType: start},
	}, nil
}

// templateRegistry converts a [Registry Values] entry such as
// MACHINE\...\Name=4,1 into a registry setting.
//
// Returns:
//   - baseline.Setting: The setting.
//   - error: The key is not below MACHINE or the type is not 1 (REG_SZ) or
//     4 (REG_DWORD).
//
// Author: Lars Krogh
// Version: 1.0.0
func (g *generator) templateRegistry(component, key, value string) (baseline.Setting, error) {
	if !strings.HasPrefix(strings.ToUpper(key), `MACHINE\`) {
		return baseline.Setting{}, fmt.Errorf("registry value %q is not below MACHINE", key)
	}
	full := key[len(`MACHINE\`):]
	i := strings.LastIndex(full, `\`)
	if i <= 0 {
		return baseline.Setting{}, fmt.Errorf("malformed registry value %q", key)
	}
	typ, data, ok := strings.Cut(value, ",")
	if !ok {
		return baseline.Setting{}, fmt.Errorf("malformed registry data for %q", key)
	}
	var regType string
	switch typ {
	case "4":
		regType = baseline.RegDWORD
	case "1":
		regType = baseline.RegSZ
		data = strings.Trim(data, `"`)
	default:
		return baseline.Setting{}, fmt.Errorf("%s: unsupported security template registry type %s", key, typ)
	}
	return g.registrySetting(component, full[:i], full[i+1:], regType, data)
}

// deltaOverrides reads the DeltaForNonDomainJoined INF and LGPO text files
// named in spec and returns them as settings, sorted by ID. Each one must
// replace a setting already in the baseline.
//
// Returns:
//   - []baseline.Setting: The overrides.
//   - error: A file is missing or uses an unsupported section, or an override
//     does not match a baseline setting.
//
// Author: Lars Krogh
// Version: 1.0.0
func (g *generator) deltaOverrides(p *sctPackage, spec sourceSpec) ([]baseline.Setting, error) {
	const component = "DeltaForNonDomainJoined (Microsoft baseline local install script)"
	var out []baseline.Setting
	infData, err := p.read(spec.DeltaINF)
	if err != nil {
		return nil, err
	}
	for section, lines := range parseINF(decodeText(infData)) {
		switch section {
		case "Unicode", "Version", "Profile Description":
		case "Registry Values":
			if len(lines) > 0 {
				return nil, errors.New("delta [Registry Values] entries are not supported")
			}
		case "Privilege Rights":
			for _, kv := range lines {
				s, err := g.userRight(component, kv[0], kv[1])
				if err != nil {
					return nil, err
				}
				out = append(out, s)
			}
		default:
			return nil, fmt.Errorf("unsupported delta section [%s]", section)
		}
	}
	txtData, err := p.read(spec.DeltaText)
	if err != nil {
		return nil, err
	}
	entries, err := parseLGPOText(decodeText(txtData))
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		s, err := g.registrySetting(component, e.Key, e.Value, e.Type, e.Data)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	for _, s := range out {
		if _, ok := g.byID[strings.ToLower(s.ID)]; !ok {
			return nil, fmt.Errorf("delta setting %s does not override a baseline setting", s.ID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
