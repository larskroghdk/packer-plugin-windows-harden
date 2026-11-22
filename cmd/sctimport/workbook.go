package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"
)

type workbookNames struct {
	registry map[string]string
	policies map[string]bool
	audit    map[string]string
}

// readWorkbook reads the baseline .xlsx workbook and indexes the policy names
// by registry location, by setting name and by audit subcategory, so that
// generated settings get the names Microsoft uses.
//
// Returns:
//   - *workbookNames: The name indexes.
//   - error: The workbook or one of its sheets cannot be read.
//
// Author: Lars Krogh
// Version: 1.1.0
func readWorkbook(data []byte) (*workbookNames, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open workbook: %w", err)
	}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}
	shared, err := readSharedStrings(files["xl/sharedStrings.xml"])
	if err != nil {
		return nil, err
	}
	sheets, err := readSheetIndex(files)
	if err != nil {
		return nil, err
	}
	names := &workbookNames{registry: map[string]string{}, policies: map[string]bool{}, audit: map[string]string{}}
	for _, sheetName := range slices.Sorted(maps.Keys(sheets)) {
		target := sheets[sheetName]
		rows, err := readSheet(files[target], shared)
		if err != nil {
			return nil, fmt.Errorf("sheet %s: %w", sheetName, err)
		}
		if len(rows) == 0 {
			continue
		}
		header := rows[0]
		col := func(name string) int {
			for i, h := range header {
				if strings.EqualFold(strings.TrimSpace(h), name) {
					return i
				}
			}
			return -1
		}
		pathCol, nameCol, regCol := col("Policy Path"), col("Policy Setting Name"), col("Registry Information")
		if pathCol < 0 || nameCol < 0 {
			continue
		}
		isAudit := strings.HasPrefix(strings.ToLower(sheetName), "advanced audit")
		for _, row := range rows[1:] {
			get := func(i int) string {
				if i < 0 || i >= len(row) {
					return ""
				}
				return strings.TrimSpace(row[i])
			}
			policyPath, settingName := get(pathCol), get(nameCol)
			if settingName == "" {
				continue
			}
			label := settingName
			if policyPath != "" {
				label = policyPath + " > " + settingName
			}
			names.policies[strings.ToLower(settingName)] = true
			if isAudit {
				names.audit[strings.ToLower(settingName)] = label
			}
			for _, loc := range registryLocations(get(regCol)) {
				if _, dup := names.registry[loc]; !dup {
					names.registry[loc] = label
				}
			}
		}
	}
	return names, nil
}

