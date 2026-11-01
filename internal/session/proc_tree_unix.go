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

// runOpenFiles returns the absolute paths of the regular files pid and its
// descendants hold open, each once, at most maxOpenFiles.
func runOpenFiles(pid int) []string {
	snapshot := newProcessSnapshot(processes())
	pids := []int{pid}
	for i := 0; i < len(pids); i++ {
		pids = append(pids, snapshot.children[pids[i]]...)
	}
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
