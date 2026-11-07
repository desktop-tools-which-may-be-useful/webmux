//go:build !linux && !darwin

package procpeek

type State struct {
	PPID  int
	State byte
	SID   int
	Start uint64
	Timer bool
}

type Table map[int]State
type Snapshot struct{}

func Processes() Table                      { return nil }
func NewSnapshot(Table) Snapshot            { return Snapshot{} }
func (Snapshot) Children(int) []int         { return nil }
func (Snapshot) Tree(int) []int             { return nil }
func (Snapshot) ForegroundBlocked(int) bool { return false }
func (Snapshot) AwaitingInput(int) bool     { return false }
func RootProcess(int) (exe, cwd string)     { return "", "" }
func processFacts(int) (ProcessFacts, bool) { return ProcessFacts{}, false }
func RunChildren(int) []string              { return nil }
func RunOpenFiles(int) []string             { return nil }
func ProcessTree(int) []int                 { return nil }
func SessionMarked(int, []byte) bool        { return false }
