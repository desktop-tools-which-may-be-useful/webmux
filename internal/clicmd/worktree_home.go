package clicmd

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"slices"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

// Home page display states of a worktree, as the Web UI decides them.
const (
	homePinned   = "pinned"
	homeSessions = "sessions"
	homeHidden   = "hidden"
)

// home is what the home page display of every repository depends on: the
// homeWorktrees setting, active sessions, and archived (closed or
// interrupted) sessions, each placed as the web home page places them.
type home struct {
	pins     map[string][]string
	sessions map[string][]contract.SessionInfo // by worktree ID
	archived map[string][]contract.SessionInfo // by worktree ID
}

func loadHome(ctx context.Context, client *localclient.Client) (home, error) {
	h := home{sessions: map[string][]contract.SessionInfo{}, archived: map[string][]contract.SessionInfo{}}
	var snapshot contract.SettingsSnapshot
	if err := client.Do(ctx, http.MethodGet, "/api/settings", nil, &snapshot); err != nil {
		return h, err
	}
	h.pins = snapshot.Values.HomeWorktrees
	var sessions []contract.SessionInfo
	if err := client.Do(ctx, http.MethodGet, "/api/sessions", nil, &sessions); err != nil {
		return h, err
	}
	for _, s := range sessions {
		if s.Workspace != nil {
			h.sessions[s.Workspace.WorktreeID] = append(h.sessions[s.Workspace.WorktreeID], s)
		}
	}
	var archived []contract.SessionInfo
	if err := client.Do(ctx, http.MethodGet, "/api/sessions?state=archived", nil, &archived); err != nil || len(archived) == 0 {
		return h, nil
	}
	var repositories []contract.Repository
	if err := client.Do(ctx, http.MethodGet, "/api/repositories", nil, &repositories); err != nil {
		return h, err
	}
	for _, s := range archived {
		if id, ok := archivedWorktree(s, repositories); ok {
			h.archived[id] = append(h.archived[id], s)
		}
	}
	return h, nil
}

// archivedWorktree is the web's archived session placement: the recorded
// worktree wins; a session recorded before associations were kept joins
// the registered worktree with the longest path containing its cwd.
func archivedWorktree(s contract.SessionInfo, repositories []contract.Repository) (string, bool) {
	if s.Workspace != nil {
		return s.Workspace.WorktreeID, true
	}
	_, tree, ok := findWorktree(repositories, s.Cwd)
	return tree.ID, ok
}

// pinned is the web's pinnedIds: the curated list, or the main worktree.
func (h home) pinned(repo contract.Repository) ([]string, bool) {
	list, curated := h.pins[repo.ID]
	if !curated {
		return mainWorktrees(repo), false
	}
	return list, true
}

// status is the web's home layout for one worktree: pinned, shown for its
// sessions or archived records, or hidden.
func (h home) status(repo contract.Repository, tree contract.Worktree) string {
	list, _ := h.pinned(repo)
	switch {
	case slices.Contains(list, tree.ID):
		return homePinned
	case len(h.sessions[tree.ID]) > 0 || len(h.archived[tree.ID]) > 0:
		return homeSessions
	}
	return homeHidden
}

// worktreeView is a Worktree with its home page state and session count.
type worktreeView struct {
	contract.Worktree
	Home     string `json:"home"`
	Sessions int    `json:"sessions"`
}

type repositoryView struct {
	contract.Repository
	Worktrees []worktreeView `json:"worktrees"`
}

func (h home) view(repo contract.Repository) repositoryView {
	v := repositoryView{Repository: repo, Worktrees: make([]worktreeView, 0, len(repo.Worktrees))}
	for _, t := range repo.Worktrees {
		v.Worktrees = append(v.Worktrees, worktreeView{Worktree: t, Home: h.status(repo, t), Sessions: len(h.sessions[t.ID])})
	}
	return v
}

// pinCommand builds worktree pin and unpin.
func pinCommand(name string, pinned bool, summary string, examples []Example) *Command {
	return &Command{
		Name: "worktree " + name, Group: GroupWorkspace, Args: "REPO WORKTREE...",
		Summary: summary,
		Description: "Only this repository's entry of the homeWorktrees setting changes, and every open browser follows. A " +
			"conflicting write from another client is read again and the change replayed, at most three times. A worktree " +
			"with sessions or archived sessions stays on the home page whether or not it is added.",
		Arguments: []Item{repoArgument, worktreeArgument},
		Exit:      exitsCommon(ExitItem{Code: 1, Text: "Also: the setting kept conflicting."}, ExitItem{Code: 2, Text: "Also: REPO or WORKTREE names nothing."}),
		Examples:  examples,
		SeeAlso:   []string{"webmux worktree home", "webmux worktree ls"},
		Bind: func(fs *flag.FlagSet) Action {
			return func(ctx context.Context, e Env, args []string) int {
				if len(args) < 2 {
					return e.Usagef("a repository and at least one worktree are required")
				}
				client, repo, code := e.repoAndClient(ctx, args[0])
				if client == nil {
					return code
				}
				defer client.Close()
				var ids []string
				for _, arg := range args[1:] {
					tree, err := e.worktreeIn(ctx, repo, arg)
					if err != nil {
						return e.locationError(err)
					}
					ids = append(ids, tree.ID)
				}
				if err := pinWorktrees(ctx, client, repo, ids, pinned); err != nil {
					return e.Fail(err)
				}
				if !pinned {
					if h, err := loadHome(ctx, client); err == nil {
						for _, id := range ids {
							if len(h.sessions[id]) > 0 || len(h.archived[id]) > 0 {
								_, _ = fmt.Fprintf(e.Stderr, "webmux: %s stays on the home page while it has sessions or archived sessions\n", id)
							}
						}
					}
				}
				return contract.ExitOK
			}
		},
	}
}

