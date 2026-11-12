package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

func committedChanges(root, base string) ([]change, error) {
	output, err := gitOutput(root, "diff", "--name-status", "-z", "--no-renames", base+"..HEAD", "--")
	if err != nil {
		return nil, fmt.Errorf("list committed overlay: %w", err)
	}
	parts := bytes.Split(output, []byte{0})
	if len(parts) > 0 && len(parts[len(parts)-1]) == 0 {
		parts = parts[:len(parts)-1]
	}
	if len(parts)%2 != 0 {
		return nil, errors.New("unexpected git diff --name-status output")
	}
	changes := make([]change, 0, len(parts)/2)
	for i := 0; i < len(parts); i += 2 {
		changes = append(changes, change{Status: string(parts[i]), Path: string(parts[i+1])})
	}
	return changes, nil
}

func checkClaims(m manifest, changes []change) []string {
	ownedFiles := map[string][]string{}
	mixedFiles := map[string][]string{}
	type prefixClaim struct {
		feature string
		prefix  string
	}
	var ownedPrefixes []prefixClaim
	for _, feature := range m.Features {
		for _, file := range feature.OwnedFiles {
			ownedFiles[file] = append(ownedFiles[file], feature.ID)
		}
		for _, file := range feature.MixedFiles {
			mixedFiles[file] = append(mixedFiles[file], feature.ID)
		}
		for _, prefix := range feature.OwnedPrefixes {
			ownedPrefixes = append(ownedPrefixes, prefixClaim{feature: feature.ID, prefix: prefix})
		}
	}

	var violations []string
	for file, features := range ownedFiles {
		if len(features) > 1 {
			violations = append(violations, fmt.Sprintf("owned file %s is claimed by multiple features: %s", file, strings.Join(features, ", ")))
		}
	}
	for i, claim := range ownedPrefixes {
		for _, other := range ownedPrefixes[:i] {
			if strings.HasPrefix(claim.prefix, other.prefix) || strings.HasPrefix(other.prefix, claim.prefix) {
				violations = append(violations, fmt.Sprintf("owned prefixes overlap: %s (%s) and %s (%s)", other.prefix, other.feature, claim.prefix, claim.feature))
			}
		}
	}
	for file := range ownedFiles {
		if features := mixedFiles[file]; len(features) != 0 {
			violations = append(violations, fmt.Sprintf("%s is both owned and mixed", file))
		}
		for _, claim := range ownedPrefixes {
			if strings.HasPrefix(file, claim.prefix) {
				violations = append(violations, fmt.Sprintf("%s redundantly matches owned prefix %s", file, claim.prefix))
			}
		}
	}
	for file := range mixedFiles {
		for _, claim := range ownedPrefixes {
			if strings.HasPrefix(file, claim.prefix) {
				violations = append(violations, fmt.Sprintf("mixed file %s conflicts with owned prefix %s", file, claim.prefix))
			}
		}
	}

	matchedOwned := map[string]bool{}
	matchedMixed := map[string]bool{}
	matchedPrefixes := map[string]bool{}
	for _, change := range changes {
		switch change.Status {
		case "A":
			matched := false
			if len(ownedFiles[change.Path]) != 0 {
				matchedOwned[change.Path] = true
				matched = true
			}
			for _, claim := range ownedPrefixes {
				if strings.HasPrefix(change.Path, claim.prefix) {
					matchedPrefixes[claim.feature+"\x00"+claim.prefix] = true
					matched = true
				}
			}
			if !matched {
				if len(mixedFiles[change.Path]) != 0 {
					violations = append(violations, change.Path+" is added but classified as mixed")
				} else {
					violations = append(violations, "unclassified added path "+change.Path)
				}
			}
		case "M":
			if len(mixedFiles[change.Path]) != 0 {
				matchedMixed[change.Path] = true
			} else if len(ownedFiles[change.Path]) != 0 {
				violations = append(violations, change.Path+" is modified upstream content but classified as owned")
			} else {
				violations = append(violations, "unclassified modified path "+change.Path)
			}
		default:
			violations = append(violations, fmt.Sprintf("unsupported committed change %s %s", change.Status, change.Path))
		}
	}
	for file := range ownedFiles {
		if !matchedOwned[file] {
			violations = append(violations, "stale owned file claim "+file)
		}
	}
	for file := range mixedFiles {
		if !matchedMixed[file] {
			violations = append(violations, "stale mixed file claim "+file)
		}
	}
	for _, claim := range ownedPrefixes {
		if !matchedPrefixes[claim.feature+"\x00"+claim.prefix] {
			violations = append(violations, "stale owned prefix claim "+claim.feature+": "+claim.prefix)
		}
	}
	return violations
}
