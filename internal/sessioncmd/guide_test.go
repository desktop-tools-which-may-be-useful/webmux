//go:build linux || darwin

package sessioncmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainchan/webmux/internal/agentguide"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

func guideEnv(t *testing.T, serve bool, session string) (Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "scg")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "s.sock")
	if serve {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /llms.txt", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "# served index\n") })
		mux.HandleFunc("GET /llms-full.txt", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "# served full\n") })
		mux.HandleFunc("GET /llms/{page}", func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "# served "+r.PathValue("page")+"\n")
		})
		mux.HandleFunc("POST /api/sessions/{id}/artifacts", func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(contract.ArtifactsFolder{Path: "/tmp/artifacts/" + r.PathValue("id")})
		})
		mux.HandleFunc("GET "+contract.PathStatus, func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(contract.StatusV1{V: 1, Bin: "/opt/webmux/bin/webmux", Socket: socket})
		})
		l, err := net.Listen("unix", socket)
		if err != nil {
			t.Fatal(err)
		}
		srv := &http.Server{Handler: mux}
		go func() { _ = srv.Serve(l) }()
		t.Cleanup(func() { _ = srv.Close() })
	}
	var stdout, stderr bytes.Buffer
	return Env{
		Connect: func() (*localclient.Client, error) { return localclient.New(localclient.UnixDialer{Path: socket}), nil },
		Stdout:  &stdout, Stderr: &stderr,
		Getenv: func(name string) string {
			if name == contract.SessionIDEnv {
				return session
			}
			return ""
		},
		NotRunningHint: "start it",
	}, &stdout, &stderr
}

func TestGuideReadsTheServer(t *testing.T) {
	e, stdout, _ := guideEnv(t, true, "")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, "# served index\n"},
		{[]string{"--full"}, "# served full\n"},
		{[]string{"artifacts"}, "# served artifacts.md\n"},
	} {
		stdout.Reset()
		if code := e.Run(context.Background(), guideCommand, tc.args); code != 0 || stdout.String() != tc.want {
			t.Errorf("guide %v: %d %q", tc.args, code, stdout)
		}
	}
	stdout.Reset()
	if code := e.Run(context.Background(), guideCommand, []string{"--path"}); code != 0 || !strings.HasPrefix(stdout.String(), "/opt/webmux/bin/webmux\n") {
		t.Fatalf("--path: %d %q", code, stdout)
	}
	if code := e.Run(context.Background(), guideCommand, []string{"nosuch"}); code != contract.ExitUsage {
		t.Fatalf("unknown topic: %d", code)
	}
	if code := e.Run(context.Background(), guideCommand, []string{"--full", "--path"}); code != contract.ExitUsage {
		t.Fatalf("--full --path: %d", code)
	}
}

func TestGuideFallsBackToTheBuiltInCopy(t *testing.T) {
	e, stdout, stderr := guideEnv(t, false, "")
	guide := agentguide.New(Directory(Commands()))
	if code := e.Run(context.Background(), guideCommand, nil); code != 0 || stdout.String() != guide.Index(nil) || !strings.Contains(stderr.String(), "built into this executable") {
		t.Fatalf("fallback: %d %q %q", code, stdout.String()[:min(40, stdout.Len())], stderr)
	}
	// The offline commands topic is the directory of this CLI's table.
	stdout.Reset()
	if code := e.Run(context.Background(), guideCommand, []string{"commands"}); code != 0 || !strings.Contains(stdout.String(), "`webmux send [options] SESSION [TEXT|-]`") {
		t.Fatalf("offline commands topic: %d %q", code, stdout)
	}
	stdout.Reset()
	if code := e.Run(context.Background(), guideCommand, []string{"--full"}); code != 0 || strings.Contains(stdout.String(), "Options:") {
		t.Fatalf("offline --full: %d", code)
	}
	if code := e.Run(context.Background(), guideCommand, []string{"--path"}); code != contract.ExitNotRunning {
		t.Fatalf("--path without a server: %d", code)
	}
}

func TestDirPrintsTheArtifactsFolder(t *testing.T) {
	e, stdout, _ := guideEnv(t, true, "abcdefgh")
	if code := e.Run(context.Background(), dirCommand, nil); code != 0 || stdout.String() != "/tmp/artifacts/abcdefgh\n" {
		t.Fatalf("dir: %d %q", code, stdout)
	}
	outside, _, _ := guideEnv(t, true, "")
	if code := outside.Run(context.Background(), dirCommand, nil); code != contract.ExitUsage {
		t.Fatalf("dir outside a session: %d", code)
	}
}

// TestDirectoryListsEveryCommandOnce keeps the commands topic a directory:
// one line per visible command, no help sections.
func TestDirectoryListsEveryCommandOnce(t *testing.T) {
	r := Commands()
	page := Directory(r)
	for _, c := range r.Visible("unix") {
		if c.Bind == nil {
			continue
		}
		line := "- `" + c.Usage() + "`: " + c.Summary + "\n"
		if strings.Count(page, line) != 1 {
			t.Errorf("directory lists %s %d times", c.Name, strings.Count(page, line))
		}
	}
	for _, c := range r {
		if c.Hidden && strings.Contains(page, "`webmux "+c.Name+" ") {
			t.Errorf("directory lists hidden %s", c.Name)
		}
	}
	for _, section := range []string{"Options:", "Examples:", "Arguments:", "Exit status:"} {
		if strings.Contains(page, section) {
			t.Errorf("directory contains %q", section)
		}
	}
	if !strings.Contains(page, "--help") || !strings.Contains(page, "help --json") {
		t.Error("directory does not point to --help and help --json")
	}
}
