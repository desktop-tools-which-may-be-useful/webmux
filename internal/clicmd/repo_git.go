package clicmd

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"text/tabwriter"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

func branchesOf(ctx context.Context, client *localclient.Client, repo contract.Repository) (contract.Branches, error) {
	var branches contract.Branches
	err := client.Do(ctx, http.MethodGet, "/api/repositories/"+url.PathEscape(repo.ID)+"/branches", nil, &branches)
	return branches, err
}

var repoBranchesCommand = &Command{
	Name: "repo branches", Group: GroupWorkspace, Args: "REPO",
	Summary:   "List a repository's local branches, their commits and the worktrees that check them out.",
	Arguments: []Item{repoArgument},
	Output: "A table: BRANCH, HEAD, WORKTREES, with the default base and branch on standard error. With --json, the Branches " +
		"object (branches, bases, defaultBase, defaultBranch, defaultParent).",
	Exit: exitsCommon(ExitItem{Code: 2, Text: "Also: REPO names nothing."}),
	Examples: []Example{
		{Command: "webmux repo branches .", Text: "See which branches are free for a new worktree."},
		{Command: "webmux repo branches . --json | jq -r .defaultBase", Text: "Read the default base."},
	},
	SeeAlso: []string{"webmux repo commits", "webmux worktree create"},
	Bind: func(fs *flag.FlagSet) Action {
		asJSON := fs.Bool("json", false, "print the Branches object")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 1 {
				return e.Usagef("exactly one repository is required")
			}
			client, repo, code := e.repoAndClient(ctx, args[0])
			if client == nil {
				return code
			}
			defer client.Close()
			branches, err := branchesOf(ctx, client, repo)
			if err != nil {
				return e.Fail(err)
			}
			if *asJSON {
				writeJSON(e.Stdout, branches)
				return contract.ExitOK
			}
			if branches.DefaultBase != nil {
				_, _ = fmt.Fprintf(e.Stderr, "webmux: default base %s %s\n", shortHash(*branches.DefaultBase), branches.DefaultBaseSubject)
			}
			if branches.DefaultBranch != nil {
				_, _ = fmt.Fprintf(e.Stderr, "webmux: default branch %s\n", *branches.DefaultBranch)
			}
			w := tabwriter.NewWriter(e.Stdout, 0, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "BRANCH\tHEAD\tWORKTREES")
			for _, b := range branches.Branches {
				trees := strings.Join(b.WorktreePaths, ", ")
				if trees == "" {
					trees = "-"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", b.Name, shortHash(b.Head), trees)
			}
			_ = w.Flush()
			return contract.ExitOK
		}
	},
}

func shortHash(hash string) string {
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}

// isObjectID reports a full hexadecimal commit ID (SHA-1 or SHA-256).
func isObjectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

