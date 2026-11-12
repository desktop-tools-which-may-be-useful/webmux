// Command downstream-check verifies the committed downstream overlay against
// downstream/ownership.json. It intentionally ignores the working tree.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"
)

const (
	manifestPath       = "downstream/ownership.json"
	trustedUpstreamRef = "refs/remotes/upstream/main"
)

type manifest struct {
	SchemaVersion   int       `json:"schemaVersion"`
	Mode            string    `json:"mode"`
	UpstreamBase    string    `json:"upstreamBase"`
	FeatureDocument string    `json:"featureDocument"`
	Features        []feature `json:"features"`
}

type feature struct {
	ID            string   `json:"id"`
	Status        string   `json:"status"`
	FactSource    string   `json:"factSource"`
	OwnedFiles    []string `json:"ownedFiles"`
	OwnedPrefixes []string `json:"ownedPrefixes"`
	MixedFiles    []string `json:"mixedFiles"`
	Bridges       []bridge `json:"bridges"`
}

type bridge struct {
	Path   string `json:"path"`
	Marker string `json:"marker"`
}

type change struct {
	Status string
	Path   string
}

func main() {
	violations, err := checkRepository(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "downstream ownership check:", err)
		os.Exit(1)
	}
	if len(violations) != 0 {
		fmt.Fprintln(os.Stderr, "downstream ownership check failed:")
		for _, violation := range violations {
			fmt.Fprintln(os.Stderr, " -", violation)
		}
		fmt.Fprintln(os.Stderr, "See downstream/README.md.")
		os.Exit(1)
	}
	fmt.Println("downstream ownership check passed")
}

func checkRepository(root string) ([]string, error) {
	data, err := gitOutput(root, "show", "HEAD:"+manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read committed %s: %w", manifestPath, err)
	}
	m, err := decodeManifest(data)
	if err != nil {
		return nil, fmt.Errorf("decode committed %s: %w", manifestPath, err)
	}

	violations := validateManifest(m)
	if len(violations) != 0 {
		return violations, nil
	}
	if err := gitRun(root, "cat-file", "-e", m.UpstreamBase+"^{commit}"); err != nil {
		return nil, fmt.Errorf("upstreamBase %s is unavailable: %w", m.UpstreamBase, err)
	}
	if err := gitRun(root, "cat-file", "-e", trustedUpstreamRef+"^{commit}"); err != nil {
		return nil, fmt.Errorf("trusted upstream ref %s is unavailable: %w", trustedUpstreamRef, err)
	}
	if err := gitRun(root, "merge-base", "--is-ancestor", m.UpstreamBase, trustedUpstreamRef); err != nil {
		return nil, fmt.Errorf("upstreamBase %s is not in trusted upstream history %s", m.UpstreamBase, trustedUpstreamRef)
	}
	if err := gitRun(root, "merge-base", "--is-ancestor", m.UpstreamBase, "HEAD"); err != nil {
		return nil, fmt.Errorf("upstreamBase %s is not an ancestor of HEAD", m.UpstreamBase)
	}

	changes, err := committedChanges(root, m.UpstreamBase)
	if err != nil {
		return nil, err
	}
	violations = append(violations, checkClaims(m, changes)...)
	violations = append(violations, checkSources(root, m)...)
	sort.Strings(violations)
	return violations, nil
}

func decodeManifest(data []byte) (manifest, error) {
	if err := rejectDuplicateKeys(data); err != nil {
		return manifest{}, err
	}
	var top map[string]json.RawMessage
	if err := decodeOne(data, &top); err != nil {
		return manifest{}, err
	}
	if err := requireFields(top, "manifest", []string{
		"featureDocument", "features", "mode", "schemaVersion", "upstreamBase",
	}); err != nil {
		return manifest{}, err
	}

	var mode string
	if err := json.Unmarshal(top["mode"], &mode); err != nil {
		return manifest{}, fmt.Errorf("mode: %w", err)
	}
	if mode != "overlay" {
		return manifest{}, fmt.Errorf("mode is %q, want overlay", mode)
	}

	var rawFeatures []map[string]json.RawMessage
	if err := json.Unmarshal(top["features"], &rawFeatures); err != nil {
		return manifest{}, fmt.Errorf("features must be an array: %w", err)
	}
	if rawFeatures == nil {
		return manifest{}, errors.New("features must be an array, not null")
	}
	featureFields := []string{
		"bridges", "factSource", "id", "mixedFiles", "ownedFiles",
		"ownedPrefixes", "status",
	}
	for i, rawFeature := range rawFeatures {
		if err := requireFields(rawFeature, fmt.Sprintf("features[%d]", i), featureFields); err != nil {
			return manifest{}, err
		}
		for _, name := range []string{"ownedFiles", "ownedPrefixes", "mixedFiles", "bridges"} {
			if bytes.Equal(bytes.TrimSpace(rawFeature[name]), []byte("null")) {
				return manifest{}, fmt.Errorf("features[%d].%s must be an array, not null", i, name)
			}
		}
		var rawBridges []map[string]json.RawMessage
		if err := json.Unmarshal(rawFeature["bridges"], &rawBridges); err != nil {
			return manifest{}, fmt.Errorf("features[%d].bridges must be an array: %w", i, err)
		}
		for j, rawBridge := range rawBridges {
			if err := requireFields(rawBridge, fmt.Sprintf("features[%d].bridges[%d]", i, j), []string{"marker", "path"}); err != nil {
				return manifest{}, err
			}
		}
	}

	var m manifest
	if err := decodeOne(data, &m); err != nil {
		return manifest{}, err
	}
	return m, nil
}

