//go:build linux || darwin

package session

// maxOpenFiles bounds the open files RunOpenFiles reports.
const maxOpenFiles = 4096

// runChildren returns the executable base names of pid's direct children.
func runChildren(pid int) []string {
	snapshot := newProcessSnapshot(processes())
	var names []string
	for _, child := range snapshot.children[pid] {
		if exe, _ := rootProcess(child); exe != "" {
			names = append(names, exe)
		}
	}
	return names
}

// ProcessTree returns pid and its descendant processes, each once. Threads
// are not processes and are not listed.
func ProcessTree(pid int) []int {
	return newProcessSnapshot(processes()).tree(pid)
}

// runOpenFiles returns the absolute paths of the regular files pid and its
// descendants hold open, each once, at most maxOpenFiles.
func runOpenFiles(pid int) []string {
	pids := newProcessSnapshot(processes()).tree(pid)
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
