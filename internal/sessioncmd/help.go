package sessioncmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
)

// helpWidth is the column help text wraps at.
const helpWidth = 80

// Option describes one flag in help and help --json.
type Option struct {
	Name        string   `json:"name"`
	Short       string   `json:"short,omitempty"`
	Type        string   `json:"type"`
	Default     string   `json:"default"`
	Repeatable  bool     `json:"repeatable,omitempty"`
	Description string   `json:"description"`
	Conflicts   []string `json:"conflicts,omitempty"`
	Requires    []string `json:"requires,omitempty"`
	Global      bool     `json:"global,omitempty"`
}

// Options lists c's flags as help shows them: one-letter aliases fold into
// their long flag, and the client flags come last, marked global.
func (e Env) Options(c *Command) []Option {
	var own, global []Option
	ownNames := map[string]bool{}
	if c.Bind != nil {
		fs := flag.NewFlagSet(c.Name, flag.ContinueOnError)
		c.Bind(fs)
		fs.VisitAll(func(f *flag.Flag) { ownNames[f.Name] = true })
		own = options(fs, c.Notes, false)
	}
	if !c.NoGlobal && e.Flags != nil && c.Bind != nil {
		fs := flag.NewFlagSet(c.Name, flag.ContinueOnError)
		e.Flags(fs)
		for _, o := range options(fs, nil, true) {
			if !ownNames[o.Name] {
				global = append(global, o)
			}
		}
	}
	return append(own, global...)
}

func options(fs *flag.FlagSet, notes map[string]Note, global bool) []Option {
	shorts := map[string]string{}
	for name, note := range notes {
		if note.AliasOf != "" {
			shorts[note.AliasOf] = name
		}
	}
	var out []Option
	fs.VisitAll(func(f *flag.Flag) {
		note := notes[f.Name]
		if note.AliasOf != "" {
			return
		}
		valueName, usage := flag.UnquoteUsage(f)
		kind := valueName
		if isBool(f) {
			kind = "bool"
		} else if kind == "" {
			kind = "value"
		}
		out = append(out, Option{
			Name: f.Name, Short: shorts[f.Name], Type: kind, Default: f.DefValue,
			Repeatable: note.Repeatable, Description: usage,
			Conflicts: note.Conflicts, Requires: note.Requires, Global: global,
		})
	})
	return out
}

func isBool(f *flag.Flag) bool {
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

// globalFlags is how help names the client flags of c, empty when none.
func (e Env) globalFlags(c *Command) []Option {
	var out []Option
	for _, o := range e.Options(c) {
		if o.Global {
			out = append(out, o)
		}
	}
	return out
}

func (e Env) registry() Registry {
	if e.Commands != nil {
		return e.Commands
	}
	return allCommands()
}

// allCommands is Commands, reached through a variable: commands that render
// the table (guide) would otherwise make an initialization cycle.
var allCommands func() Registry

func init() { allCommands = Commands }

// PrintHelp writes c's help to standard output.
func (e Env) PrintHelp(c *Command) {
	renderHelp(e.Stdout, e.registry(), c, e.Options(c))
}

func renderHelp(w io.Writer, r Registry, c *Command, opts []Option) {
	var b strings.Builder
	fmt.Fprintf(&b, "Usage: %s\n\n", c.Usage())
	b.WriteString(wrap(c.Summary, "", helpWidth))
	for _, paragraph := range strings.Split(c.Description, "\n\n") {
		if strings.TrimSpace(paragraph) != "" {
			b.WriteString("\n" + wrap(paragraph, "", helpWidth))
		}
	}
	if children := r.Children(c.Name); len(children) > 0 {
		b.WriteString("\nCommands:\n")
		writeItems(&b, commandItems(children))
	}
	if len(c.Arguments) > 0 {
		b.WriteString("\nArguments:\n")
		writeItems(&b, c.Arguments)
	}
	if len(opts) > 0 {
		b.WriteString("\nOptions:\n")
		writeItems(&b, optionItems(opts))
	}
	if len(c.Env) > 0 {
		b.WriteString("\nEnvironment:\n")
		writeItems(&b, c.Env)
	}
	if c.Output != "" {
		b.WriteString("\nOutput:\n")
		b.WriteString(wrap(c.Output, "  ", helpWidth))
	}
	if len(c.Exit) > 0 {
		b.WriteString("\nExit status:\n")
		items := make([]Item, 0, len(c.Exit))
		for _, x := range c.Exit {
			items = append(items, Item{Name: fmt.Sprint(x.Code), Text: x.Text})
		}
		writeItems(&b, items)
	}
	if len(c.Examples) > 0 {
		b.WriteString("\nExamples:\n")
		for _, x := range c.Examples {
			b.WriteString("  " + x.Command + "\n")
			b.WriteString(wrap(x.Text, "      ", helpWidth))
		}
	}
	if len(c.SeeAlso) > 0 {
		b.WriteString("\nSee also:\n")
		b.WriteString(wrap(strings.Join(c.SeeAlso, ", "), "  ", helpWidth))
	}
	_, _ = io.WriteString(w, b.String())
}

func commandItems(cmds []*Command) []Item {
	items := make([]Item, 0, len(cmds))
	for _, c := range cmds {
		items = append(items, Item{Name: c.Name, Text: c.Summary})
	}
	return items
}

func optionItems(opts []Option) []Item {
	items := make([]Item, 0, len(opts))
	for _, o := range opts {
		name := "--" + o.Name
		if o.Short != "" {
			name = "-" + o.Short + ", " + name
		}
		if o.Type != "bool" {
			name += " " + o.Type
		}
		text := sentence(o.Description)
		if o.Default != "" && o.Default != "false" && o.Default != "0" && o.Default != "0s" && o.Type != "bool" {
			text += fmt.Sprintf(" Default: %s.", o.Default)
		}
		if o.Repeatable {
			text += " Repeatable."
		}
		if len(o.Conflicts) > 0 {
			text += " Conflicts with " + dashed(o.Conflicts) + "."
		}
		if len(o.Requires) > 0 {
			text += " Requires " + dashed(o.Requires) + "."
		}
		if o.Global {
			text += " Global."
		}
		items = append(items, Item{Name: name, Text: text})
	}
	return items
}

// sentence capitalizes text and ends it with a period, so flag usage
// strings written in the flag package's lower-case style read as sentences
// next to the notes appended to them.
func sentence(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	text = strings.ToUpper(text[:1]) + text[1:]
	if !strings.HasSuffix(text, ".") && !strings.HasSuffix(text, ")") {
		text += "."
	}
	return text
}

func dashed(names []string) string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, "--"+n)
	}
	return strings.Join(out, ", ")
}

