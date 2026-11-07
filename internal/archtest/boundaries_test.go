package archtest

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestSessionFocusedBoundaries verifies that internal/session stays focused on
// PTY lifecycle, the ring-buffer I/O pump, and session management by
// delegating VT preview emulation to internal/vtpreview and cross-platform
// process/port inspection to internal/procpeek.
func TestSessionFocusedBoundaries(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range []string{"internal/vtpreview", "internal/procpeek"} {
		if webmux.layers[pkg] != shared {
			t.Errorf("%s layer = %q, want %q", pkg, webmux.layers[pkg], shared)
		}
	}

	// 1. internal/session must import both internal/vtpreview and
	// internal/procpeek, and must never import xterm-go/vt directly or read
	// /proc / XNU proc_info tables itself.
	sessionFiles := parseDirGoFiles(t, filepath.Join(root, "internal/session"), false)
	importsVTPreview := false
	importsProcPeek := false
	for _, sf := range sessionFiles {
		for _, imp := range sf.ast.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			switch path {
			case "github.com/rainchan/webmux/internal/vtpreview":
				importsVTPreview = true
			case "github.com/rainchan/webmux/internal/procpeek":
				importsProcPeek = true
			case "github.com/0magnet/xterm-go/vt":
				t.Errorf("%s imports %s directly; VT emulation belongs in internal/vtpreview", sf.rel, path)
			}
		}
		ast.Inspect(sf.ast, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BasicLit:
				if x.Kind == token.STRING {
					v, _ := strconv.Unquote(x.Value)
					if strings.HasPrefix(v, "/proc") {
						t.Errorf("%s:%d references %q; /proc scanning belongs in internal/procpeek", sf.rel, sf.fset.Position(x.Pos()).Line, v)
					}
				}
			case *ast.SelectorExpr:
				switch x.Sel.Name {
				case "SYS_PROC_INFO", "SysctlKinfoProc", "SysctlKinfoProcSlice":
					t.Errorf("%s:%d uses %s; Darwin process/socket inspection belongs in internal/procpeek", sf.rel, sf.fset.Position(x.Pos()).Line, x.Sel.Name)
				}
			}
			return true
		})
	}
	if !importsVTPreview {
		t.Error("internal/session does not import internal/vtpreview")
	}
	if !importsProcPeek {
		t.Error("internal/session does not import internal/procpeek")
	}

	// 2. internal/vtpreview and internal/procpeek must be self-contained and
	// never depend on internal/session or on each other.
	for _, pkg := range []string{"internal/vtpreview", "internal/procpeek"} {
		for _, pf := range parseDirGoFiles(t, filepath.Join(root, pkg), true) {
			for _, imp := range pf.ast.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				if path == "github.com/rainchan/webmux/internal/session" ||
					(pkg == "internal/vtpreview" && path == "github.com/rainchan/webmux/internal/procpeek") ||
					(pkg == "internal/procpeek" && path == "github.com/rainchan/webmux/internal/vtpreview") {
					t.Errorf("%s imports %s", pf.rel, path)
				}
			}
		}
	}
}

// TestCliCmdPackageBoundary verifies that the shared CLI command package lives
// at internal/clicmd (not internal/sessioncmd) and is classified in the client
// layer.
func TestCliCmdPackageBoundary(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "internal/sessioncmd")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("legacy internal/sessioncmd directory still exists (%v); CLI commands belong in internal/clicmd", err)
	}
	if webmux.layers["internal/clicmd"] != client {
		t.Errorf("internal/clicmd layer = %q, want %q", webmux.layers["internal/clicmd"], client)
	}
	files := parseDirGoFiles(t, filepath.Join(root, "internal/clicmd"), false)
	if len(files) == 0 {
		t.Fatal("internal/clicmd has no production Go files")
	}
	for _, f := range files {
		if f.ast.Name.Name != "clicmd" {
			t.Errorf("%s declares package %s, want clicmd", f.rel, f.ast.Name.Name)
		}
	}
}