func decodeOne(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("multiple JSON values")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func requireFields(object map[string]json.RawMessage, name string, want []string) error {
	got := make([]string, 0, len(object))
	for field := range object {
		got = append(got, field)
	}
	sort.Strings(got)
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		return fmt.Errorf("%s fields are %v, want exactly %v", name, got, want)
	}
	return nil
}

func rejectDuplicateKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return errors.New("object key is not a string")
				}
				if seen[key] {
					return fmt.Errorf("duplicate JSON key %q", key)
				}
				seen[key] = true
				if err := walk(); err != nil {
					return err
				}
			}
			_, err := decoder.Token()
			return err
		case '[':
			for decoder.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err := decoder.Token()
			return err
		default:
			return fmt.Errorf("unexpected JSON delimiter %q", delim)
		}
	}
	if err := walk(); err != nil {
		return err
	}
	if _, err := decoder.Token(); err == nil {
		return errors.New("multiple JSON values")
	}
	return nil
}

var (
	commitRE  = regexp.MustCompile(`^[0-9a-f]{40}$`)
	featureRE = regexp.MustCompile(`^WM-DS-[0-9]{2}$`)
)

func validateManifest(m manifest) []string {
	var violations []string
	if m.SchemaVersion != 1 {
		violations = append(violations, fmt.Sprintf("schemaVersion is %d, want 1", m.SchemaVersion))
	}
	if m.Mode != "overlay" {
		violations = append(violations, fmt.Sprintf("mode is %q, want overlay", m.Mode))
	}
	if !commitRE.MatchString(m.UpstreamBase) {
		violations = append(violations, "upstreamBase must be a full lowercase commit ID")
	}
	if err := validateFilePath(m.FeatureDocument); err != nil {
		violations = append(violations, "featureDocument: "+err.Error())
	}
	if len(m.Features) == 0 {
		violations = append(violations, "features must not be empty")
	}

	ids := make([]string, len(m.Features))
	for i, feature := range m.Features {
		ids[i] = feature.ID
		violations = append(violations, validateFeature(feature, m.FeatureDocument)...)
	}
	violations = append(violations, sortedUnique("feature ids", ids)...)
	sort.Strings(violations)
	return violations
}

func validateFeature(f feature, featureDocument string) []string {
	prefix := "feature " + f.ID + ": "
	var violations []string
	if !featureRE.MatchString(f.ID) {
		violations = append(violations, prefix+"id must match WM-DS-NN")
	}
	if f.Status != "active" && f.Status != "pending-upstream" && f.Status != "retiring" && f.Status != "retired" {
		violations = append(violations, prefix+"status must be active, pending-upstream, retiring, or retired")
	}
	if f.Status == "retired" && (len(f.OwnedFiles) != 0 || len(f.OwnedPrefixes) != 0 || len(f.MixedFiles) != 0 || len(f.Bridges) != 0) {
		violations = append(violations, prefix+"retired feature must not retain ownership claims")
	}
	factPath, anchor, ok := strings.Cut(f.FactSource, "#")
	if !ok || anchor == "" {
		violations = append(violations, prefix+"factSource must be a path plus #anchor")
	} else {
		if err := validateFilePath(factPath); err != nil {
			violations = append(violations, prefix+"factSource: "+err.Error())
		}
		if factPath != featureDocument {
			violations = append(violations, prefix+"factSource must point into featureDocument")
		}
		if expected := markdownSlug(f.ID); expected != "" && anchor != expected {
			violations = append(violations, fmt.Sprintf("%sfactSource anchor must be #%s", prefix, expected))
		}
	}

	for name, paths := range map[string][]string{
		"ownedFiles": f.OwnedFiles,
		"mixedFiles": f.MixedFiles,
	} {
		violations = append(violations, sortedUnique(prefix+name, paths)...)
		for _, file := range paths {
			if err := validateFilePath(file); err != nil {
				violations = append(violations, prefix+name+": "+err.Error())
			}
		}
	}
	violations = append(violations, sortedUnique(prefix+"ownedPrefixes", f.OwnedPrefixes)...)
	for _, ownedPrefix := range f.OwnedPrefixes {
		if err := validatePrefix(ownedPrefix); err != nil {
			violations = append(violations, prefix+"ownedPrefixes: "+err.Error())
		}
	}

	bridgePaths := make([]string, len(f.Bridges))
	mixed := stringSet(f.MixedFiles)
	for i, bridge := range f.Bridges {
		bridgePaths[i] = bridge.Path
		if err := validateFilePath(bridge.Path); err != nil {
			violations = append(violations, prefix+"bridge: "+err.Error())
		}
		if bridge.Marker == "" {
			violations = append(violations, prefix+"bridge marker must not be empty")
		}
		if !mixed[bridge.Path] {
			violations = append(violations, prefix+"bridge path "+bridge.Path+" must also be in mixedFiles")
		}
	}
	violations = append(violations, sortedUnique(prefix+"bridge paths", bridgePaths)...)
	return violations
}

