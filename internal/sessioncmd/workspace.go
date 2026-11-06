package sessioncmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"slices"
	"strings"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

// errRepoArgument marks a repository or worktree argument that names
// nothing or more than one thing: a usage error.
var errRepoArgument = errors.New("repository")

func repositories(ctx context.Context, client *localclient.Client) ([]contract.Repository, error) {
	var list []contract.Repository
	err := client.Do(ctx, http.MethodGet, "/api/repositories", nil, &list)
	return list, err
}

// within reports whether dir is root or below it. Both are server paths,
// which are POSIX paths whatever the client's OS (the Windows client talks
// to a server in WSL).
func within(root, dir string) bool {
	if root == "" || dir == "" {
		return false
	}
	root, dir = path.Clean(root), path.Clean(dir)
	return dir == root || root == "/" || strings.HasPrefix(dir, root+"/")
}

// findWorktree returns the registered worktree that contains dir, the
// deepest one when worktrees nest.
func findWorktree(list []contract.Repository, dir string) (contract.Repository, contract.Worktree, bool) {
	var repo contract.Repository
	var tree contract.Worktree
	found := false
	for _, r := range list {
		for _, t := range r.Worktrees {
			if within(t.Path, dir) && (!found || len(t.Path) > len(tree.Path)) {
				repo, tree, found = r, t, true
			}
		}
	}
	return repo, tree, found
}

// resolveRepository maps a REPO argument: "" or "." is the repository the
// current directory belongs to, otherwise an ID or a path inside one.
func (e Env) resolveRepository(ctx context.Context, list []contract.Repository, arg string) (contract.Repository, error) {
	names := make([]string, 0, len(list))
	for _, r := range list {
		names = append(names, fmt.Sprintf("%s (%s)", r.Name, r.ID))
		if r.ID == arg {
			return r, nil
		}
	}
	candidates := strings.Join(names, ", ")
	if candidates == "" {
		candidates = "none; register one with the Web UI or webmux repo add"
	}
	target := arg
	if target == "." {
		target = ""
	}
	dir, err := e.localPath(ctx, target)
	if err != nil {
		return contract.Repository{}, err
	}
	if repo, _, ok := findWorktree(list, dir); ok {
		return repo, nil
	}
	for _, r := range list {
		if within(r.AnchorPath, dir) {
			return r, nil
		}
	}
	if arg == "" || arg == "." {
		return contract.Repository{}, fmt.Errorf("%w: the current directory belongs to no registered repository (registered: %s)", errRepoArgument, candidates)
	}
	return contract.Repository{}, fmt.Errorf("%w: %q is no registered repository (registered: %s)", errRepoArgument, arg, candidates)
}

// resolveWorktree maps --worktree REPO:WORKTREE or a worktree path.
func (e Env) resolveWorktree(ctx context.Context, list []contract.Repository, arg string) (contract.Repository, contract.Worktree, error) {
	if repoArg, treeArg, ok := strings.Cut(arg, ":"); ok && !strings.Contains(repoArg, "/") && !isDriveLetter(repoArg) {
		repo, err := e.resolveRepository(ctx, list, repoArg)
		if err != nil {
			return contract.Repository{}, contract.Worktree{}, err
		}
		for _, t := range repo.Worktrees {
			if t.ID == treeArg || (t.Branch != nil && *t.Branch == treeArg) {
				return repo, t, nil
			}
		}
		return contract.Repository{}, contract.Worktree{}, fmt.Errorf("%w: repository %s has no worktree %q", errRepoArgument, repo.Name, treeArg)
	}
	dir, err := e.localPath(ctx, arg)
	if err != nil {
		return contract.Repository{}, contract.Worktree{}, err
	}
	repo, tree, ok := findWorktree(list, dir)
	if !ok {
		return contract.Repository{}, contract.Worktree{}, fmt.Errorf("%w: %s is in no registered worktree", errRepoArgument, dir)
	}
	return repo, tree, nil
}

// localPath resolves a directory argument the way --cwd does, and expands
// a leading ~ with $HOME so it can be compared with server paths.
func (e Env) localPath(ctx context.Context, arg string) (string, error) {
	dir, err := e.ResolveCwd(ctx, arg)
	if err != nil {
		return "", err
	}
	if rest, ok := strings.CutPrefix(dir, "~"); ok && e.Getenv != nil {
		if home := e.Getenv("HOME"); home != "" && (rest == "" || strings.HasPrefix(rest, "/")) {
			dir = home + rest
		}
	}
	return path.Clean(dir), nil
}

