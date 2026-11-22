package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"unicode/utf16"

	"github.com/larskroghdk/packer-plugin-windows-harden/internal/baseline"
)

// decodeText converts data to a string, honouring a UTF-16 LE, UTF-16 BE or
// UTF-8 byte order mark and assuming UTF-8 otherwise.
//
// Author: Lars Krogh
// Version: 1.0.0
func decodeText(data []byte) string {
	switch {
	case len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE:
		return decodeUTF16(data[2:], binary.LittleEndian)
	case len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF:
		return decodeUTF16(data[2:], binary.BigEndian)
	case len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF:
		return string(data[3:])
	}
	return string(data)
}

// decodeUTF16 decodes UTF-16 data without a byte order mark. A trailing odd
// byte is ignored.
//
// Author: Lars Krogh
// Version: 1.0.0
func decodeUTF16(data []byte, order binary.ByteOrder) string {
	u := make([]uint16, len(data)/2)
	for i := range u {
		u[i] = order.Uint16(data[i*2:])
	}
	return string(utf16.Decode(u))
}

// parseINF splits INF text into sections of key/value pairs, skipping blank
// lines and ';' comments. Sections without entries are kept with no pairs.
//
// Returns:
//   - map[string][][2]string: The parsed sections.
//   - error: A line is malformed or exceeds the scanner limit.
//
// Author: Lars Krogh
// Version: 1.1.0
func parseINF(text string) (map[string][][2]string, error) {
	sections := map[string][][2]string{}
	current := ""
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current = line[1 : len(line)-1]
			if _, ok := sections[current]; !ok {
				sections[current] = nil
			}
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || current == "" || strings.TrimSpace(k) == "" {
			return nil, fmt.Errorf("malformed INF entry %q", line)
		}
		sections[current] = append(sections[current], [2]string{strings.TrimSpace(k), strings.TrimSpace(v)})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read INF: %w", err)
	}
	return sections, nil
}

type lgpoRegistryEntry struct {
	Key, Value, Type, Data string
}

// parseLGPOText parses LGPO.exe text output, which is a sequence of four-line
// Computer blocks: scope, key, value name and TYPE:data.
//
// Returns:
//   - []lgpoRegistryEntry: The registry entries in file order.
//   - error: A block is not a Computer block, is truncated, or uses a type
//     other than DWORD or SZ.
//
// Author: Lars Krogh
// Version: 1.1.0
func parseLGPOText(text string) ([]lgpoRegistryEntry, error) {
	var lines []string
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, ";") {
			continue
		}
		lines = append(lines, line)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read LGPO text: %w", err)
	}
	var out []lgpoRegistryEntry
	for i := 0; i < len(lines); i++ {
		if lines[i] == "" {
			continue
		}
		if !strings.EqualFold(lines[i], "Computer") {
			return nil, fmt.Errorf("unsupported LGPO text block starting with %q", lines[i])
		}
		if i+3 >= len(lines) {
			return nil, errors.New("truncated LGPO text block")
		}
		key, value, action := lines[i+1], lines[i+2], lines[i+3]
		i += 3
		typ, data, ok := strings.Cut(action, ":")
		if !ok {
			return nil, fmt.Errorf("unsupported LGPO action %q", action)
		}
		var regType string
		switch strings.ToUpper(typ) {
		case "DWORD":
			regType = baseline.RegDWORD
		case "SZ":
			regType = baseline.RegSZ
		default:
			return nil, fmt.Errorf("unsupported LGPO action %q", action)
		}
		out = append(out, lgpoRegistryEntry{Key: key, Value: value, Type: regType, Data: data})

	}
	return out, nil
}
