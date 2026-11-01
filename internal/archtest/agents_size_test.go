package archtest

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// agentsMaxBytes is the AGENTS.md budget. The file is loaded into every agent
// session, so each byte is paid for on every task; rules needed only
// occasionally belong in docs/agent-rules/ instead.
const agentsMaxBytes = 10 << 10

const agentsGuide = "AGENTS.md"

// TestAgentsGuideSize fails when AGENTS.md exceeds its budget. The failure text
// is addressed to the next agent: it states the measured size and the required
// reorganization.
func TestAgentsGuideSize(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", agentsGuide))
	if err != nil {
		t.Fatal(err)
	}
	if msg := agentsSizeViolation(len(data)); msg != "" {
		t.Error(msg)
	}
}

// agentsSizeViolation returns the instruction for an AGENTS.md of the given
// size, or "" when it fits the budget.
func agentsSizeViolation(size int) string {
	if size <= agentsMaxBytes {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s is %s (%d bytes), over its %s budget by %d bytes.\n\n",
		agentsGuide, kib(size), size, kib(agentsMaxBytes), size-agentsMaxBytes)
	b.WriteString("It is loaded into every agent session, so every byte costs on every task.\n")
	b.WriteString("Move the rules that are needed least often down into the docs:\n\n")
	b.WriteString("  1. Pick the least frequently needed sections or bullets: reference tables, per-area detail,\n")
	b.WriteString("     CI layout, rarely used commands. Keep what almost every change needs\n")
	b.WriteString("     (architecture invariants, security rules, core commands, conventions).\n")
	b.WriteString("  2. Move the full text into docs/agent-rules/<topic>.md (create the page when none fits)\n")
	b.WriteString("     and list it in docs/agent-rules.md. Keep the rule's wording; do not paraphrase it away.\n")
	fmt.Fprintf(&b, "  3. Leave a one-line summary and a link in %s so the rule can still be found.\n", agentsGuide)
	b.WriteString("  4. New docs must satisfy docs/doc-governance.md (20-400 lines, at most 300 display columns\n")
	b.WriteString("     per line, at most 48 KiB, at most 20 child directories); run `make docs-check`.\n")
	b.WriteString("  5. Update any links, docs/user-stories* references and `make format-check` output the move affects.\n\n")
	b.WriteString("Do not satisfy this test by deleting rules, shortening them until they lose meaning,\n")
	b.WriteString("or raising agentsMaxBytes in internal/archtest/agents_size_test.go.\n")
	return b.String()
}

func kib(n int) string {
	return fmt.Sprintf("%.1f KiB", float64(n)/1024)
}

// TestAgentsSizeViolationMessage proves the gate: it passes at the budget, and
// one byte over it produces the instruction agents must act on.
func TestAgentsSizeViolationMessage(t *testing.T) {
	if msg := agentsSizeViolation(agentsMaxBytes); msg != "" {
		t.Fatalf("at the budget: unexpected violation %q", msg)
	}
	msg := agentsSizeViolation(agentsMaxBytes + 1)
	for _, want := range []string{
		"over its 10.0 KiB budget by 1 bytes",
		"docs/agent-rules/<topic>.md",
		"docs/agent-rules.md",
		"make docs-check",
		"do not paraphrase",
		"raising agentsMaxBytes",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("violation message lacks %q:\n%s", want, msg)
		}
	}
}

// TestAgentsRulePagesExist keeps the sunk-rule index honest: every page that
// docs/agent-rules.md links to must exist.
func TestAgentsRulePagesExist(t *testing.T) {
	root := filepath.Join("..", "..")
	index, err := os.ReadFile(filepath.Join(root, "docs", "agent-rules.md"))
	if err != nil {
		t.Fatal(err)
	}
	var found int
	for _, line := range strings.Split(string(index), "\n") {
		_, rest, ok := strings.Cut(line, "](agent-rules/")
		if !ok {
			continue
		}
		page, _, _ := strings.Cut(rest, ")")
		found++
		if _, err := os.Stat(filepath.Join(root, "docs", "agent-rules", page)); errors.Is(err, fs.ErrNotExist) {
			t.Errorf("docs/agent-rules.md links to missing page agent-rules/%s", page)
		}
	}
	if found == 0 {
		t.Error("docs/agent-rules.md lists no pages under agent-rules/")
	}
}
