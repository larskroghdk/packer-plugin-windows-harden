package main

import (
	"archive/zip"
	"bytes"
	"slices"
	"testing"
)

// zipPart builds an in-memory zip containing one file and returns that file,
// so tests can exercise readers that take a *zip.File.
//
// Author: Lars Krogh
// Version: 1.0.0
func zipPart(t *testing.T, name, content string) *zip.File {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return r.File[0]
}

// TestReadSheet checks shared and inline strings, column gaps and cells without
// a reference.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestReadSheet(t *testing.T) {
	sheet := `<worksheet><sheetData>
<row><c r="A1" t="s"><v>0</v></c><c r="C1" t="inlineStr"><is><t>inline</t></is></c></row>
<row><c t="s"><v>1</v></c><c><v>42</v></c></row>
</sheetData></worksheet>`
	rows, err := readSheet(zipPart(t, "sheet.xml", sheet), []string{"first", "second"})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"first", "", "inline"}, {"second", "42"}}
	if len(rows) != len(want) || !slices.Equal(rows[0], want[0]) || !slices.Equal(rows[1], want[1]) {
		t.Errorf("readSheet = %q, want %q", rows, want)
	}
}

// TestReadSheetRejectsBadSharedStringIndex checks that a shared string index
// outside the table is an error rather than an empty cell.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestReadSheetRejectsBadSharedStringIndex(t *testing.T) {
	sheet := `<worksheet><sheetData><row><c r="A1" t="s"><v>7</v></c></row></sheetData></worksheet>`
	if _, err := readSheet(zipPart(t, "sheet.xml", sheet), []string{"only"}); err == nil {
		t.Error("expected an error for a shared string index outside the table")
	}
}

// TestRegistryLocations checks that both HKLM\...!value and MACHINE\...\value
// forms are extracted and that other hives are ignored.
//
// Author: Lars Krogh
// Version: 1.0.0
func TestRegistryLocations(t *testing.T) {
	cell := "HKLM\\Software\\Policies\\X!Value; MACHINE\\System\\Y\\Name\nHKCU\\Software\\Z!Ignored"
	got := registryLocations(cell)
	want := []string{`software\policies\x!value`, `system\y!name`}
	if !slices.Equal(got, want) {
		t.Errorf("registryLocations = %q, want %q", got, want)
	}
}