// registryLocations extracts the HKLM registry locations from a workbook
// "Registry Information" cell and returns them as lower-case "path!value"
// keys. Locations in other hives are ignored.
//
// Author: Lars Krogh
// Version: 1.0.0
func registryLocations(cell string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(cell, func(r rune) bool { return r == ';' || r == '\n' || r == '\r' }) {
		part = strings.TrimSpace(part)
		lower := strings.ToLower(part)
		switch {
		case strings.HasPrefix(lower, `hklm\`) && strings.Contains(part, "!"):
			out = append(out, strings.ToLower(part[len(`hklm\`):]))
		case strings.HasPrefix(lower, `machine\`):
			rest := part[len(`machine\`):]
			if i := strings.LastIndex(rest, `\`); i > 0 {
				out = append(out, strings.ToLower(rest[:i]+"!"+rest[i+1:]))
			}
		}
	}
	return out
}

type sharedStringTable struct {
	Items []struct {
		T    string `xml:"t"`
		Runs []struct {
			T string `xml:"t"`
		} `xml:"r"`
	} `xml:"si"`
}

// readSharedStrings returns the workbook's shared string table, joining rich
// text runs. A workbook without the table has no shared strings.
//
// Returns:
//   - []string: The strings, indexed as referenced by cells.
//   - error: The table cannot be decoded.
//
// Author: Lars Krogh
// Version: 1.0.0
func readSharedStrings(f *zip.File) ([]string, error) {
	if f == nil {
		return nil, nil
	}
	var table sharedStringTable
	if err := decodeZipXML(f, &table); err != nil {
		return nil, fmt.Errorf("shared strings: %w", err)
	}
	out := make([]string, len(table.Items))
	for i, item := range table.Items {
		if len(item.Runs) == 0 {
			out[i] = item.T
			continue
		}
		var b strings.Builder
		for _, run := range item.Runs {
			b.WriteString(run.T)
		}
		out[i] = b.String()
	}
	return out, nil
}

// readSheetIndex maps each sheet name to its part path in the zip, resolved
// through the workbook relationships.
//
// Returns:
//   - map[string]string: Sheet name to part path, for sheets whose part exists.
//   - error: workbook.xml or its relationships cannot be decoded.
//
// Author: Lars Krogh
// Version: 1.0.0
func readSheetIndex(files map[string]*zip.File) (map[string]string, error) {
	var wb struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := decodeZipXML(files["xl/workbook.xml"], &wb); err != nil {
		return nil, fmt.Errorf("workbook.xml: %w", err)
	}
	var rels struct {
		Rel []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := decodeZipXML(files["xl/_rels/workbook.xml.rels"], &rels); err != nil {
		return nil, fmt.Errorf("workbook rels: %w", err)
	}
	targets := map[string]string{}
	for _, r := range rels.Rel {
		t := strings.TrimPrefix(r.Target, "/")
		if !strings.HasPrefix(t, "xl/") {
			t = path.Join("xl", t)
		}
		targets[r.ID] = t
	}
	out := map[string]string{}
	for _, s := range wb.Sheets {
		if t, ok := targets[s.RID]; ok && files[t] != nil {
			out[s.Name] = t
		}
	}
	return out, nil
}

type xlsxSheet struct {
	Rows []struct {
		Cells []struct {
			Ref    string `xml:"r,attr"`
			Type   string `xml:"t,attr"`
			Value  string `xml:"v"`
			Inline string `xml:"is>t"`
		} `xml:"c"`
	} `xml:"sheetData>row"`
}

// readSheet returns the cell text of every row in a worksheet. Cells are placed
// by their reference, or after the previous cell when the reference is
// omitted. Shared strings are resolved through shared.
//
// Returns:
//   - [][]string: One slice per row, padded with empty strings for skipped
//     columns.
//   - error: The sheet cannot be decoded or a cell references a shared string
//     outside the table.
//
// Author: Lars Krogh
// Version: 1.0.0
func readSheet(f *zip.File, shared []string) ([][]string, error) {
	var sheet xlsxSheet
	if err := decodeZipXML(f, &sheet); err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(sheet.Rows))
	for _, r := range sheet.Rows {
		var row []string
		for _, c := range r.Cells {
			column := columnIndex(c.Ref)
			if column < 0 {
				column = len(row)
			}
			for len(row) <= column {
				row = append(row, "")
			}
			switch c.Type {
			case "s":
				n, err := strconv.Atoi(c.Value)
				if err != nil || n < 0 || n >= len(shared) {
					return nil, fmt.Errorf("cell %s: invalid shared string index %q", c.Ref, c.Value)
				}
				row[column] = shared[n]
			case "inlineStr":
				row[column] = c.Inline
			default:
				row[column] = c.Value
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// columnIndex returns the zero-based column of a cell reference such as "C12",
// or -1 when ref has no column letters.
//
// Author: Lars Krogh
// Version: 1.0.0
func columnIndex(ref string) int {
	n := 0
	for _, r := range ref {
		if r < 'A' || r > 'Z' {
			break
		}
		n = n*26 + int(r-'A'+1)
	}
	return n - 1
}

// decodeZipXML decodes the XML part f into v.
//
// Returns:
//   - error: f is nil, cannot be opened, or is not valid XML for v.
//
// Author: Lars Krogh
// Version: 1.0.0
func decodeZipXML(f *zip.File, v any) error {
	if f == nil {
		return errors.New("missing part")
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	return xml.NewDecoder(rc).Decode(v)

}
