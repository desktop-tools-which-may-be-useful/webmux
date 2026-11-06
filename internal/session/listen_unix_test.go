//go:build linux || darwin

package session

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// listenDeadline covers the 2-second scan cadence over a process table that
// may be a second old, plus scheduling slack under -race.
const listenDeadline = 4 * time.Second

func TestSessionListeningPorts(t *testing.T) {
	if os.Getenv("WEBMUX_TEST_LISTEN_HELPER") == "1" {
		runListenHelper(t)
		return
	}
	t.Parallel()
	excluded := freePort(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(Options{DefaultCwd: t.TempDir(), ExcludePorts: []int{excluded}})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	})
	sub := m.Subscribe()
	defer m.Unsubscribe(sub)
	// The helper is a grandchild: bash outlives it so the session keeps
	// running after the listener stops.
	script := `"$@"; printf 'HELPER_EXITED\n'; exec sleep 60`
	s, err := m.Create(CreateInput{
		Cmd: []string{"/bin/bash", "--noprofile", "--norc", "-c", script, "listen", executable, "-test.run=^TestSessionListeningPorts$"},
		Env: map[string]string{"WEBMUX_TEST_LISTEN_HELPER": "1", "WEBMUX_TEST_EXCLUDED_PORT": strconv.Itoa(excluded)},
	})
	if err != nil {
		t.Fatal(err)
	}
	port, helper := helperListening(t, s)
	want := []ListeningPort{{Port: port, Process: filepath.Base(executable)}}
	waitListeningPorts(t, sub, s.Info().ID, excluded, func(ports []ListeningPort) bool { return slices.Equal(ports, want) })
	if err := syscall.Kill(helper, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	waitListeningPorts(t, sub, s.Info().ID, excluded, func(ports []ListeningPort) bool { return len(ports) == 0 })
	waitForOutput(t, s, "HELPER_EXITED")
	if info := s.Info(); info.ExitCode != nil || len(info.ListeningPorts) != 0 {
		t.Fatalf("session after listener stopped: %+v", info)
	}
}

func TestShellSidecarListeningPorts(t *testing.T) {
	if os.Getenv("WEBMUX_TEST_LISTEN_HELPER") == "1" {
		runListenHelper(t)
		return
	}
	t.Parallel()
	excluded := freePort(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(Options{DefaultCwd: t.TempDir(), ExcludePorts: []int{excluded}})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		m.Shutdown(ctx)
	})
	sub := m.Subscribe()
	defer m.Unsubscribe(sub)
	s, err := m.CreateShell(ShellInput{
		CreateInput: CreateInput{
			Cmd: []string{"omp"},
			Env: map[string]string{"WEBMUX_TEST_LISTEN_HELPER": "1", "WEBMUX_TEST_EXCLUDED_PORT": strconv.Itoa(excluded)},
		},
		Overlay: &Overlay{Hybrid: &HybridInfo{Side: "acp", State: "active", Agent: "omp", AgentTitle: "omp"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	script := `"$@" >&2; printf 'HELPER_EXITED\n' >&2; exec sleep 60`
	sidecar, err := s.StartSidecar([]string{"/bin/bash", "--noprofile", "--norc", "-c", script, "listen", executable, "-test.run=^TestShellSidecarListeningPorts$"}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = sidecar.Stop(ctx)
	})
	port, helper := sidecarHelperListening(t, sidecar)
	want := []ListeningPort{{Port: port, Process: filepath.Base(executable)}}
	waitListeningPorts(t, sub, s.Info().ID, excluded, func(ports []ListeningPort) bool { return slices.Equal(ports, want) })
	if err := syscall.Kill(helper, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	waitListeningPorts(t, sub, s.Info().ID, excluded, func(ports []ListeningPort) bool { return len(ports) == 0 })
}

func sidecarHelperListening(t *testing.T, c *Sidecar) (port, pid int) {
	t.Helper()
	pattern := regexp.MustCompile(`LISTENING (\d+) (\d+)`)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if match := pattern.FindSubmatch(c.Stderr()); match != nil {
			port, _ = strconv.Atoi(string(match[1]))
			pid, _ = strconv.Atoi(string(match[2]))
			return port, pid
		}
		if time.Now().After(deadline) {
			t.Fatalf("sidecar listener never started: %q", c.Stderr())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// runListenHelper listens on a free loopback port and on the excluded one,
// reports them, and waits to be killed.
func runListenHelper(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	own, err := net.Listen("tcp", "127.0.0.1:"+os.Getenv("WEBMUX_TEST_EXCLUDED_PORT"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = own.Close() }()
	fmt.Printf("LISTENING %d %d\n", listener.Addr().(*net.TCPAddr).Port, os.Getpid())
	time.Sleep(time.Minute)
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port
}

func helperListening(t *testing.T, s *Session) (port, pid int) {
	t.Helper()
	pattern := regexp.MustCompile(`LISTENING (\d+) (\d+)`)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if match := pattern.FindSubmatch(s.Buffer()); match != nil {
			port, _ = strconv.Atoi(string(match[1]))
			pid, _ = strconv.Atoi(string(match[2]))
			return port, pid
		}
		if time.Now().After(deadline) {
			t.Fatalf("listener never started: %q", s.Buffer())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// waitListeningPorts waits for an updated event of session id whose ports
// satisfy done; no event may report the excluded port.
func waitListeningPorts(t *testing.T, sub *ListSubscriber, id string, excluded int, done func([]ListeningPort) bool) {
	t.Helper()
	timeout := time.After(listenDeadline)
	for {
		select {
		case event := <-sub.Out:
			if event.Type != "updated" || event.Session.ID != id {
				continue
			}
			ports := event.Session.ListeningPorts
			if slices.ContainsFunc(ports, func(p ListeningPort) bool { return p.Port == excluded }) {
				t.Fatalf("excluded port %d reported: %+v", excluded, ports)
			}
			if done(ports) {
				return
			}
		case <-sub.Slow:
			t.Fatal("list subscriber fell behind")
		case <-timeout:
			t.Fatalf("listening ports not updated within %s", listenDeadline)
		}
	}
}