// TestTraySystemSeparation verifies that Windows system administration
// operations (firewall rules, login autostart registry, single-instance/quit
// process termination, installer launch) are isolated in internal/tray/traysys,
// keeping internal/tray focused on platform-independent state/views and
// internal/tray/winui focused on the notification-area UI.
func TestTraySystemSeparation(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range []string{"internal/tray", "internal/tray/traysys", "internal/tray/winui"} {
		if webmux.layers[pkg] != client {
			t.Errorf("%s layer = %q, want %q", pkg, webmux.layers[pkg], client)
		}
	}

	// 1. internal/tray must not import traysys, winui, or Windows registry/GUI
	// packages.
	for _, f := range parseDirGoFiles(t, filepath.Join(root, "internal/tray"), true) {
		for _, imp := range f.ast.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			switch path {
			case "github.com/rainchan/webmux/internal/tray/traysys",
				"github.com/rainchan/webmux/internal/tray/winui",
				"golang.org/x/sys/windows/registry",
				"fyne.io/systray":
				t.Errorf("%s imports %s; internal/tray must stay focused on tray state and views", f.rel, path)
			}
		}
	}

	// 2. internal/tray/winui must not import traysys, registry, or os/exec,
	// nor invoke system administration primitives directly.
	forbiddenWinUISymbols := map[string]string{
		"CreateToolhelp32Snapshot": "process table scanning belongs in internal/tray/traysys",
		"Process32First":           "process table scanning belongs in internal/tray/traysys",
		"Process32Next":            "process table scanning belongs in internal/tray/traysys",
		"TerminateProcess":         "process termination belongs in internal/tray/traysys",
		"OpenProcess":              "process handle management belongs in internal/tray/traysys",
		"CreateMutex":              "single-instance mutex management belongs in internal/tray/traysys",
		"OpenMutex":                "single-instance mutex management belongs in internal/tray/traysys",
		"ShellExecute":             "external process/browser launch belongs in internal/tray/traysys",
		"ShellExecuteExW":          "elevated command execution belongs in internal/tray/traysys",
	}
	for _, f := range parseDirGoFiles(t, filepath.Join(root, "internal/tray/winui"), true) {
		for _, imp := range f.ast.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			switch path {
			case "github.com/rainchan/webmux/internal/tray/traysys",
				"golang.org/x/sys/windows/registry",
				"os/exec":
				t.Errorf("%s imports %s; system administration belongs in internal/tray/traysys", f.rel, path)
			}
		}
		ast.Inspect(f.ast, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if reason, bad := forbiddenWinUISymbols[x.Sel.Name]; bad {
					t.Errorf("%s:%d uses %s (%s)", f.rel, f.fset.Position(x.Pos()).Line, x.Sel.Name, reason)
				}
			case *ast.BasicLit:
				if x.Kind == token.STRING {
					v, _ := strconv.Unquote(x.Value)
					if v == "netsh.exe" || v == "ShellExecuteExW" || strings.Contains(v, `CurrentVersion\Run`) || strings.Contains(v, `CurrentVersion\Uninstall`) {
						t.Errorf("%s:%d contains system-management literal %q; move to internal/tray/traysys", f.rel, f.fset.Position(x.Pos()).Line, v)
					}
				}
			}
			return true
		})
	}

	// 3. internal/tray/traysys must not depend on internal/tray or winui.
	traysysFiles := parseDirGoFiles(t, filepath.Join(root, "internal/tray/traysys"), true)
	if len(traysysFiles) == 0 {
		t.Fatal("internal/tray/traysys has no Go files")
	}
	for _, f := range traysysFiles {
		for _, imp := range f.ast.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if path == "github.com/rainchan/webmux/internal/tray" ||
				path == "github.com/rainchan/webmux/internal/tray/winui" ||
				path == "fyne.io/systray" {
				t.Errorf("%s imports %s; internal/tray/traysys must be a leaf system package", f.rel, path)
			}
		}
	}
}

type parsedGoFile struct {
	rel  string
	fset *token.FileSet
	ast  *ast.File
}

func parseDirGoFiles(t *testing.T, dir string, includeTests bool) []parsedGoFile {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []parsedGoFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if !includeTests && strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		full := filepath.Join(dir, e.Name())
		rel, _ := filepath.Rel(root, full)
		rel = filepath.ToSlash(rel)
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, full, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		out = append(out, parsedGoFile{rel: rel, fset: fset, ast: file})
	}
	return out
}
