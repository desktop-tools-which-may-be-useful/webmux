// Package agentguide is the WebMux shell guide for scripts and CLI agents,
// served llms.txt-style: an index at /llms.txt, one Markdown page per topic
// at /llms/<topic>.md and everything at /llms-full.txt. It also writes the
// short brief that {{webmux}} expands to. Pages are embedded and rendered
// in memory; nothing is written to disk.
package agentguide

import (
	"embed"
	"fmt"
	"strings"
	"sync"
)

//go:embed llms/*.md
var pages embed.FS

// Topic is one page of the guide.
type Topic struct {
	Name  string
	Title string
	// Summary follows the link in the index.
	Summary string
	// Optional topics are listed under "## Optional".
	Optional bool
	// Section groups the index.
	Section string
}

// Topics lists every page in index order; /llms-full.txt joins them in
// this order.
var Topics = []Topic{
	{Name: "connect", Title: "连接与身份", Section: "基础", Summary: "找到 webmux 可执行文件与套接字，会话中的变量，会话内被禁止的操作与排错。"},
	{Name: "sessions", Title: "会话", Section: "基础", Summary: "列出与详情、父会话与标签、输入与等待、输出与画面、结束与改名、附着与链接。"},
	{Name: "launch", Title: "启动会话", Section: "基础", Summary: "用 run 启动、跟随或等待会话，在新 worktree 中启动；用 chat new 一次完成快速开始。"},
	{Name: "artifacts", Title: "临时文件夹", Section: "基础", Summary: "交付文件放在哪里、上传文件在哪里，以及怎样告诉用户。"},
	{Name: "commands", Title: "命令参考", Section: "参考", Summary: "每个命令的用法、选项与默认值，由命令表生成。"},
	{Name: "exit-codes", Title: "退出码", Section: "参考", Summary: "退出码与输出约定。"},
	{Name: "rules", Title: "操作守则", Section: "参考", Summary: "操作其他会话、Token 与外部内容的规则。"},
	{Name: "recipes", Title: "常用流程", Optional: true, Summary: "分派任务给 agent、跑测试取退出码、后台任务、了解当前环境、交付文件、查看帮助的完整例子。"},
}

// Instance is what the local socket adds to the guide and the brief. The
// TCP listener serves the guide without it.
type Instance struct {
	Version string
	// LocalURL is the token-free local origin, such as http://127.0.0.1:7681.
	LocalURL string
	// Bin is the server's executable (WEBMUX_BIN); empty when unknown.
	Bin string

	mu     sync.Mutex
	socket string
}

// SetSocket records the local API socket once it listens.
func (i *Instance) SetSocket(path string) {
	i.mu.Lock()
	i.socket = path
	i.mu.Unlock()
}

// Socket is the local API socket, or "" while there is none.
func (i *Instance) Socket() string {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.socket
}

// GuideURL is the address of the index on the local listener.
func (i *Instance) GuideURL() string {
	return strings.TrimSuffix(i.LocalURL, "/") + "/llms.txt"
}

// Page returns the Markdown of topic name.
func Page(name string) (string, bool) {
	for _, t := range Topics {
		if t.Name == name {
			data, err := pages.ReadFile("llms/" + name + ".md")
			if err != nil {
				return "", false
			}
			return string(data), true
		}
	}
	return "", false
}

const summary = "WebMux 是单用户的 Web 终端：会话是服务端持有的 PTY 与进程，浏览器断开不会结束会话。" +
	"`webmux` 命令经本机套接字控制它，不需要 Token；命令以服务用户的身份运行，不是沙箱。"

const prefix = "下文的 `webmux` 指 WebMux 可执行文件：优先使用 `\"$WEBMUX_BIN\"`（服务端正在运行的那个文件），" +
	"其次是 `{{webmux}}` 简述或本实例信息中的绝对路径，最后才是 `PATH` 中的 `webmux`。" +
	"环境变量丢失时加 `--socket <套接字>`，并用会话 ID 代替 `self`。详见 [连接与身份](/llms/connect.md)。"

const when = "需要查看或控制 WebMux 会话、启动命令或 agent、了解当前会话所在的仓库与分支、给会话打标签、" +
	"或把文件交给用户时阅读本手册。先读本索引，再只读需要的主题；任务与 WebMux 无关时不必读取。"

// Index renders /llms.txt. A nil instance leaves out everything specific
// to the running server, for the unauthenticated TCP listener.
func Index(instance *Instance) string {
	var b strings.Builder
	b.WriteString("# WebMux\n\n> " + summary + "\n\n")
	if instance != nil {
		b.WriteString("本实例：\n\n")
		fmt.Fprintf(&b, "- 版本：%s\n", instance.Version)
		if instance.Bin != "" {
			fmt.Fprintf(&b, "- 可执行文件：`%s`\n", instance.Bin)
		}
		if socket := instance.Socket(); socket != "" {
			fmt.Fprintf(&b, "- 套接字：`%s`\n", socket)
		}
		b.WriteString("\n")
	}
	b.WriteString(prefix + "\n\n" + when + "\n")
	var section string
	for _, t := range Topics {
		heading := t.Section
		if t.Optional {
			heading = "Optional"
		}
		if heading != section {
			section = heading
			b.WriteString("\n## " + heading + "\n\n")
		}
		fmt.Fprintf(&b, "- [%s](/llms/%s.md): %s\n", t.Title, t.Name, t.Summary)
	}
	return b.String()
}

// Full renders /llms-full.txt: the index, then every topic in order.
func Full(instance *Instance) string {
	var b strings.Builder
	b.WriteString(Index(instance))
	for _, t := range Topics {
		if page, ok := Page(t.Name); ok {
			b.WriteString("\n---\n\n" + page)
		}
	}
	return b.String()
}

// BriefInput is the session-specific part of the {{webmux}} brief.
type BriefInput struct {
	SessionID string
	// Dir is the session's artifacts folder.
	Dir string
}

// Brief is the text {{webmux}} expands to: who the session is, where the
// executable, socket and guide are, and the one artifacts rule every task
// needs. Paths are inserted verbatim, without shell quoting.
func (i *Instance) Brief(in BriefInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "本会话运行在 WebMux 中（会话 ID：%s）。\n", in.SessionID)
	bin, socket := i.Bin, i.Socket()
	if bin != "" {
		fmt.Fprintf(&b, "- WebMux 命令：%s", bin)
		if socket != "" {
			fmt.Fprintf(&b, "（套接字：%s）", socket)
		}
		b.WriteString("\n  `webmux` 不在 PATH 中时用这个绝对路径；环境变量 WEBMUX_* 丢失时加 `--socket <套接字>`，并用上面的会话 ID 代替 self。\n")
	}
	reader := "webmux guide"
	if bin != "" {
		reader = bin + " guide"
	}
	fmt.Fprintf(&b, "- WebMux shell 工具手册（llms.txt）：%s\n", i.GuideURL())
	fmt.Fprintf(&b, "  也可以运行 `%s`", reader)
	if socket != "" {
		fmt.Fprintf(&b, "（等价于 `curl -s --unix-socket %s http://webmux/llms.txt`）", socket)
	}
	b.WriteString("，不需要 Token。需要查看或控制会话、启动命令或 agent、了解当前仓库与分支、给会话打标签或交付文件时，先读索引再按需读主题；任务与此无关时不必读取。\n")
	fmt.Fprintf(&b, "- 本会话的临时文件夹：%s\n", in.Dir)
	b.WriteString("  交付给我的截图、报告、导出数据等文件保存到这里而不是项目仓库，规则见 /llms/artifacts.md。")
	return b.String()
}
