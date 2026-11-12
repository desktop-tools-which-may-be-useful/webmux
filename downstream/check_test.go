package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkingManifestSchemaAndSources(t *testing.T) {
	data, err := os.ReadFile("ownership.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := decodeManifest(data)
	if err != nil {
		t.Fatalf("decodeManifest: %v", err)
	}
	if violations := validateManifest(m); len(violations) != 0 {
		t.Fatalf("validateManifest:\n%s", strings.Join(violations, "\n"))
	}

	root := filepath.Clean("..")
	document, err := os.ReadFile(filepath.Join(root, m.FeatureDocument))
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(m.Features))
	for i, feature := range m.Features {
		ids[i] = feature.ID
	}
	if got := markdownFeatureTableIDs(document); strings.Join(got, "\x00") != strings.Join(ids, "\x00") {
		t.Errorf("feature table IDs are %v, want %v", got, ids)
	}
	if got := markdownFeatureHeadings(document); strings.Join(got, "\x00") != strings.Join(ids, "\x00") {
		t.Errorf("feature headings are %v, want %v", got, ids)
	}
	statuses := markdownFeatureStatuses(document)
	for _, feature := range m.Features {
		if got := statuses[feature.ID]; len(got) != 1 || got[0] != feature.Status {
			t.Errorf("feature %s statuses are %v, want [%s]", feature.ID, got, feature.Status)
		}
		_, anchor, _ := strings.Cut(feature.FactSource, "#")
		if !markdownHasAnchor(document, anchor) {
			t.Errorf("feature %s factSource anchor #%s is missing", feature.ID, anchor)
		}
		for _, bridge := range feature.Bridges {
			content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(bridge.Path)))
			if err != nil {
				t.Fatal(err)
			}
			if count := strings.Count(string(content), bridge.Marker); count != 1 {
				t.Errorf("feature %s bridge marker occurs %d times in %s, want exactly once", feature.ID, count, bridge.Path)
			}
		}
	}
}

