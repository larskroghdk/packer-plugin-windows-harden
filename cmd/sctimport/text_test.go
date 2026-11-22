package main

import (
	"strings"
	"testing"
)

// TestTextParsersRejectTruncatedInput prevents scanner errors from silently
// discarding baseline delta settings after an oversized line.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestTextParsersRejectTruncatedInput(t *testing.T) {
	long := strings.Repeat("x", 70_000)
	if _, err := parseINF("[Privilege Rights]\nSeNetworkLogonRight=*S-1-5-32-544\n" + long); err == nil {
		t.Fatal("INF scanner failure was ignored")
	}
	if _, err := parseLGPOText("Computer\nSoftware\\X\nValue\nDWORD:1\n;" + long); err == nil {
		t.Fatal("LGPO scanner failure was ignored")
	}
	if _, err := parseINF("[Privilege Rights]\nmalformed"); err == nil {
		t.Fatal("malformed INF line was ignored")
	}
	sections, err := parseINF("; comment\n[Privilege Rights]\nSeNetworkLogonRight = *S-1-5-32-544\n")
	if err != nil || sections["Privilege Rights"][0][1] != "*S-1-5-32-544" {
		t.Fatalf("valid INF = %v, %v", sections, err)
	}
}
