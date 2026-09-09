// Package markdown implements pymax's lightweight Markdown-to-Element
// formatter used when sending message text, a port of pymax's
// formatting.markdown.Formatter.
package markdown

import (
	"strings"
	"unicode/utf16"

	"github.com/SonChegg/PyMax/types"
)

// bmpMax is the highest code point in the Basic Multilingual Plane;
// characters above it are encoded as UTF-16 surrogate pairs (2 code units).
const bmpMax = 0xFFFF

var markerTypes = map[string]string{
	"```": "CODE",
	"**":  "STRONG",
	"__":  "UNDERLINE",
	"~~":  "STRIKETHROUGH",
	"`":   "MONOSPACED",
	"_":   "EMPHASIZED",
	"*":   "EMPHASIZED",
}

// markerOrder must be checked longest-prefix-first so "**" is not
// mistaken for two "*" markers, matching pymax's MARKER_ORDER.
var markerOrder = []string{"```", "**", "__", "~~", "`", "_", "*"}

// Format converts markdown-flavored text into Max's clean-text +
// Element-span representation, a port of pymax's Formatter.format_markdown.
func Format(text string) (string, []types.Element) {
	runes := []rune(text)
	var clean strings.Builder
	var entities []types.Element

	i := 0
	cleanPos := 0
	active := map[string]int{}
	lineStart := true

	codeUnitsLen := func(s string) int {
		n := 0
		for _, r := range s {
			n += len(utf16.Encode([]rune{r}))
		}
		return n
	}

	for i < len(runes) {
		handled := false

		if label, url, next, ok := parseLink(runes, i); ok {
			start := cleanPos
			l := codeUnitsLen(label)
			clean.WriteString(label)
			cleanPos += l
			u := url
			entities = append(entities, types.Element{
				Type:       "LINK",
				From:       intPtr(start),
				Length:     intPtr(l),
				Attributes: &types.ElementAttributes{URL: &u},
			})
			i = next
			lineStart = false
			continue
		}

		if lineStart && runes[i] == '#' {
			startI := i
			for i < len(runes) && runes[i] == '#' {
				i++
			}
			if i < len(runes) && runes[i] == ' ' {
				i++
				start := cleanPos
				for i < len(runes) && runes[i] != '\n' {
					ch := runes[i]
					clean.WriteRune(ch)
					i++
					cleanPos += runeWidth(ch)
				}
				length := cleanPos - start
				if length > 0 {
					entities = append(entities, types.Element{Type: "HEADING", From: intPtr(start), Length: intPtr(length)})
				}
				lineStart = false
				continue
			}
			i = startI
		}

		if lineStart && runes[i] == '>' {
			i++
			if i < len(runes) && runes[i] == ' ' {
				i++
			}
			start := cleanPos
			for i < len(runes) && runes[i] != '\n' {
				ch := runes[i]
				clean.WriteRune(ch)
				i++
				cleanPos += runeWidth(ch)
			}
			length := cleanPos - start
			if length > 0 {
				entities = append(entities, types.Element{Type: "QUOTE", From: intPtr(start), Length: intPtr(length)})
			}
			lineStart = false
			continue
		}

		for _, marker := range markerOrder {
			if !hasPrefixAt(runes, i, marker) {
				continue
			}
			markerLen := len([]rune(marker))

			if _, isActive := active[marker]; !isActive {
				if marker == "```" {
					closingIndex := indexOfFrom(runes, marker, i+markerLen, -1)
					if closingIndex == -1 || closingIndex == i+markerLen {
						clean.WriteString(marker)
						cleanPos += markerLen
						i += markerLen
						handled = true
						break
					}
					active[marker] = cleanPos
					i += markerLen

					lineEnd := indexOfRune(runes, '\n', i)
					if lineEnd != -1 && lineEnd < closingIndex {
						i = lineEnd + 1
					}
					handled = true
					break
				}

				end := indexOfRune(runes, '\n', i+markerLen)
				closingIndex := indexOfFrom(runes, marker, i+markerLen, end)
				if closingIndex == -1 || closingIndex == i+markerLen {
					clean.WriteString(marker)
					cleanPos += markerLen
					i += markerLen
					handled = true
					break
				}

				active[marker] = cleanPos
				i += markerLen
				handled = true
				break
			}

			start := active[marker]
			length := cleanPos - start
			if length > 0 {
				entities = append(entities, types.Element{Type: markerTypes[marker], From: intPtr(start), Length: intPtr(length)})
			}
			delete(active, marker)
			i += markerLen
			handled = true
			break
		}

		if handled {
			lineStart = false
			continue
		}

		ch := runes[i]
		clean.WriteRune(ch)
		lineStart = ch == '\n'
		i++
		cleanPos += runeWidth(ch)
	}

	return clean.String(), entities
}

func runeWidth(r rune) int {
	if r > bmpMax {
		return 2
	}
	return 1
}

func intPtr(v int) *int { return &v }

func hasPrefixAt(runes []rune, i int, marker string) bool {
	m := []rune(marker)
	if i+len(m) > len(runes) {
		return false
	}
	for j, r := range m {
		if runes[i+j] != r {
			return false
		}
	}
	return true
}

// indexOfFrom finds the next occurrence of marker in runes[from:end)
// (end == -1 means "to the end"), returning -1 if not found.
func indexOfFrom(runes []rune, marker string, from, end int) int {
	m := []rune(marker)
	limit := len(runes) - len(m)
	if end != -1 {
		limit = end - len(m)
	}
	for i := from; i <= limit; i++ {
		match := true
		for j, r := range m {
			if runes[i+j] != r {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func indexOfRune(runes []rune, r rune, from int) int {
	for i := from; i < len(runes); i++ {
		if runes[i] == r {
			return i
		}
	}
	return -1
}

// parseLink recognizes a "[label](url)" span starting at i, a port of
// pymax's Formatter._parse_link.
func parseLink(runes []rune, i int) (label, url string, next int, ok bool) {
	if i >= len(runes) || runes[i] != '[' {
		return "", "", 0, false
	}

	labelEnd := indexOfRune(runes, ']', i+1)
	if labelEnd == -1 {
		return "", "", 0, false
	}
	if labelEnd+1 >= len(runes) || runes[labelEnd+1] != '(' {
		return "", "", 0, false
	}

	urlStart := labelEnd + 2
	urlEnd := indexOfRune(runes, ')', urlStart)
	if urlEnd == -1 {
		return "", "", 0, false
	}

	label = string(runes[i+1 : labelEnd])
	url = string(runes[urlStart:urlEnd])
	if label == "" || url == "" {
		return "", "", 0, false
	}

	return label, url, urlEnd + 1, true
}