func TestDecodeManifestRequiresFrozenSchema(t *testing.T) {
	valid := map[string]any{
		"schemaVersion":   1,
		"mode":            "overlay",
		"upstreamBase":    strings.Repeat("a", 40),
		"featureDocument": "downstream/README.md",
		"features": []any{map[string]any{
			"id":            "WM-DS-01",
			"status":        "active",
			"factSource":    "downstream/README.md#wm-ds-01",
			"ownedFiles":    []any{},
			"ownedPrefixes": []any{},
			"mixedFiles":    []any{},
			"bridges":       []any{},
		}},
	}
	encode := func(value any) []byte {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	if _, err := decodeManifest(encode(valid)); err != nil {
		t.Fatalf("valid schema: %v", err)
	}

	withUnknown := cloneJSON(t, valid)
	withUnknown["extra"] = true
	if _, err := decodeManifest(encode(withUnknown)); err == nil {
		t.Error("unknown top-level field was accepted")
	}

	missingArray := cloneJSON(t, valid)
	feature := missingArray["features"].([]any)[0].(map[string]any)
	delete(feature, "ownedPrefixes")
	if _, err := decodeManifest(encode(missingArray)); err == nil {
		t.Error("missing required feature array was accepted")
	}

	withUnknownFeature := cloneJSON(t, valid)
	feature = withUnknownFeature["features"].([]any)[0].(map[string]any)
	feature["extra"] = true
	if _, err := decodeManifest(encode(withUnknownFeature)); err == nil {
		t.Error("unknown feature field was accepted")
	}
}

func TestRejectsBaseOutsideTrustedUpstreamHistory(t *testing.T) {
	root := newOverlayRepository(t)
	fakeBase := strings.TrimSpace(gitTest(t, root, "rev-parse", "HEAD"))
	updateTestManifest(t, root, func(value map[string]any) {
		value["upstreamBase"] = fakeBase
	})
	gitTest(t, root, "add", manifestPath)
	gitTest(t, root, "commit", "-qm", "point at downstream-only base")

	_, err := checkRepository(root)
	if err == nil || !strings.Contains(err.Error(), "is not in trusted upstream history") {
		t.Fatalf("downstream-only base error = %v", err)
	}
}

func TestRenderedMarkdownContext(t *testing.T) {
	root := newOverlayRepository(t)
	document := testFeatureDocument("WM-DS-01", "active") + `

~~~markdown
## WM-DS-99
| **WM-DS-99** | fenced | **active** |
~~~

    ## WM-DS-98
    | **WM-DS-98** | indented | **active** |

<!--
## WM-DS-97
| **WM-DS-97** | commented | **active** |
-->

## Audit

| ID | Result | Status |
| --- | --- | --- |
| **WM-DS-96** | unrelated | **active** |
`
	writeTestFile(t, root, "downstream/README.md", document)
	writeTestFile(t, root, "AGENTS.md", `base
[downstream](downstream/AGENTS.md)

~~~markdown
[downstream](downstream/AGENTS.md)
~~~

    [downstream](downstream/AGENTS.md)

<!-- [downstream](downstream/AGENTS.md) -->
`)
	gitTest(t, root, "add", "AGENTS.md", "downstream/README.md")
	gitTest(t, root, "commit", "-qm", "add non-rendered examples")

	if violations, err := checkRepository(root); err != nil || len(violations) != 0 {
		t.Fatalf("non-rendered examples affected ownership facts: violations=%v err=%v", violations, err)
	}
}

func TestFencedFeatureCatalogDoesNotSatisfyManifest(t *testing.T) {
	root := newOverlayRepository(t)
	document := strings.Replace(
		testFeatureDocument("WM-DS-01", "active"),
		"| ID | Feature | Status |\n| --- | --- | --- |\n| `WM-DS-01` | Feature | `active` |",
		"```markdown\n| ID | Feature | Status |\n| --- | --- | --- |\n| `WM-DS-01` | Feature | `active` |\n```",
		1,
	)
	writeTestFile(t, root, "downstream/README.md", document)
	gitTest(t, root, "add", "downstream/README.md")
	gitTest(t, root, "commit", "-qm", "fence feature catalog")

	violations, err := checkRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	if !containsViolation(violations, "feature table IDs are [], want exactly [WM-DS-01]") {
		t.Fatalf("violations %v allow a fenced feature catalog", violations)
	}
}

func TestFactSourceRejectsHeadingAnchorCollision(t *testing.T) {
	for name, collision := range map[string]string{
		"atx":    "## WM DS 01",
		"setext": "WM DS 01\n---",
	} {
		t.Run(name, func(t *testing.T) {
			root := newOverlayRepository(t)
			document := strings.Replace(
				testFeatureDocument("WM-DS-01", "active"),
				"## WM-DS-01",
				collision+"\n\nCollision.\n\n## WM-DS-01",
				1,
			)
			writeTestFile(t, root, "downstream/README.md", document)
			gitTest(t, root, "add", "downstream/README.md")
			gitTest(t, root, "commit", "-qm", "collide feature anchor")

			violations, err := checkRepository(root)
			if err != nil {
				t.Fatal(err)
			}
			if !containsViolation(violations, "exact heading anchor is #wm-ds-01-1") {
				t.Fatalf("violations %v do not reject a colliding preceding heading", violations)
			}
		})
	}
}

func TestRetiredFeatureMustReleaseClaims(t *testing.T) {
	retired := feature{
		ID:         "WM-DS-01",
		Status:     "retired",
		FactSource: "downstream/README.md#wm-ds-01",
		OwnedFiles: []string{"owned.txt"},
	}
	if violations := validateFeature(retired, "downstream/README.md"); !containsViolation(violations, "retired feature must not retain ownership claims") {
		t.Fatalf("retired feature claims were accepted: %v", violations)
	}
	retired.Status = "retiring"
	if violations := validateFeature(retired, "downstream/README.md"); containsViolation(violations, "retired feature must not retain ownership claims") {
		t.Fatalf("retiring feature claims were rejected: %v", violations)
	}
}

func TestDecodeManifestRejectsDuplicateRawKeys(t *testing.T) {
	data := []byte(`{"schemaVersion":1,"schemaVersion":1,"mode":"overlay","upstreamBase":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","featureDocument":"downstream/README.md","features":[]}`)
	if _, err := decodeManifest(data); err == nil || !strings.Contains(err.Error(), `duplicate JSON key "schemaVersion"`) {
		t.Fatalf("duplicate key error = %v", err)
	}
}

func TestCommittedOverlayIgnoresDirtyAndRejectsUnclassifiedCommit(t *testing.T) {
	root := newOverlayRepository(t)
	if violations, err := checkRepository(root); err != nil || len(violations) != 0 {
		t.Fatalf("initial overlay: violations=%v err=%v", violations, err)
	}

	writeTestFile(t, root, "dirty.txt", "not committed\n")
	writeTestFile(t, root, "AGENTS.md", "base\n[downstream](downstream/AGENTS.md)\nworking tree only\n")
	if violations, err := checkRepository(root); err != nil || len(violations) != 0 {
		t.Fatalf("dirty tree changed committed check: violations=%v err=%v", violations, err)
	}

	gitTest(t, root, "add", "dirty.txt")
	gitTest(t, root, "commit", "-qm", "unclassified committed file")
	violations, err := checkRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	if !containsViolation(violations, "unclassified added path dirty.txt") {
		t.Fatalf("violations %v do not reject committed dirty.txt", violations)
	}
}

func TestValidatePathsRejectParentDirectory(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "file", err: validateFilePath("..")},
		{name: "prefix", err: validatePrefix("../")},
	} {
		if test.err == nil {
			t.Errorf("%s path accepted parent directory", test.name)
		}
	}
}

