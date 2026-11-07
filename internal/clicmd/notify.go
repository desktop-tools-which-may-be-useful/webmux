package clicmd

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/contract"
)

// maxNotifyMessage mirrors the server's limit on a notify message.
const maxNotifyMessage = 200

var notifyKinds = []string{"notify", "done", "error", "question"}

var notifyCommand = &Command{
	Name: "notify", Group: GroupSessions, Args: "[MESSAGE...]",
	Summary: "Raise attention on a session (badge, toast and push), or clear it with --clear.",
	Description: "The session shows the attention like any other: a badge in the list, a toast in open browsers and a push " +
		"notification on subscribed devices. It replaces whatever attention is latched, whatever raised it, and is then " +
		"cleared by the usual rules: the user typing or opening it, the program's next status report, or notify --clear.\n\n" +
		"MESSAGE words are joined with spaces. The message is kept only in memory and is shown as the toast detail and as the " +
		"push body (unless push.include_title is false, which also hides it). --clear removes the attention whatever raised " +
		"it, as user input does, on every device.",
	Arguments: []Item{{Name: "MESSAGE", Text: fmt.Sprintf("Optional text, at most %d characters after trimming, no control characters.", maxNotifyMessage)}},
	Env:       []Item{sessionIDEnv},
	Output:    "Nothing; with --json, the SessionInfo after the change (nothing for --clear).",
	Exit: exitsCommon(
		ExitItem{Code: 1, Text: "No such session, or the session has exited."},
	),
	Examples: []Example{
		{Command: `make release; webmux notify --kind done "release build finished"`, Text: "Tell the user a long task in this session is done."},
		{Command: `webmux notify --kind question "pick a migration plan in the terminal"`, Text: "Ask for the user's attention from an agent's tool."},
		{Command: "webmux notify --clear --session abcd2345", Text: "Clear a child session's attention after handling it."},
	},
	SeeAlso: []string{"webmux wait", "webmux get"},
	Notes: map[string]Note{
		"clear": {Conflicts: []string{"kind"}},
		"kind":  {Conflicts: []string{"clear"}},
	},
	Bind: func(fs *flag.FlagSet) Action {
		session := fs.String("session", "self", "the `SESSION` to notify; self is the current session")
		kind := fs.String("kind", "notify", "the attention `KIND`: "+strings.Join(notifyKinds, ", "))
		clearIt := fs.Bool("clear", false, "clear the session's attention instead of raising it")
		asJSON := fs.Bool("json", false, "print the SessionInfo after the change")
		return func(ctx context.Context, e Env, args []string) int {
			message := strings.TrimSpace(strings.Join(args, " "))
			kindSet := false
			fs.Visit(func(f *flag.Flag) { kindSet = kindSet || f.Name == "kind" })
			switch {
			case *clearIt && (kindSet || message != ""):
				return e.Usagef("--clear takes no --kind or MESSAGE")
			case !slices.Contains(notifyKinds, *kind):
				return e.Usagef("unknown kind %q (%s)", *kind, strings.Join(notifyKinds, ", "))
			case utf8.RuneCountInString(message) > maxNotifyMessage:
				return e.Usagef("the message has %d characters; at most %d are allowed", utf8.RuneCountInString(message), maxNotifyMessage)
			case strings.ContainsFunc(message, unicode.IsControl):
				return e.Usagef("the message cannot contain control characters such as newlines")
			}
			if *session == "self" && (e.Getenv == nil || e.Getenv(contract.SessionIDEnv) == "") {
				return e.Usagef("--session is required outside a WebMux session")
			}
			id, code, ok := e.SessionID(*session)
			if !ok {
				return code
			}
			client, code := e.connect()
			if client == nil {
				return code
			}
			defer client.Close()
			path := "/api/sessions/" + url.PathEscape(id) + "/attention"
			if *clearIt {
				if err := client.Do(ctx, http.MethodDelete, path, nil, nil); err != nil {
					return e.failNew(ctx, client, err)
				}
				return contract.ExitOK
			}
			var info contract.SessionInfo
			req := contract.AttentionRequest{Kind: contract.AttentionKind(*kind), Message: message}
			if err := client.Do(ctx, http.MethodPost, path, req, &info); err != nil {
				return e.failNew(ctx, client, err)
			}
			if *asJSON {
				writeJSON(e.Stdout, info)
			}
			return contract.ExitOK
		}
	},
}