// isDriveLetter reports a Windows drive prefix such as C, so C:\src is a
// path rather than REPO:WORKTREE.
func isDriveLetter(s string) bool {
	return len(s) == 1 && (s[0] >= 'a' && s[0] <= 'z' || s[0] >= 'A' && s[0] <= 'Z')
}

// defaultWorktreePath is where quick start puts a new branch's worktree:
// beside the repository in <name>.worktrees/<branch leaf>.
func defaultWorktreePath(repo contract.Repository, branch string) string {
	anchor := path.Clean(repo.AnchorPath)
	return path.Join(path.Dir(anchor), path.Base(anchor)+".worktrees", path.Base(branch))
}

// pinWorktrees adds (or with pinned false removes) worktrees from the home
// page display of repo, the web's homeWorktrees setting. Only repo's entry
// changes; a conflicting write from another client is read again and the
// change replayed, at most three times.
func pinWorktrees(ctx context.Context, client *localclient.Client, repo contract.Repository, ids []string, pinned bool) error {
	return updateHome(ctx, client, repo, func(current []string, curated bool) ([]string, bool) {
		if !curated {
			current = mainWorktrees(repo)
		}
		next := slices.Clone(current)
		for _, id := range ids {
			has := slices.Contains(next, id)
			switch {
			case pinned && !has:
				next = append(next, id)
			case !pinned && has:
				next = slices.DeleteFunc(next, func(v string) bool { return v == id })
			}
		}
		return next, true
	})
}

// mainWorktrees is the web's default display of a repository nobody
// curated: its main worktree.
func mainWorktrees(repo contract.Repository) []string {
	var ids []string
	for _, t := range repo.Worktrees {
		if t.Kind == "main" {
			ids = append(ids, t.ID)
		}
	}
	return ids
}

// freshRepository reads repository id again; false when it cannot.
func freshRepository(ctx context.Context, client *localclient.Client, id string) (contract.Repository, bool) {
	list, err := repositories(ctx, client)
	if err != nil {
		return contract.Repository{}, false
	}
	i := slices.IndexFunc(list, func(r contract.Repository) bool { return r.ID == id })
	if i < 0 {
		return contract.Repository{}, false
	}
	return list[i], true
}

// updateHome rewrites repo's homeWorktrees entry with change, which gets
// the current list and whether the repository was ever curated, and
// returns the new list and false to delete the entry.
func updateHome(ctx context.Context, client *localclient.Client, repo contract.Repository, change func(current []string, curated bool) ([]string, bool)) error {
	for attempt := 0; ; attempt++ {
		var snapshot contract.SettingsSnapshot
		if err := client.Do(ctx, http.MethodGet, "/api/settings", nil, &snapshot); err != nil {
			return err
		}
		home := map[string][]string{}
		for id, list := range snapshot.Values.HomeWorktrees {
			home[id] = list
		}
		current, curated := home[repo.ID]
		next, keep := change(current, curated)
		if keep {
			// Drop pins of worktrees the repository no longer lists, as
			// the web does for a healthy snapshot. The list is read again
			// on every attempt, so a worktree another client created (and
			// pinned) meanwhile is kept; without a fresh list nothing is
			// dropped.
			if fresh, ok := freshRepository(ctx, client, repo.ID); ok && fresh.Availability == "available" && fresh.Error == "" {
				next = slices.DeleteFunc(slices.Clone(next), func(id string) bool {
					return !slices.ContainsFunc(fresh.Worktrees, func(t contract.Worktree) bool { return t.ID == id })
				})
			}
			home[repo.ID] = next
		} else {
			delete(home, repo.ID)
		}
		raw, err := json.Marshal(home)
		if err != nil {
			return err
		}
		revision := snapshot.Revision
		patch := contract.SettingsPatch{BaseRevision: &revision, Changes: contract.SettingsChanges{"homeWorktrees": raw}}
		err = client.Do(ctx, http.MethodPatch, "/api/settings", patch, nil)
		var apiErr *localclient.APIError
		if err == nil || !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict || attempt == 2 {
			return err
		}
	}
}
