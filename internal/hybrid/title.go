package hybrid

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// maxPromptTitleRunes bounds the first-prompt title.
	maxPromptTitleRunes = 80
)

func dropControl(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.ToValidUTF8(s, ""))
}

// PromptTitle is the title for a first prompt: control
// characters dropped, whitespace runs folded to one space, and at most
// maxPromptTitleRunes runes, the last an ellipsis when cut.
func PromptTitle(text string) string {
	s := strings.Join(strings.Fields(dropControl(strings.Map(func(r rune) rune {
		// Line breaks and tabs separate words; strip only other controls.
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, text))), " ")
	if utf8.RuneCountInString(s) <= maxPromptTitleRunes {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:maxPromptTitleRunes-1])) + "…"
}
