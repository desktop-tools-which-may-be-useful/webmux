package main

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
)

type markdownHeading struct {
	Level  int
	Text   string
	Anchor string
}

type markdownDocument struct {
	Headings      []markdownHeading
	FeatureIDs    []string
	FeatureStatus map[string][]string
	Visible       []byte
}

func parseMarkdown(data []byte) markdownDocument {
	doc := markdownDocument{FeatureStatus: map[string][]string{}}
	anchorCounts := map[string]int{}
	inFence := false
	fenceChar := byte(0)
	fenceLen := 0
	inComment := false
	featuresSection := false
	catalogStarted := false
	catalogDone := false
	setextCandidate := ""
	var visible strings.Builder
	addHeading := func(level int, text string) {
		base := markdownSlug(text)
		anchor := base
		if count := anchorCounts[base]; count != 0 {
			anchor = fmt.Sprintf("%s-%d", base, count)
		}
		anchorCounts[base]++
		doc.Headings = append(doc.Headings, markdownHeading{Level: level, Text: text, Anchor: anchor})
		if level == 2 {
			featuresSection = text == "Features"
			if catalogStarted {
				catalogDone = true
			}
		}
	}

	for _, raw := range strings.Split(string(data), "\n") {
		line := raw
		trimmed := strings.TrimSpace(line)
		if inFence {
			setextCandidate = ""
			if run := fenceRun(trimmed, fenceChar); run >= fenceLen {
				inFence = false
			}
			continue
		}
		line = stripHTMLComments(line, &inComment)
		if strings.TrimSpace(line) == "" {
			setextCandidate = ""
			visible.WriteByte('\n')
			if catalogStarted {
				catalogDone = true
			}
			continue
		}
		if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
			setextCandidate = ""
			continue
		}
		trimmed = strings.TrimSpace(line)
		if run, char := openingFence(trimmed); run != 0 {
			setextCandidate = ""
			inFence, fenceChar, fenceLen = true, char, run
			continue
		}
		visible.WriteString(line)
		visible.WriteByte('\n')

		if level := markdownSetextLevel(trimmed); level != 0 && setextCandidate != "" {
			addHeading(level, markdownPlainText(setextCandidate))
			setextCandidate = ""
			continue
		}
		if level, text, ok := markdownHeadingLine(trimmed); ok {
			addHeading(level, text)
			setextCandidate = ""
			continue
		}

		if !featuresSection || catalogDone || !strings.HasPrefix(trimmed, "|") {
			setextCandidate = markdownSetextCandidate(trimmed)
			continue
		}
		setextCandidate = ""
		cells := markdownTableCells(trimmed)
		if len(cells) < 3 {
			continue
		}
		if !catalogStarted {
			if strings.EqualFold(markdownPlainText(cells[0]), "ID") {
				catalogStarted = true
			}
			continue
		}
		if tableSeparator(cells) {
			continue
		}
		id := markdownPlainText(cells[0])
		status := markdownPlainText(cells[2])
		if strings.HasPrefix(id, "WM-DS-") {
			doc.FeatureIDs = append(doc.FeatureIDs, id)
			doc.FeatureStatus[id] = append(doc.FeatureStatus[id], status)
		}
	}
	doc.Visible = []byte(visible.String())
	return doc
}

func openingFence(line string) (int, byte) {
	for _, char := range []byte{'`', '~'} {
		if run := fenceRun(line, char); run >= 3 {
			return run, char
		}
	}
	return 0, 0
}

func fenceRun(line string, char byte) int {
	count := 0
	for count < len(line) && line[count] == char {
		count++
	}
	return count
}

func stripHTMLComments(line string, inComment *bool) string {
	var visible strings.Builder
	for line != "" {
		if *inComment {
			end := strings.Index(line, "-->")
			if end < 0 {
				return visible.String()
			}
			line = line[end+3:]
			*inComment = false
			continue
		}
		start := strings.Index(line, "<!--")
		if start < 0 {
			visible.WriteString(line)
			break
		}
		visible.WriteString(line[:start])
		line = line[start+4:]
		*inComment = true
	}
	return visible.String()
}

func markdownSetextLevel(line string) int {
	if line == "" || (line[0] != '=' && line[0] != '-') {
		return 0
	}
	for i := 1; i < len(line); i++ {
		if line[i] != line[0] {
			return 0
		}
	}
	if line[0] == '=' {
		return 1
	}
	return 2
}

func markdownSetextCandidate(line string) string {
	if line == "" || strings.HasPrefix(line, "|") {
		return ""
	}
	return line
}

func markdownHeadingLine(line string) (int, string, bool) {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level >= len(line) || line[level] != ' ' {
		return 0, "", false
	}
	text := strings.TrimSpace(line[level+1:])
	text = strings.TrimSpace(strings.TrimRight(text, "#"))
	return level, markdownPlainText(text), text != ""
}

func markdownTableCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func markdownPlainText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.Trim(text, "`*_~ ")
	return strings.TrimSpace(text)
}

func tableSeparator(cells []string) bool {
	for _, cell := range cells {
		cell = strings.Trim(cell, ": ")
		if len(cell) < 3 || strings.Trim(cell, "-") != "" {
			return false
		}
	}
	return true
}

func markdownFeatureHeadings(data []byte) []string {
	var headings []string
	for _, heading := range parseMarkdown(data).Headings {
		if heading.Level == 2 && strings.HasPrefix(heading.Text, "WM-DS-") {
			headings = append(headings, heading.Text)
		}
	}
	return headings
}

func markdownFeatureTableIDs(data []byte) []string {
	return parseMarkdown(data).FeatureIDs
}

func markdownFeatureStatuses(data []byte) map[string][]string {
	return parseMarkdown(data).FeatureStatus
}

func markdownExactHeadingAnchors(data []byte, want string) []string {
	var anchors []string
	for _, heading := range parseMarkdown(data).Headings {
		if heading.Level == 2 && heading.Text == want {
			anchors = append(anchors, heading.Anchor)
		}
	}
	return anchors
}

func markdownExactHeadingCount(data []byte, want string) int {
	return len(markdownExactHeadingAnchors(data, want))
}

func markdownHasAnchor(data []byte, want string) bool {
	for _, heading := range parseMarkdown(data).Headings {
		if heading.Anchor == want {
			return true
		}
	}
	return false
}

func markdownMarkerCount(data []byte, marker string) int {
	return bytes.Count(parseMarkdown(data).Visible, []byte(marker))
}

func markdownSlug(heading string) string {
	var builder strings.Builder
	lastSpace := false
	for _, r := range strings.ToLower(heading) {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r), r == '-', r == '_':
			builder.WriteRune(r)
			lastSpace = false
		case unicode.IsSpace(r):
			if builder.Len() != 0 && !lastSpace {
				builder.WriteByte('-')
				lastSpace = true
			}
		}
	}
	return strings.TrimSuffix(builder.String(), "-")
}