func sortedUnique(name string, values []string) []string {
	var violations []string
	if !sort.StringsAreSorted(values) {
		violations = append(violations, name+" must be sorted")
	}
	seen := map[string]bool{}
	for _, value := range values {
		if seen[value] {
			violations = append(violations, fmt.Sprintf("%s contains duplicate %q", name, value))
		}
		seen[value] = true
	}
	return violations
}

func validateFilePath(file string) error {
	if file == "" || file == "." || file == ".." || path.IsAbs(file) || path.Clean(file) != file || strings.Contains(file, "\\") || strings.HasSuffix(file, "/") {
		return fmt.Errorf("invalid repository file path %q", file)
	}
	if strings.HasPrefix(file, "../") {
		return fmt.Errorf("path escapes repository: %q", file)
	}
	return nil
}

func validatePrefix(prefix string) error {
	if !strings.HasSuffix(prefix, "/") {
		return fmt.Errorf("prefix %q must end in /", prefix)
	}
	return validateFilePath(strings.TrimSuffix(prefix, "/"))
}

func checkSources(root string, m manifest) []string {
	var violations []string
	document, err := committedFile(root, m.FeatureDocument)
	if err != nil {
		violations = append(violations, fmt.Sprintf("featureDocument %s: %v", m.FeatureDocument, err))
		return violations
	}
	ids := make([]string, len(m.Features))
	for i, feature := range m.Features {
		ids[i] = feature.ID
	}
	if tableIDs := markdownFeatureTableIDs(document); strings.Join(tableIDs, "\x00") != strings.Join(ids, "\x00") {
		violations = append(violations, fmt.Sprintf("feature table IDs are %v, want exactly %v", tableIDs, ids))
	}
	if headings := markdownFeatureHeadings(document); strings.Join(headings, "\x00") != strings.Join(ids, "\x00") {
		violations = append(violations, fmt.Sprintf("feature headings are %v, want exactly %v", headings, ids))
	}
	statuses := markdownFeatureStatuses(document)
	for _, feature := range m.Features {
		if documented := statuses[feature.ID]; len(documented) != 1 || documented[0] != feature.Status {
			violations = append(violations, fmt.Sprintf("feature %s status in featureDocument must occur once and equal %s", feature.ID, feature.Status))
		}
		_, anchor, _ := strings.Cut(feature.FactSource, "#")
		headingAnchors := markdownExactHeadingAnchors(document, feature.ID)
		if len(headingAnchors) != 1 {
			violations = append(violations, fmt.Sprintf("feature %s heading occurs %d times, want exactly once", feature.ID, len(headingAnchors)))
		} else if headingAnchors[0] != anchor {
			violations = append(violations, fmt.Sprintf("feature %s factSource anchor #%s resolves to another heading; exact heading anchor is #%s", feature.ID, anchor, headingAnchors[0]))
		}
		for _, bridge := range feature.Bridges {
			content, err := committedFile(root, bridge.Path)
			if err != nil {
				violations = append(violations, fmt.Sprintf("feature %s bridge %s: %v", feature.ID, bridge.Path, err))
				continue
			}
			count := bytes.Count(content, []byte(bridge.Marker))
			if strings.EqualFold(path.Ext(bridge.Path), ".md") {
				count = markdownMarkerCount(content, bridge.Marker)
			}
			if count != 1 {
				violations = append(violations, fmt.Sprintf("feature %s bridge marker occurs %d times in %s, want exactly once", feature.ID, count, bridge.Path))
			}
		}
	}
	return violations
}

func committedFile(root, file string) ([]byte, error) {
	return gitOutput(root, "show", "HEAD:"+file)
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func gitOutput(root string, args ...string) ([]byte, error) {
	commandArgs := append([]string{"-C", root}, args...)
	command := exec.Command("git", commandArgs...)
	output, err := command.Output()
	if err == nil {
		return output, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		message := strings.TrimSpace(string(exitErr.Stderr))
		if message != "" {
			return nil, errors.New(message)
		}
	}
	return nil, err
}

func gitRun(root string, args ...string) error {
	_, err := gitOutput(root, args...)
	return err
}
