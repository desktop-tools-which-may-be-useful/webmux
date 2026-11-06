package sessioncmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/rainchan/webmux/internal/agentguide"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/localclient"
)

func topicNames() []string {
	names := make([]string, 0, len(agentguide.Topics))
	for _, t := range agentguide.Topics {
		names = append(names, t.Name)
	}
	return names
}

var guideCommand = &Command{
	Name: "guide", Group: GroupGuide, Args: "[TOPIC]",
	Summary: "Print the WebMux shell guide for scripts and agents (the llms.txt index, a topic, or everything).",
	Description: "The guide is served by the running server at /llms.txt, /llms/<topic>.md and /llms-full.txt; " +
		"this command reads it over the local socket, so it matches the server's version and needs no token. " +
		"When the server is not running, the copy built into this executable is printed with a note on standard error. " +
		"The guide holds concepts, how to choose a command and cross-command recipes; its commands topic lists every " +
		"command in one line, and each command's options stay in its --help.",
	Arguments: []Item{{Name: "TOPIC", Text: "One of: " + strings.Join(topicNames(), ", ") + "."}},
	Env: []Item{
		{Name: "WEBMUX_SOCKET", Text: "Local API socket used when neither --socket nor --config is given."},
	},
	Output: "Markdown. With --path, two lines: the server's executable (WEBMUX_BIN) and its local API socket.",
	Exit: []ExitItem{
		{Code: 0, Text: "Printed (also from the built-in copy when the server is not running)."},
		{Code: 1, Text: "The server knows no executable (--path)."},
		{Code: 2, Text: "Unknown topic or usage error."},
		{Code: 3, Text: "WebMux is not running (--path)."},
	},
	Examples: []Example{
		{Command: "webmux guide", Text: "Print the index of the guide."},
		{Command: "webmux guide artifacts", Text: "Read where to put files for the user."},
		{Command: "webmux guide --full > ~/.claude/skills/webmux/reference.md", Text: "Save the whole guide as a reference for a skill."},
	},
	SeeAlso: []string{"webmux help", "webmux dir"},
	Notes: map[string]Note{
		"full": {Conflicts: []string{"path"}},
		"path": {Conflicts: []string{"full"}},
	},
	Bind: func(fs *flag.FlagSet) Action {
		full := fs.Bool("full", false, "print every topic after the index (/llms-full.txt)")
		path := fs.Bool("path", false, "print the server's executable and local API socket")
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) > 1 {
				return e.Usagef("at most one topic is allowed")
			}
			if *full && *path {
				return e.Usagef("--full and --path cannot be combined")
			}
			if (*full || *path) && len(args) == 1 {
				return e.Usagef("a topic cannot be combined with --full or --path")
			}
			if *path {
				return e.guidePath(ctx)
			}
			page := "/llms.txt"
			if *full {
				page = "/llms-full.txt"
			}
			if len(args) == 1 {
				if !slices.Contains(topicNames(), args[0]) {
					return e.Usagef("unknown topic %q (topics: %s)", args[0], strings.Join(topicNames(), ", "))
				}
				page = "/llms/" + args[0] + ".md"
			}
			return e.guide(ctx, page, args, *full)
		}
	},
}

func (e Env) guide(ctx context.Context, page string, args []string, full bool) int {
	client, err := e.Connect()
	var text []byte
	if err == nil {
		defer client.Close()
		text, err = client.Get(ctx, page)
	}
	if err != nil {
		var apiErr *localclient.APIError
		// An older server answers the unknown route with a bare 404.
		olderServer := errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound && apiErr.Message == ""
		if !olderServer && !errors.Is(err, localclient.ErrNotRunning) {
			return e.Fail(err)
		}
		// A stopped or older server: print the copy built into this CLI.
		_, _ = fmt.Fprintln(e.Stderr, "webmux guide: the server did not serve the guide; printing the copy built into this executable, which may differ from the server's version")
		guide := agentguide.New(Directory(e.registry()))
		switch {
		case full:
			text = []byte(guide.Full(nil, ""))
		case len(args) == 1:
			body, _ := guide.Page(args[0])
			text = []byte(body)
		default:
			text = []byte(guide.Index(nil, ""))
		}
	}
	_, _ = e.Stdout.Write(text)
	return contract.ExitOK
}

func (e Env) guidePath(ctx context.Context) int {
	client, code := e.connect()
	if client == nil {
		return code
	}
	defer client.Close()
	status, err := client.Status(ctx)
	if err != nil {
		return e.Fail(err)
	}
	_, _ = fmt.Fprintf(e.Stdout, "%s\n%s\n", status.Bin, status.Socket)
	if status.Bin == "" {
		return contract.ExitError
	}
	return contract.ExitOK
}

var dirCommand = &Command{
	Name: "dir", Group: GroupSessions, Args: "[SESSION]",
	Summary: "Print a session's artifacts folder, creating it when needed.",
	Description: "The artifacts folder is where the user and the programs of a session exchange files (webmux guide artifacts " +
		"has the rules). Inside a session SESSION defaults to self.",
	Arguments: []Item{sessionArgument},
	Env:       []Item{sessionIDEnv},
	Output:    "The absolute path, unquoted.",
	Exit: []ExitItem{
		{Code: 0, Text: "Printed."},
		{Code: 1, Text: "No such session, or the folder is unavailable."},
		{Code: 2, Text: "Usage error, or no SESSION outside a session."},
		{Code: 3, Text: "WebMux is not running."},
	},
	Examples: []Example{
		{Command: `cp report.html "$(webmux dir self)/"`, Text: "Hand a report to the user."},
		{Command: "ls \"$(webmux dir)/uploads\"", Text: "List the files the user uploaded to this session."},
	},
	SeeAlso: []string{"webmux guide artifacts", "webmux get"},
	Bind: func(fs *flag.FlagSet) Action {
		return func(ctx context.Context, e Env, args []string) int {
			if len(args) > 1 {
				return e.Usagef("at most one session ID is allowed")
			}
			arg := "self"
			if len(args) == 1 {
				arg = args[0]
			} else if e.Getenv == nil || e.Getenv(contract.SessionIDEnv) == "" {
				return e.Usagef("a session ID is required outside a WebMux session")
			}
			id, code, ok := e.SessionID(arg)
			if !ok {
				return code
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			var folder contract.ArtifactsFolder
			if err := client.Do(ctx, http.MethodPost, "/api/sessions/"+url.PathEscape(id)+"/artifacts", nil, &folder); err != nil {
				return e.Fail(err)
			}
			_, _ = fmt.Fprintln(e.Stdout, folder.Path)
			return contract.ExitOK
		}
	},
}
