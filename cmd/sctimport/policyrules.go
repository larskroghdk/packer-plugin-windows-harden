package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type policyRules struct {
	ComputerConfig   []registryRule         `xml:"ComputerConfig"`
	UserConfig       []registryRule         `xml:"UserConfig"`
	SecurityTemplate []securityTemplateRule `xml:"SecurityTemplate"`
	AuditSubcategory []auditRule            `xml:"AuditSubcategory"`
}

type registryRule struct {
	Key        string `xml:"Key"`
	Value      string `xml:"Value"`
	RegType    string `xml:"RegType"`
	RegData    string `xml:"RegData"`
	PolicyName string `xml:"PolicyName"`
}

type securityTemplateRule struct {
	Section    string `xml:"Section,attr"`
	LineItem   string `xml:"LineItem"`
	PolicyName string `xml:"PolicyName"`
}

type auditRule struct {
	GUID       string `xml:"GUID"`
	Name       string `xml:"Name"`
	Setting    int    `xml:"Setting"`
	PolicyName string `xml:"PolicyName"`
}

// parsePolicyRules decodes a PolicyAnalyzer .PolicyRules file, which may be
// UTF-16 encoded.
//
// Returns:
//   - *policyRules: The decoded rules.
//   - error: The XML is invalid.
//
// Author: Lars Krogh
// Version: 1.0.0
func parsePolicyRules(data []byte) (*policyRules, error) {
	var rules policyRules
	dec := xml.NewDecoder(strings.NewReader(decodeText(data)))
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	if err := dec.Decode(&rules); err != nil {
		return nil, fmt.Errorf("decode PolicyRules: %w", err)
	}
	return &rules, nil
}

// components returns the names of all GPO components that contribute a rule,
// including User Configuration components.
//
// Author: Lars Krogh
// Version: 1.0.0
func (rules *policyRules) components() map[string]bool {
	out := map[string]bool{}
	for _, e := range rules.ComputerConfig {
		out[e.PolicyName] = true
	}
	for _, e := range rules.UserConfig {
		out[e.PolicyName] = true
	}
	for _, e := range rules.SecurityTemplate {
		out[e.PolicyName] = true
	}
	for _, e := range rules.AuditSubcategory {

		out[e.PolicyName] = true
	}
	return out
}
