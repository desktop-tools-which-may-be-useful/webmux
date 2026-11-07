//go:build linux || darwin

package procpeek

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRunProcessesIncludesRootAndDirectChildren(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 30 & wait")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	var facts []ProcessFacts
	deadline := time.Now().Add(time.Second)
	for {
		facts = RunProcesses(cmd.Process.Pid)
		if len(facts) == 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(facts) != 2 || facts[0].PID != cmd.Process.Pid || (facts[0].Executable != "sh" && facts[0].Executable != "dash" && facts[0].Executable != "bash") || facts[1].Executable != "sleep" {
		t.Fatalf("RunProcesses = %+v", facts)
	}
	for _, process := range facts {
		if process.Cwd == "" || process.Start == "" || process.Domain == "" {
			t.Fatalf("incomplete process facts: %+v", process)
		}
		if runtime.GOOS == "linux" && !strings.HasPrefix(process.Domain, "linux:") {
			t.Fatalf("Linux domain = %q", process.Domain)
		}
	}
}

func TestLookupProcessRejectsExitedPID(t *testing.T) {
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if facts, ok := LookupProcess(cmd.Process.Pid); ok {
		t.Fatalf("exited process returned %+v", facts)
	}
	if facts, ok := LookupProcess(0); ok || facts != (ProcessFacts{}) {
		t.Fatalf("invalid PID returned %+v %v", facts, ok)
	}
}

func TestLinuxProcessStartIsDecimalTicks(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux stat format")
	}
	facts, ok := LookupProcess(1)
	if !ok {
		t.Skip("PID 1 is not readable in this environment")
	}
	if _, err := strconv.ParseUint(facts.Start, 10, 64); err != nil {
		t.Fatalf("start = %q: %v", facts.Start, err)
	}
	if strings.Count(facts.Domain, ":") < 3 || !strings.Contains(facts.Domain, ":pid:[") {
		t.Fatalf("domain = %q", facts.Domain)
	}
}
