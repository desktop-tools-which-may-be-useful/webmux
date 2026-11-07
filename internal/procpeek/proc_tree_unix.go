//go:build linux || darwin

package procpeek

// maxOpenFiles bounds the open files RunOpenFiles reports.
const maxOpenFiles = 4096

// RunChildren returns the executable base names of pid's direct children.
func RunChildren(pid int) []string {
	snapshot := NewSnapshot(Processes())
	var names []string
	for _, child := range snapshot.children[pid] {
		if exe, _ := RootProcess(child); exe != "" {
			names = append(names, exe)
		}
	}
	return names
}

// ProcessTree returns pid and its descendant processes, each once. Threads
// are not processes and are not listed.
func ProcessTree(pid int) []int {
	return NewSnapshot(Processes()).Tree(pid)
}

// RunOpenFiles returns the absolute paths of the regular files pid and its
// descendants hold open, each once, at most maxOpenFiles.
func RunOpenFiles(pid int) []string {
	pids := NewSnapshot(Processes()).Tree(pid)
	seen := map[string]bool{}
	var paths []string
	for _, p := range pids {
		for _, path := range processOpenFiles(p) {
			if len(paths) == maxOpenFiles {
				return paths
			}
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}
	return paths
}