// writeItems writes a two-column list: names that fit get the text on the
// same line, longer ones put it on the next line.
func writeItems(b *strings.Builder, items []Item) {
	const column = 24
	for _, it := range items {
		name := "  " + it.Name
		if len(name) < column-1 {
			text := wrap(it.Text, strings.Repeat(" ", column), helpWidth)
			b.WriteString(name + strings.Repeat(" ", column-len(name)) + strings.TrimLeft(text, " "))
			continue
		}
		b.WriteString(name + "\n")
		b.WriteString(wrap(it.Text, strings.Repeat(" ", column), helpWidth))
	}
}

// wrap word-wraps text at width with every line indented by indent. It
// never breaks a word, so a long path stays whole.
func wrap(text, indent string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	line := indent
	for _, word := range words {
		if len(line) > len(indent) && displayWidth(line)+1+displayWidth(word) > width {
			b.WriteString(line + "\n")
			line = indent
		}
		if len(line) > len(indent) {
			line += " "
		}
		line += word
	}
	b.WriteString(line + "\n")
	return b.String()
}

// displayWidth counts wide runes such as CJK as two columns.
func displayWidth(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x1100 && (r <= 0x115f || (r >= 0x2e80 && r <= 0xa4cf) || (r >= 0xac00 && r <= 0xd7a3) || (r >= 0xf900 && r <= 0xfaff) || (r >= 0xfe30 && r <= 0xfe4f) || (r >= 0xff00 && r <= 0xff60) || (r >= 0xffe0 && r <= 0xffe6)) {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// HelpDoc is one command in help --json.
type HelpDoc struct {
	Path        []string   `json:"path"`
	Group       string     `json:"group"`
	Usage       string     `json:"usage"`
	Summary     string     `json:"summary"`
	Description string     `json:"description,omitempty"`
	Args        []Item     `json:"args"`
	Options     []Option   `json:"options"`
	Env         []Item     `json:"env"`
	Output      string     `json:"output,omitempty"`
	ExitCodes   []ExitItem `json:"exitCodes"`
	Examples    []Example  `json:"examples"`
	SeeAlso     []string   `json:"seeAlso"`
}

// HelpTree is the help --json document.
type HelpTree struct {
	Version  string    `json:"version"`
	Commands []HelpDoc `json:"commands"`
}

// Docs describes the commands of r for help --json and generated
// references.
func (e Env) Docs(r Registry) []HelpDoc {
	docs := make([]HelpDoc, 0, len(r))
	for _, c := range r {
		docs = append(docs, HelpDoc{
			Path: strings.Fields(c.Name), Group: c.Group, Usage: c.Usage(), Summary: c.Summary,
			Description: c.Description, Args: nonNil(c.Arguments), Options: nonNil(e.Options(c)),
			Env: nonNil(c.Env), Output: c.Output, ExitCodes: nonNil(c.Exit), Examples: nonNil(c.Examples),
			SeeAlso: nonNil(c.SeeAlso),
		})
	}
	return docs
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// HelpCommand implements `help [--json] [COMMAND...]` for registry r.
func (e Env) HelpCommand(r Registry, version string, args []string) int {
	asJSON := false
	var path []string
	for _, arg := range args {
		switch {
		case arg == "--json" || arg == "-json":
			asJSON = true
		case isHelpFlag(arg):
		default:
			path = append(path, arg)
		}
	}
	visible := r.Visible(e.Platform)
	e.Commands = r
	if len(path) > 0 {
		c, rest := visible.Find(path)
		if c == nil || len(rest) > 0 {
			_, _ = fmt.Fprintf(e.Stderr, "webmux help: unknown command %q\nRun 'webmux help' for the list of commands.\n", strings.Join(path, " "))
			return 2
		}
		if asJSON {
			writeHelpJSON(e.Stdout, HelpTree{Version: version, Commands: e.Docs(visible.Under(c.Name))})
			return 0
		}
		e.PrintHelp(c)
		return 0
	}
	if asJSON {
		writeHelpJSON(e.Stdout, HelpTree{Version: version, Commands: e.Docs(visible)})
		return 0
	}
	PrintOverview(e.Stdout, visible)
	if e.HelpFooter != "" {
		_, _ = io.WriteString(e.Stdout, "\n"+wrap(e.HelpFooter, "", helpWidth))
	}
	return 0
}

func writeHelpJSON(w io.Writer, tree HelpTree) {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(tree)
}

// PrintOverview lists every top-level command of r by group.
func PrintOverview(w io.Writer, r Registry) {
	var b strings.Builder
	b.WriteString("Usage: webmux <command> [options] [arguments]\n\n")
	b.WriteString(wrap("WebMux keeps terminal sessions alive in the browser and on the command line. Run 'webmux <command> --help' for a command's options and examples, or 'webmux help --json' for the whole command tree.", "", helpWidth))
	for _, group := range Groups {
		var items []Item
		for _, c := range r {
			if c.Group != group || c.Hidden {
				continue
			}
			// Subcommands are listed when their parent is not a group.
			if parent := parentName(c.Name); parent != "" {
				if p, _ := r.Find(strings.Fields(parent)); p != nil && p.Name == parent && p.Bind == nil {
					continue
				}
			}
			items = append(items, Item{Name: c.Name, Text: c.Summary})
		}
		if len(items) == 0 {
			continue
		}
		slices.SortStableFunc(items, func(a, b Item) int { return strings.Compare(a.Name, b.Name) })
		b.WriteString("\n" + group + ":\n")
		writeItems(&b, items)
	}
	_, _ = io.WriteString(w, b.String())
}

func parentName(name string) string {
	if i := strings.LastIndex(name, " "); i >= 0 {
		return name[:i]
	}
	return ""
}

// Directory renders the command directory of the shell guide
// (/llms/commands.md) from the command table: every visible command that
// runs, one line each with its usage and summary, by group as webmux help
// lists them. Options, output and examples stay in --help.
func Directory(r Registry) string {
	var b strings.Builder
	b.WriteString("# 命令目录\n\n")
	b.WriteString("每个命令一行：用法与一句话摘要，与 `webmux help` 的概览同源（英文）。" +
		"选项、默认值、输出、退出码与示例用 `webmux <命令> --help` 查看，整棵命令树用 `webmux help --json`；" +
		"不要猜测选项。退出码约定见 [退出码](/llms/exit-codes.md)。\n")
	visible := r.Visible("unix")
	for _, group := range Groups {
		var cmds []*Command
		for _, c := range visible {
			if c.Group == group && c.Bind != nil {
				cmds = append(cmds, c)
			}
		}
		if len(cmds) == 0 {
			continue
		}
		slices.SortStableFunc(cmds, func(a, b *Command) int { return strings.Compare(a.Name, b.Name) })
		b.WriteString("\n## " + group + "\n\n")
		for _, c := range cmds {
			fmt.Fprintf(&b, "- `%s`: %s\n", c.Usage(), c.Summary)
		}
	}
	return b.String()
}