var (
	worktreePinCommand = pinCommand("pin", true, "Add worktrees to the home page.", []Example{
		{Command: "webmux worktree pin . exp/a", Text: "Show a worktree on every browser's home page."},
		{Command: "webmux worktree pin . exp/a exp/b", Text: "Add several at once."},
	})
	worktreeUnpinCommand = pinCommand("unpin", false, "Take worktrees off the home page.", []Example{
		{Command: "webmux worktree unpin . exp/a", Text: "Hide a worktree you are done with (it stays while it has sessions)."},
		{Command: "webmux worktree ls . --home --json | jq -r '.[].worktrees[] | select(.kind != \"main\") | .id' | xargs webmux worktree unpin .", Text: "Leave only the main worktree."},
	})
)

var worktreeHomeCommand = &Command{
	Name: "worktree home", Group: GroupWorkspace, Args: "REPO [WORKTREE...]",
	Summary: "Show or replace which worktrees of a repository are added to the home page.",
	Description: "Without options, the added worktrees are listed and marked as the default when nobody curated the repository " +
		"(only the main worktree). --set replaces the list with the given worktrees; --reset deletes the repository's " +
		"entry, so the home page shows its main worktree again, plus worktrees with sessions or archived records.",
	Arguments: []Item{repoArgument, {Name: "WORKTREE", Text: "With --set, the worktrees to add (ID, branch or path)."}},
	Output:    "Lines \"ID<TAB>BRANCH<TAB>PATH\"; with --json, {repositoryId, default, worktrees}.",
	Exit:      exitsCommon(ExitItem{Code: 1, Text: "Also: the setting kept conflicting."}),
	Examples: []Example{
		{Command: "webmux worktree home .", Text: "See which worktrees the home page shows."},
		{Command: "webmux worktree home . --reset", Text: "Go back to showing only the main worktree."},
	},
	SeeAlso: []string{"webmux worktree pin", "webmux worktree ls"},
	Notes:   map[string]Note{"set": {Conflicts: []string{"reset"}}, "reset": {Conflicts: []string{"set"}}},
	Bind: func(fs *flag.FlagSet) Action {
		set := fs.Bool("set", false, "replace the list with the WORKTREE arguments")
		reset := fs.Bool("reset", false, "delete the repository's entry (show the main worktree)")
		asJSON := fs.Bool("json", false, "print {repositoryId, default, worktrees}")
		return func(ctx context.Context, e Env, args []string) int {
			switch {
			case len(args) < 1:
				return e.Usagef("a repository is required")
			case *set && *reset:
				return e.Usagef("--set and --reset cannot be combined")
			case len(args) > 1 && !*set:
				return e.Usagef("worktrees are taken only with --set")
			}
			client, repo, code := e.repoAndClient(ctx, args[0])
			if client == nil {
				return code
			}
			defer client.Close()
			switch {
			case *set:
				ids := []string{}
				for _, arg := range args[1:] {
					tree, err := e.worktreeIn(ctx, repo, arg)
					if err != nil {
						return e.locationError(err)
					}
					ids = append(ids, tree.ID)
				}
				err := updateHome(ctx, client, repo, func([]string, bool) ([]string, bool) { return ids, true })
				if err != nil {
					return e.Fail(err)
				}
			case *reset:
				if err := updateHome(ctx, client, repo, func([]string, bool) ([]string, bool) { return nil, false }); err != nil {
					return e.Fail(err)
				}
			}
			h, err := loadHome(ctx, client)
			if err != nil {
				return e.Fail(err)
			}
			ids, curated := h.pinned(repo)
			trees := []contract.Worktree{}
			for _, t := range repo.Worktrees {
				if slices.Contains(ids, t.ID) {
					trees = append(trees, t)
				}
			}
			if *asJSON {
				writeJSON(e.Stdout, map[string]any{"repositoryId": repo.ID, "default": !curated, "worktrees": trees})
				return contract.ExitOK
			}
			if !curated {
				_, _ = fmt.Fprintln(e.Stderr, "webmux: not curated; the home page shows the main worktree")
			}
			for _, t := range trees {
				_, _ = fmt.Fprintf(e.Stdout, "%s\t%s\t%s\n", t.ID, worktreeName(t), t.Path)
			}
			return contract.ExitOK
		}
	},
}