func TestOwnedClaimsMustBeUniqueAcrossFeatures(t *testing.T) {
	m := manifest{Features: []feature{
		{ID: "WM-DS-01", OwnedFiles: []string{"owned.txt"}, OwnedPrefixes: []string{"owned/"}},
		{ID: "WM-DS-02", OwnedFiles: []string{"owned.txt"}, OwnedPrefixes: []string{"owned/nested/"}, MixedFiles: []string{"owned.txt"}},
	}}
	violations := checkClaims(m, []change{{Status: "A", Path: "owned.txt"}, {Status: "A", Path: "owned/file.txt"}})
	if !containsViolation(violations, "owned file owned.txt is claimed by multiple features") {
		t.Fatalf("violations %v do not reject duplicate owned file", violations)
	}
	if !containsViolation(violations, "owned.txt is both owned and mixed") {
		t.Fatalf("violations %v do not reject owned-vs-mixed overlap", violations)
	}
	if !containsViolation(violations, "owned prefixes overlap") {
		t.Fatalf("violations %v do not reject overlapping owned prefixes", violations)
	}
}

func TestFeatureStatusMustMatchDocument(t *testing.T) {
	root := newOverlayRepository(t)
	writeTestFile(t, root, "downstream/README.md", testFeatureDocument("WM-DS-01", "retired"))
	gitTest(t, root, "add", "downstream/README.md")
	gitTest(t, root, "commit", "-qm", "drift feature status")

	violations, err := checkRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	if !containsViolation(violations, "feature WM-DS-01 status in featureDocument") {
		t.Fatalf("violations %v do not reject status drift", violations)
	}
}

func TestFeatureDocumentRejectsUnregisteredFeature(t *testing.T) {
	root := newOverlayRepository(t)
	document := strings.Replace(
		testFeatureDocument("WM-DS-01", "active"),
		"| `WM-DS-01` | Feature | `active` |",
		"| `WM-DS-01` | Feature | `active` |\n| **WM-DS-99** | Extra | **active** |",
		1,
	) + "\n## WM-DS-99\n"
	writeTestFile(t, root, "downstream/README.md", document)
	gitTest(t, root, "add", "downstream/README.md")
	gitTest(t, root, "commit", "-qm", "add unregistered feature")

	violations, err := checkRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	if !containsViolation(violations, "feature table IDs are [WM-DS-01 WM-DS-99], want exactly [WM-DS-01]") {
		t.Fatalf("violations %v do not reject formatted unregistered feature table row", violations)
	}
	if !containsViolation(violations, "feature headings are [WM-DS-01 WM-DS-99], want exactly [WM-DS-01]") {
		t.Fatalf("violations %v do not reject unregistered feature heading", violations)
	}
}