var repoCommitsCommand = &Command{
	Name: "repo commits", Group: GroupWorkspace, Args: "REPO",
	Summary: "List recent commits, to pick a --base for a new worktree.",
	Description: "--from names where the history starts: a local branch, a base the repository offers, or a full commit ID; " +
		"at most two. Without it, the history of the default base is listed.",
	Arguments: []Item{repoArgument},
	Output:    "Lines \"HASH<TAB>DATE<TAB>AUTHOR<TAB>SUBJECT\"; with --json, the Commits object.",
	Exit:      exitsCommon(ExitItem{Code: 2, Text: "Also: REPO or a --from names nothing."}),
	Examples: []Example{
		{Command: "webmux repo commits .", Text: "See the latest commits of the default base."},
		{Command: "webmux repo commits . --from main --from feat/x --json", Text: "Compare two branches' recent history."},
	},
	SeeAlso: []string{"webmux repo branches", "webmux worktree create"},
	Notes:   map[string]Note{"from": {Repeatable: true}},
	Bind: func(fs *flag.FlagSet) Action {
		var from stringsFlag
		fs.Var(&from, "from", "start the history at `REF` (branch, base or full commit ID)")
		asJSON := fs.Bool("json", false, "print the Commits object")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 1 {
				return e.Usagef("exactly one repository is required")
			}
			if len(from) > 2 {
				return e.Usagef("at most two --from are allowed")
			}
			client, repo, code := e.repoAndClient(ctx, args[0])
			if client == nil {
				return code
			}
			defer client.Close()
			branches, err := branchesOf(ctx, client, repo)
			if err != nil {
				return e.Fail(err)
			}
			if len(from) == 0 && branches.DefaultBase != nil {
				from = append(from, *branches.DefaultBase)
			}
			query := url.Values{}
			for _, ref := range from {
				oid, ok := resolveRef(branches, ref)
				if !ok {
					return e.Usagef("unknown ref %q: give a local branch, a base from webmux repo branches, or a full commit ID", ref)
				}
				query.Add("from", oid)
			}
			if len(query) == 0 {
				return e.Usagef("the repository has no default base; give --from")
			}
			var commits contract.Commits
			if err := client.Do(ctx, http.MethodGet, "/api/repositories/"+url.PathEscape(repo.ID)+"/commits?"+query.Encode(), nil, &commits); err != nil {
				return e.Fail(err)
			}
			if *asJSON {
				writeJSON(e.Stdout, commits)
				return contract.ExitOK
			}
			for _, c := range commits.Commits {
				_, _ = fmt.Fprintf(e.Stdout, "%s\t%s\t%s\t%s\n", shortHash(c.Hash), c.CommittedAt, c.Author, c.Subject)
			}
			return contract.ExitOK
		}
	},
}

// resolveRef maps a branch or base name to its commit; a full commit ID
// stands for itself.
func resolveRef(branches contract.Branches, ref string) (string, bool) {
	if isObjectID(ref) {
		return ref, true
	}
	for _, b := range branches.Branches {
		if b.Name == ref {
			return b.Head, true
		}
	}
	for _, b := range branches.Bases {
		if b.Name == ref {
			return b.Head, true
		}
	}
	return "", false
}

var repoBaseCommand = &Command{
	Name: "repo base", Group: GroupWorkspace, Args: "REPO BRANCH",
	Summary: "Fetch a branch's upstream and report the base a new worktree would start from.",
	Description: "When the upstream is ahead, the base is the fetched commit; when the local branch is ahead or diverged, or " +
		"fetching fails, it stays local. A fetch failure is a warning on standard error and does not change the exit status.",
	Arguments: []Item{repoArgument, {Name: "BRANCH", Text: "A local branch with an upstream, such as main."}},
	Output:    "\"BASE<TAB>SOURCE<TAB>SUBJECT\"; with --json, the RefreshBranchBaseResponse.",
	Exit:      exitsCommon(ExitItem{Code: 2, Text: "Also: REPO names nothing."}),
	Examples: []Example{
		{Command: "webmux repo base . main", Text: "Check whether origin/main has news before branching."},
		{Command: "webmux worktree create . --branch fix/x --base \"$(webmux repo base . main | cut -f1)\"", Text: "Branch from the fetched base."},
	},
	SeeAlso: []string{"webmux worktree create", "webmux repo branches"},
	Bind: func(fs *flag.FlagSet) Action {
		asJSON := fs.Bool("json", false, "print the RefreshBranchBaseResponse")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) != 2 {
				return e.Usagef("a repository and a branch are required")
			}
			client, repo, code := e.repoAndClient(ctx, args[0])
			if client == nil {
				return code
			}
			defer client.Close()
			var result contract.RefreshBranchBaseResponse
			if err := client.Do(ctx, http.MethodPost, "/api/repositories/"+url.PathEscape(repo.ID)+"/branches/refresh",
				contract.RefreshBranchBaseRequest{Branch: args[1]}, &result); err != nil {
				return e.Fail(err)
			}
			if result.FetchWarning != "" {
				_, _ = fmt.Fprintln(e.Stderr, "webmux: fetching failed; using the local branch:", result.FetchWarning)
			}
			if *asJSON {
				writeJSON(e.Stdout, result)
			} else {
				_, _ = fmt.Fprintf(e.Stdout, "%s\t%s\t%s\n", result.Base, result.BaseSource, result.BaseSubject)
			}
			return contract.ExitOK
		}
	},
}
