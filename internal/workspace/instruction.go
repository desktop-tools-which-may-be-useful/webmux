package workspace

import (
	"regexp"
	"strings"
)

// artifactsDirPlaceholder stands for the session's interaction artifacts
// folder in an expanded instruction; it is replaced once the session exists.
const artifactsDirPlaceholder = "{{" + artifactsDirName + "}}"

// webmuxPlaceholder stands for the session's WebMux brief (the shell guide
// address, executable, socket and artifacts folder); it is replaced once
// the session exists.
const webmuxPlaceholder = "{{" + webmuxReferenceName + "}}"

// userReference is the built-in {{user}}: where the user's own message goes
// in an expanded instruction. {{dir}} is the artifacts folder and
// {{webmux}} the brief above. A quick command cannot be named any of them.
const (
	artifactsDirName    = "dir"
	userReferenceName   = "user"
	userReference       = "{{" + userReferenceName + "}}"
	webmuxReferenceName = "webmux"
)

// QuickCommand is a saved quick command: an initialization instruction
// inserts its Text where it writes {{Name}}.
type QuickCommand struct {
	Name string
	Text string
}

// instructionReference matches {{name}}; the name may not contain braces or
// line breaks.
var instructionReference = regexp.MustCompile(`\{\{([^{}\n]+)\}\}`)

// expandInstruction replaces each {{name}} of template with the text of the
// quick command called name. Replacement is one pass: the inserted text is
// not scanned again. {{user}}, {{dir}} and {{webmux}} stay in the text for the
// caller to fill. The names no command has are returned in order of first
// appearance and their references are left as written.
func expandInstruction(template string, commands []QuickCommand) (string, []string) {
	byName := make(map[string]string, len(commands))
	for _, c := range commands {
		byName[c.Name] = c.Text
	}
	var missing []string
	seen := map[string]bool{}
	text := instructionReference.ReplaceAllStringFunc(template, func(ref string) string {
		name := strings.TrimSpace(ref[2 : len(ref)-2])
		if name == userReferenceName {
			return userReference
		}
		if name == artifactsDirName {
			return artifactsDirPlaceholder
		}
		if name == webmuxReferenceName {
			return webmuxPlaceholder
		}
		if replacement, ok := byName[name]; ok {
			return replacement
		}
		if !seen[name] {
			seen[name] = true
			missing = append(missing, name)
		}
		return ref
	})
	return text, missing
}

// fillUser puts the user's message where instruction has {{user}}. Without
// one, the message follows the instruction after a blank line. The message
// is inserted last, so nothing in it is read as a reference.
func fillUser(instruction, user string) string {
	if !strings.Contains(instruction, userReference) {
		return instruction + "\n\n" + user
	}
	return strings.ReplaceAll(instruction, userReference, user)
}

// promptSize is the byte length fillUser would produce.
func promptSize(instruction, user string) int {
	n := strings.Count(instruction, userReference)
	if n == 0 {
		return len(instruction) + 2 + len(user)
	}
	return len(instruction) - n*len(userReference) + n*len(user)
}