func TestStableFeatureIDMustBeLevelTwoHeading(t *testing.T) {
	root := newOverlayRepository(t)
	document := strings.Replace(testFeatureDocument("WM-DS-01", "active"), "## WM-DS-01", "### WM-DS-01", 1)
	writeTestFile(t, root, "downstream/README.md", document)
	gitTest(t, root, "add", "downstream/README.md")
	gitTest(t, root, "commit", "-qm", "demote feature heading")

	violations, err := checkRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	if !containsViolation(violations, "feature headings are [], want exactly [WM-DS-01]") {
		t.Fatalf("violations %v do not reject non-level-two feature heading", violations)
	}
}

func TestStableFeatureIDMustBeExactHeading(t *testing.T) {
	root := newOverlayRepository(t)
	writeTestFile(t, root, "downstream/README.md", testFeatureDocument("WM DS 01", "active"))
	gitTest(t, root, "add", "downstream/README.md")
	gitTest(t, root, "commit", "-qm", "rewrite feature heading")

	violations, err := checkRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	if !containsViolation(violations, "feature WM-DS-01 heading occurs 0 times") {
		t.Fatalf("violations %v do not reject rewritten stable ID heading", violations)
	}
}

func TestBridgeMarkerMustOccurExactlyOnce(t *testing.T) {
	root := newOverlayRepository(t)
	writeTestFile(t, root, "AGENTS.md", "base\n[downstream](downstream/AGENTS.md)\n[downstream](downstream/AGENTS.md)\n")
	gitTest(t, root, "add", "AGENTS.md")
	gitTest(t, root, "commit", "-qm", "duplicate bridge")

	violations, err := checkRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	if !containsViolation(violations, "bridge marker occurs 2 times") {
		t.Fatalf("violations %v do not reject duplicate bridge marker", violations)
	}
}

func newOverlayRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitTest(t, root, "init", "-q")
	gitTest(t, root, "config", "user.name", "Downstream Test")
	gitTest(t, root, "config", "user.email", "downstream@example.invalid")
	writeTestFile(t, root, "AGENTS.md", "base\n")
	writeTestFile(t, root, "upstream.txt", "base\n")
	gitTest(t, root, "add", ".")
	gitTest(t, root, "commit", "-qm", "upstream base")
	base := strings.TrimSpace(gitTest(t, root, "rev-parse", "HEAD"))
	gitTest(t, root, "update-ref", trustedUpstreamRef, base)

	writeTestFile(t, root, "AGENTS.md", "base\n[downstream](downstream/AGENTS.md)\n")
	writeTestFile(t, root, "overlay.txt", "downstream\n")
	writeTestFile(t, root, "downstream/README.md", testFeatureDocument("WM-DS-01", "active"))
	manifestData := map[string]any{
		"schemaVersion":   1,
		"mode":            "overlay",
		"upstreamBase":    base,
		"featureDocument": "downstream/README.md",
		"features": []any{map[string]any{
			"id":            "WM-DS-01",
			"status":        "active",
			"factSource":    "downstream/README.md#wm-ds-01",
			"ownedFiles":    []string{"overlay.txt"},
			"ownedPrefixes": []string{"downstream/"},
			"mixedFiles":    []string{"AGENTS.md"},
			"bridges": []any{map[string]any{
				"path":   "AGENTS.md",
				"marker": "[downstream](downstream/AGENTS.md)",
			}},
		}},
	}
	data, err := json.MarshalIndent(manifestData, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, manifestPath, string(data)+"\n")
	gitTest(t, root, "add", ".")
	gitTest(t, root, "commit", "-qm", "downstream overlay")
	return root
}

func testFeatureDocument(heading, status string) string {
	return "# Overlay\n\n" +
		"## Features\n\n" +
		"| ID | Feature | Status |\n" +
		"| --- | --- | --- |\n" +
		"| `WM-DS-01` | Feature | `" + status + "` |\n\n" +
		"## " + heading + "\n"
}

func updateTestManifest(t *testing.T, root string, mutate func(map[string]any)) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(manifestPath)))
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	mutate(value)
	data, err = json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, manifestPath, string(data)+"\n")
}

func cloneJSON(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var clone map[string]any
	if err := json.Unmarshal(data, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func writeTestFile(t *testing.T, root, name, content string) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-C", root}, args...)
	command := exec.Command("git", commandArgs...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func containsViolation(violations []string, want string) bool {
	for _, violation := range violations {
		if strings.Contains(violation, want) {
			return true
		}
	}
	return false
}
