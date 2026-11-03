package docguard

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"unicode"
)

// Navigation rules: every document a reader may need is reachable by following
// relative links from an entry point, every relative link resolves, and the
// README stays a small landing page instead of a manual.
const (
	maxReadmeLines  = 170
	maxReadmeBytes  = 14 << 10
	minReadmeShots  = 4
	maxReadmeImages = 20
)

// entryPoints are where readers start: humans on the README, agents on AGENTS.md.
var entryPoints = []string{"README.md", "AGENTS.md"}

// reachRoots hold the Markdown that must be reachable; root-level *.md files
// are added separately.
var reachRoots = []string{"docs", "baseline", "openspec/specs"}

// readmeLinks are the destinations the README must offer: the project
// introduction, the documentation hub, help and the contributor/security
// policies.
var readmeLinks = []struct{ target, purpose string }{
	{"docs/overview.md", "项目介绍"},
	{"docs/README.md", "文档中心"},
	{"docs/faq.md", "常见问题与获取帮助"},
	{"SECURITY.md", "安全策略"},
	{"CONTRIBUTING.md", "参与贡献"},
}

// readmeSections are the level-2 headings a visitor looks for first.
var readmeSections = []string{"快速开始", "文档", "获取帮助"}

// TestDocsNavigation is the failing gate for links, reachability and the
// README budget. Like TestDocsGovernance, its output is addressed to the next
// agent.
func TestDocsNavigation(t *testing.T) {
	fsys := os.DirFS(filepath.Join("..", ".."))
	violations, err := checkNavigation(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) == 0 {
		return
	}
	t.Error(navigationReport(violations))
}

func checkNavigation(fsys fs.FS) ([]violation, error) {
	files, err := navigationFiles(fsys)
	if err != nil {
		return nil, err
	}
	docs := newDocSet(fsys)
	var out []violation
	for _, name := range files {
		if !mustReach(name) && !strings.HasPrefix(name, "openspec/changes/") {
			continue
		}
		v, err := checkLinks(docs, name)
		if err != nil {
			return nil, err
		}
		out = append(out, v...)
	}

	reached, err := reachable(docs)
	if err != nil {
		return nil, err
	}
	for _, name := range files {
		if mustReach(name) && !reached[name] {
			out = append(out, violation{name, "unreachable", unreachableMessage(name)})
		}
	}

	v, err := checkReadme(docs)
	if err != nil {
		return nil, err
	}
	out = append(out, v...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].path != out[j].path {
			return out[i].path < out[j].path
		}
		return out[i].rule < out[j].rule
	})
	return out, nil
}

// navigationFiles lists the Markdown files whose links are checked: root-level
// files, the reach roots and in-flight OpenSpec changes. Archived changes are
// immutable history and keep the links they were written with.
func navigationFiles(fsys fs.FS) ([]string, error) {
	files, err := fs.Glob(fsys, "*.md")
	if err != nil {
		return nil, err
	}
	for _, root := range append(append([]string{}, reachRoots...), "openspec/changes") {
		err := fs.WalkDir(fsys, root, func(name string, d fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return fs.SkipDir
				}
				return err
			}
			if d.IsDir() && name == "openspec/changes/archive" {
				return fs.SkipDir
			}
			if !d.IsDir() && isMarkdown(name) {
				files = append(files, name)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}

func isMarkdown(name string) bool {
	return strings.EqualFold(path.Ext(name), ".md")
}

// mustReach reports whether readers must be able to find name by following
// links. In-flight OpenSpec changes are excluded: the OpenSpec CLI lists them
// and archiving moves them, so a hand-kept index would always be stale.
func mustReach(name string) bool {
	switch {
	case !isMarkdown(name):
		return false
	case !strings.Contains(name, "/"):
		return true
	case strings.HasPrefix(name, "docs/"), strings.HasPrefix(name, "baseline/"):
		return true
	case strings.HasPrefix(name, "openspec/specs/"):
		return path.Base(name) == "spec.md"
	}
	return false
}

// docSet caches parsed Markdown files.
type docSet struct {
	fsys    fs.FS
	parsed  map[string]*mdDoc
	missing map[string]bool
}

type mdDoc struct {
	links   []mdLink
	anchors map[string]bool
	lines   int
	bytes   int
	data    []byte
}

type mdLink struct {
	target string
	line   int
	image  bool
	// source marks an HTML media reference other than <img>, such as a
	// <source srcset> candidate: it carries no alt text, the sibling <img> does.
	source bool
	hasAlt bool
}

func newDocSet(fsys fs.FS) *docSet {
	return &docSet{fsys: fsys, parsed: map[string]*mdDoc{}, missing: map[string]bool{}}
}

func (s *docSet) get(name string) (*mdDoc, error) {
	if d, ok := s.parsed[name]; ok {
		return d, nil
	}
	data, err := fs.ReadFile(s.fsys, name)
	if err != nil {
		return nil, err
	}
	d := parseMarkdown(data)
	s.parsed[name] = d
	return d, nil
}

var (
	inlineLink   = regexp.MustCompile(`(!?)\[((?:[^\[\]]|\[[^\]]*\])*)\]\(\s*<?([^()\s<>]+)>?(?:\s+(?:"[^"]*"|'[^']*'))?\s*\)`)
	refDef       = regexp.MustCompile(`^\s{0,3}\[[^\]]+\]:\s*<?([^\s>]+)>?`)
	htmlAttr     = regexp.MustCompile(`\b(src|href|srcset)\s*=\s*"([^"]*)"`)
	htmlOpenTag  = regexp.MustCompile(`<[A-Za-z][A-Za-z0-9-]*\b[^>]*>`)
	htmlAlt      = regexp.MustCompile(`\balt\s*=\s*"([^"]*)"`)
	htmlAnchor   = regexp.MustCompile(`<a\b[^>]*\b(?:id|name)\s*=\s*"([^"]+)"`)
	atxHeading   = regexp.MustCompile(`^ {0,3}#{1,6}\s+(.*?)(?:\s+#+)?\s*$`)
	headingLink  = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	htmlTag      = regexp.MustCompile(`<[^>]+>`)
	urlScheme    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
	level2Header = regexp.MustCompile(`^## +(.+?)\s*$`)
)

// parseMarkdown extracts relative-link candidates, heading anchors and size.
// Fenced code blocks and inline code spans are ignored: examples there are not
// navigation.
func parseMarkdown(data []byte) *mdDoc {
	d := &mdDoc{anchors: map[string]bool{}, lines: lineCount(data), bytes: len(data), data: data}
	seen := map[string]int{}
	rawLines := strings.Split(string(data), "\n")
	prose := make([]string, len(rawLines))
	var fenceChar byte
	fenceLen := 0
	for i, raw := range rawLines {
		trimmed := strings.TrimSpace(raw)
		if char, n, rest, ok := fenceRun(trimmed); ok {
			switch {
			case fenceChar == 0:
				fenceChar, fenceLen = char, n
			case char == fenceChar && n >= fenceLen && strings.TrimSpace(rest) == "":
				fenceChar, fenceLen = 0, 0
			}
			continue
		}
		if fenceChar != 0 {
			continue
		}
		// Headings keep code spans: GitHub builds the anchor from their text.
		if m := atxHeading.FindStringSubmatch(raw); m != nil {
			s := slug(m[1])
			n := seen[s]
			seen[s] = n + 1
			if n > 0 {
				s = fmt.Sprintf("%s-%d", s, n)
			}
			d.anchors[s] = true
		}
		line := stripCodeSpans(raw)
		prose[i] = line
		d.links = append(d.links, inlineLinks(line, i+1)...)
		if m := refDef.FindStringSubmatch(line); m != nil {
			d.links = append(d.links, mdLink{target: m[1], line: i + 1})
		}
	}
	// HTML tags may span lines (a <picture> cell wrapped to stay narrow), so
	// they are matched on the whole prose text.
	text := strings.Join(prose, "\n")
	for _, loc := range htmlOpenTag.FindAllStringIndex(text, -1) {
		tag := text[loc[0]:loc[1]]
		line := strings.Count(text[:loc[0]], "\n") + 1
		img := strings.HasPrefix(strings.ToLower(tag), "<img")
		if m := htmlAnchor.FindStringSubmatch(tag); m != nil {
			d.anchors[m[1]] = true
		}
		alt := htmlAlt.FindStringSubmatch(tag)
		for _, m := range htmlAttr.FindAllStringSubmatch(tag, -1) {
			for _, target := range attrTargets(m[1], m[2]) {
				l := mdLink{target: target, line: line, image: m[1] != "href"}
				l.source = l.image && !img
				l.hasAlt = img && alt != nil && strings.TrimSpace(alt[1]) != ""
				d.links = append(d.links, l)
			}
		}
	}
	return d
}

// inlineLinks returns Markdown links and images on one line, including an
// image nested in a link's text such as a linked badge.
func inlineLinks(line string, n int) []mdLink {
	var out []mdLink
	for _, m := range inlineLink.FindAllStringSubmatch(line, -1) {
		out = append(out, mdLink{target: m[3], line: n, image: m[1] == "!", hasAlt: strings.TrimSpace(m[2]) != ""})
		if strings.Contains(m[2], "](") {
			out = append(out, inlineLinks(m[2], n)...)
		}
	}
	return out
}

// attrTargets splits srcset candidates ("a.png 1x, b.png 2x") into URLs.
func attrTargets(attr, value string) []string {
	if attr != "srcset" {
		return []string{value}
	}
	var out []string
	for _, part := range strings.Split(value, ",") {
		if f := strings.Fields(part); len(f) > 0 {
			out = append(out, f[0])
		}
	}
	return out
}

// stripCodeSpans removes inline code spans, matching backtick runs of equal
// length as CommonMark does.
func stripCodeSpans(line string) string {
	var b strings.Builder
	for i := 0; i < len(line); {
		if line[i] != '`' {
			b.WriteByte(line[i])
			i++
			continue
		}
		n := 0
		for i+n < len(line) && line[i+n] == '`' {
			n++
		}
		run := strings.Repeat("`", n)
		end := -1
		for j := i + n; j < len(line); {
			k := strings.Index(line[j:], run)
			if k < 0 {
				break
			}
			k += j
			m := 0
			for k+m < len(line) && line[k+m] == '`' {
				m++
			}
			if m == n {
				end = k
				break
			}
			j = k + m
		}
		if end < 0 {
			b.WriteString(line[i : i+n])
			i += n
			continue
		}
		i = end + n
	}
	return b.String()
}

// slug reproduces GitHub's heading anchors: inline markup is reduced to its
// text, the result is lower-cased, punctuation (including fullwidth
// punctuation) is dropped and spaces become hyphens. CJK characters are kept.
func slug(heading string) string {
	heading = headingLink.ReplaceAllString(heading, "$1")
	heading = htmlTag.ReplaceAllString(heading, "")
	heading = strings.ToLower(strings.TrimSpace(heading))
	var b strings.Builder
	for _, r := range heading {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || r == '_':
			b.WriteRune(r)
		case unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// resolve maps a link in from to a repository path and anchor. ok is false for
// external URLs, absolute paths and pure same-page anchors are resolved to from.
func resolve(from, target string) (file, anchor string, ok bool, err error) {
	if urlScheme.MatchString(target) || strings.HasPrefix(target, "//") || strings.HasPrefix(target, "/") {
		return "", "", false, nil
	}
	rawFile, rawAnchor, _ := strings.Cut(target, "#")
	if q := strings.IndexByte(rawFile, '?'); q >= 0 {
		rawFile = rawFile[:q]
	}
	if rawFile, err = url.PathUnescape(rawFile); err != nil {
		return "", "", true, err
	}
	if rawAnchor, err = url.PathUnescape(rawAnchor); err != nil {
		return "", "", true, err
	}
	if rawFile == "" {
		return from, rawAnchor, true, nil
	}
	file = path.Join(path.Dir(from), rawFile)
	return file, rawAnchor, true, nil
}

func checkLinks(docs *docSet, name string) ([]violation, error) {
	d, err := docs.get(name)
	if err != nil {
		return nil, err
	}
	var bad []string
	for _, l := range d.links {
		file, anchor, ok, err := resolve(name, l.target)
		if !ok {
			continue
		}
		if err != nil {
			bad = append(bad, fmt.Sprintf("第 %d 行 %s（无法解码）", l.line, l.target))
			continue
		}
		if file == ".." || strings.HasPrefix(file, "../") {
			bad = append(bad, fmt.Sprintf("第 %d 行 %s（指向仓库之外）", l.line, l.target))
			continue
		}
		info, err := fs.Stat(docs.fsys, file)
		if err != nil {
			bad = append(bad, fmt.Sprintf("第 %d 行 %s（%s 不存在）", l.line, l.target, file))
			continue
		}
		if anchor == "" || info.IsDir() || !isMarkdown(file) {
			continue
		}
		target, err := docs.get(file)
		if err != nil {
			return nil, err
		}
		if !target.anchors[anchor] {
			bad = append(bad, fmt.Sprintf("第 %d 行 %s（%s 中没有标题锚点 #%s）", l.line, l.target, file, anchor))
		}
	}
	if len(bad) == 0 {
		return nil, nil
	}
	return []violation{{name, "links", brokenLinksMessage(name, bad)}}, nil
}

// reachable walks Markdown links from the entry points. A link to a directory
// reaches its README.md, which is what GitHub renders for it.
func reachable(docs *docSet) (map[string]bool, error) {
	seen := map[string]bool{}
	queue := []string{}
	for _, e := range entryPoints {
		if _, err := fs.Stat(docs.fsys, e); err == nil {
			queue = append(queue, e)
		}
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if seen[name] {
			continue
		}
		seen[name] = true
		d, err := docs.get(name)
		if err != nil {
			return nil, err
		}
		for _, l := range d.links {
			file, _, ok, err := resolve(name, l.target)
			if !ok || err != nil || file == name {
				continue
			}
			info, err := fs.Stat(docs.fsys, file)
			if err != nil {
				continue
			}
			if info.IsDir() {
				file = path.Join(file, "README.md")
				if _, err := fs.Stat(docs.fsys, file); err != nil {
					continue
				}
			}
			if isMarkdown(file) && !seen[file] {
				queue = append(queue, file)
			}
		}
	}
	return seen, nil
}

// checkReadme keeps the README a landing page: bounded size, the sections and
// links visitors look for, and a real screenshot gallery with alt text.
func checkReadme(docs *docSet) ([]violation, error) {
	const name = "README.md"
	if _, err := fs.Stat(docs.fsys, name); err != nil {
		return []violation{{name, "readme", "README.md 不存在。仓库首页必须提供项目介绍、截图、快速开始、文档与获取帮助入口。"}}, nil
	}
	d, err := docs.get(name)
	if err != nil {
		return nil, err
	}
	var out []violation
	if d.lines > maxReadmeLines || d.bytes > maxReadmeBytes {
		out = append(out, violation{name, "readme-size", readmeSizeMessage(d.lines, d.bytes)})
	}
	if lines, longest := wideLines(d.data); len(lines) > 0 {
		out = append(out, violation{name, "readme-width", readmeWidthMessage(lines, longest)})
	}

	headings := map[string]bool{}
	for _, line := range strings.Split(string(d.data), "\n") {
		if m := level2Header.FindStringSubmatch(line); m != nil {
			headings[m[1]] = true
		}
	}
	var missing []string
	for _, want := range readmeSections {
		found := false
		for h := range headings {
			if strings.Contains(h, want) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, "“## …"+want+"…”二级标题")
		}
	}
	targets := map[string]bool{}
	shots := map[string]bool{}
	images := map[string]bool{}
	var noAlt []string
	for _, l := range d.links {
		file, _, ok, err := resolve(name, l.target)
		if !ok || err != nil {
			continue
		}
		targets[file] = true
		if !l.image {
			continue
		}
		images[file] = true
		if strings.HasPrefix(file, "docs/images/") && !strings.HasPrefix(file, "docs/images/brand/") {
			shots[file] = true
		}
		if !l.hasAlt && !l.source {
			noAlt = append(noAlt, fmt.Sprintf("第 %d 行 %s", l.line, l.target))
		}
	}
	for _, want := range readmeLinks {
		if !targets[want.target] {
			missing = append(missing, fmt.Sprintf("指向 %s（%s）的链接", want.target, want.purpose))
		}
	}
	if len(missing) > 0 {
		out = append(out, violation{name, "readme-structure", readmeStructureMessage(missing)})
	}
	if len(shots) < minReadmeShots || len(images) > maxReadmeImages {
		out = append(out, violation{name, "readme-images", readmeImagesMessage(len(shots), len(images))})
	}
	if len(noAlt) > 0 {
		out = append(out, violation{name, "readme-alt", readmeAltMessage(noAlt)})
	}
	return out, nil
}

func brokenLinksMessage(name string, bad []string) string {
	return fmt.Sprintf("%s：%d 个相对链接失效：%s。请改为指向现有文件与现有标题锚点（锚点按 GitHub 规则生成：小写、去掉标点、空格变连字符、同名标题追加 -1、-2）；文件被拆分或改名时，按 docs/doc-governance.md 更新所有引用；目标已归档的 OpenSpec 变更请指向 openspec/changes/archive/<日期>-<名称>/。不要删除链接来让校验通过，除非目标内容已确认不再存在。", name, len(bad), strings.Join(bad, "；"))
}

func unreachableMessage(name string) string {
	where := "语义最近的索引（docs/README.md 的对应分组，或同名目录的树状索引，如 docs/usage.md 之于 docs/usage/）"
	switch {
	case strings.HasPrefix(name, "openspec/specs/"):
		where = "docs/specs.md 中对应的主题分组"
	case strings.HasPrefix(name, "baseline/"):
		where = "baseline/README.md"
	case !strings.Contains(name, "/"):
		where = "README.md 或 docs/README.md"
	}
	return fmt.Sprintf("%s：无法从 README.md 或 AGENTS.md 沿相对链接到达，读者找不到它。请在%s中加入指向它的链接，并用一句话说明它的用途；不要把链接堆进根目录 README.md（它有体积预算），也不要为了通过校验删除文档。确认内容已过时或重复时，按 docs/doc-governance.md 合并到现有文档后再删除。", name, where)
}

func readmeSizeMessage(lines, bytes int) string {
	return fmt.Sprintf("README.md：%d 行、%d 字节，超过首页预算 %d 行或 %d KiB。README 是落地页，只放一句话定位、核心卖点、截图、最短的快速开始、文档与帮助入口；详细说明请原文迁移到 docs/（项目介绍放 docs/overview.md，常见问题放 docs/faq.md，安装与配置放 docs/deployment/），README 中只留一行摘要和链接。不要通过合并行、删减安全提示或去掉截图来过线。", lines, bytes, maxReadmeLines, maxReadmeBytes>>10)
}

func readmeWidthMessage(lines []int, longest int) string {
	places := make([]string, len(lines))
	for i, n := range lines {
		places[i] = fmt.Sprint(n)
	}
	return fmt.Sprintf("README.md：第 %s 行超过 %d 列（最长 %d 列，汉字按 2 列计）。首页段落应短而可扫读：把长句改写为短句或列表，把细节移到 docs/ 并链接过去；HTML 表格单元格可以拆成多行书写。", strings.Join(places, "、"), maxLineWidth, longest)
}

func readmeStructureMessage(missing []string) string {
	return fmt.Sprintf("README.md：缺少首页必备入口：%s。访客打开首页时要能立刻找到项目介绍、快速开始、全部文档和获取帮助的方式；请补上对应章节或链接，详细内容放在 docs/ 中。", strings.Join(missing, "、"))
}

func readmeImagesMessage(shots, images int) string {
	return fmt.Sprintf("README.md：引用了 %d 张 docs/images/ 下的界面截图（品牌图除外），共 %d 张图片；首页至少需要 %d 张真实界面截图、至多 %d 张图片。截图用 pnpm --dir web screenshots 生成或取自 docs/images/<主题>/，更多截图请放到 docs/overview.md。", shots, images, minReadmeShots, maxReadmeImages)
}

func readmeAltMessage(noAlt []string) string {
	return fmt.Sprintf("README.md：以下图片缺少替代文本（alt）：%s。请写出图片展示的内容，方便读屏用户与图片加载失败时阅读。", strings.Join(noAlt, "；"))
}

func navigationReport(violations []violation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "文档导航校验未通过（%d 项）。规则见 docs/doc-governance.md 的“链接与可达性”和“README 预算”：所有文档都要能从 README.md 或 AGENTS.md 沿链接找到，相对链接与锚点必须有效，README 保持为简短的落地页。", len(violations))
	for _, v := range violations {
		fmt.Fprintf(&b, "\n\n%s", v.msg)
	}
	return b.String()
}

func TestNavigationRules(t *testing.T) {
	lines := func(n int) string {
		return strings.Repeat("- 一行\n", n)
	}
	shots := ""
	for i := range minReadmeShots {
		shots += fmt.Sprintf("<img src=\"docs/images/s%d.png\" alt=\"截图 %d\">\n", i, i)
	}
	readme := "# WebMux\n\n" + shots +
		"<picture><source media=\"(prefers-color-scheme: dark)\" srcset=\"docs/images/brand/dark.svg\">\n" +
		"  <img src=\"docs/images/brand/light.svg\" alt=\"WebMux\"></picture>\n" +
		"[![Test](https://example.com/badge.svg)](https://example.com/ci)\n" +
		"## 为什么\n[介绍](docs/overview.md)\n" +
		"## 快速开始\n```sh\nsee [x](missing-in-fence.md)\n```\n" +
		"## 文档\n[文档中心](docs/README.md)、[安全](SECURITY.md)、[贡献](CONTRIBUTING.md)\n" +
		"## 获取帮助\n[常见问题](docs/faq.md#常见问题)\n"

	fsys := fstest.MapFS{}
	add := func(name, data string) {
		fsys[name] = &fstest.MapFile{Data: []byte(data)}
	}
	for i := range minReadmeShots {
		add(fmt.Sprintf("docs/images/s%d.png", i), "png")
	}
	add("docs/images/brand/dark.svg", "svg")
	add("docs/images/brand/light.svg", "svg")
	add("README.md", readme)
	add("AGENTS.md", "# Agents\n[规则](docs/agent-rules.md)\n")
	add("SECURITY.md", "# 安全\n")
	add("CONTRIBUTING.md", "# 贡献\n")
	add("docs/overview.md", "# 介绍\n")
	add("docs/faq.md", "# 帮助\n## 常见问题\n")
	add("docs/agent-rules.md", "# 规则\n[目录](rules)\n")
	add("docs/rules/README.md", "# 目录\n")
	add("docs/README.md", strings.Join([]string{
		"# 文档中心",
		"[同页](#文档中心)、[外链](https://example.com/x.md)、[绝对](/docs/x.md)",
		"[重复标题](guide.md#安装-1)、[全角标点](guide.md#配置cli)、[代码标题](guide.md#webmux-run)",
		"[HTML 锚点](guide.md#custom)",
		"[坏锚点](guide.md#不存在)、[坏文件](gone.md)、[越界](../../outside.md)",
		"`[行内代码](ignored.md)` 与 ``[双反引号 ` 内](ignored.md)``",
		"[规格](specs.md)、[基线](../baseline)",
		"[ref]: guide.md#安装",
		"[坏引用]: nowhere.md",
	}, "\n")+"\n")
	add("docs/guide.md", "# 指南\n## 安装\n## 安装\n## 配置（CLI）\n### `webmux run`\n<a id=\"custom\"></a>\n")
	add("docs/specs.md", "# 规格\n[a](../openspec/specs/a/spec.md)\n")
	add("docs/orphan.md", lines(minLines))
	add("docs/orphan-tree/note.md", "# 孤立\n")
	add("baseline/README.md", "# 基线\n[报告](REPORT.md)\n")
	add("baseline/REPORT.md", "# 报告\n")
	add("baseline/LOST.md", "# 遗失\n")
	add("NOTES.md", "# 根目录笔记\n")
	add("openspec/specs/a/spec.md", "# a\n")
	add("openspec/specs/b/spec.md", "# b\n")
	add("openspec/specs/b/notes.md", "# 非 spec 文件不要求可达\n")
	add("openspec/changes/work/proposal.md", "# 在途变更\n[坏](../../specs/missing/spec.md)\n")
	add("openspec/changes/archive/2026-01-01-old/proposal.md", "# 归档\n[坏](gone.md)\n")

	violations, err := checkNavigation(fsys)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"NOTES.md|unreachable",
		"baseline/LOST.md|unreachable",
		"docs/README.md|links",
		"docs/orphan-tree/note.md|unreachable",
		"docs/orphan.md|unreachable",
		"openspec/changes/work/proposal.md|links",
		"openspec/specs/b/spec.md|unreachable",
	}
	var got []string
	for _, v := range violations {
		got = append(got, v.path+"|"+v.rule)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("violations:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	messages := map[string]string{}
	for _, v := range violations {
		messages[v.path+"|"+v.rule] = v.msg
	}
	for key, texts := range map[string][]string{
		"docs/README.md|links": {
			"4 个相对链接失效",
			"guide.md#不存在（docs/guide.md 中没有标题锚点 #不存在）",
			"gone.md（docs/gone.md 不存在）",
			"../../outside.md（指向仓库之外）",
			"nowhere.md（docs/nowhere.md 不存在）",
			"第 5 行", "第 9 行",
		},
		"docs/orphan.md|unreachable":              {"无法从 README.md 或 AGENTS.md", "docs/README.md"},
		"docs/orphan-tree/note.md|unreachable":    {"同名目录的树状索引"},
		"openspec/specs/b/spec.md|unreachable":    {"docs/specs.md"},
		"baseline/LOST.md|unreachable":            {"baseline/README.md"},
		"NOTES.md|unreachable":                    {"README.md 或 docs/README.md"},
		"openspec/changes/work/proposal.md|links": {"openspec/specs/missing/spec.md 不存在"},
	} {
		for _, text := range texts {
			if !strings.Contains(messages[key], text) {
				t.Errorf("%s message %q missing %q", key, messages[key], text)
			}
		}
	}
	if strings.Contains(messages["docs/README.md|links"], "ignored.md") {
		t.Error("links inside inline code spans were checked")
	}
	if !strings.Contains(navigationReport(violations), "docs/doc-governance.md") {
		t.Error("report does not point at docs/doc-governance.md")
	}

	// README budget and structure, checked on variants of the valid README.
	for _, tc := range []struct {
		name, readme string
		rules        []string
		texts        []string
	}{
		{"too long", readme + lines(maxReadmeLines), []string{"readme-size"}, []string{"超过首页预算", "docs/overview.md"}},
		{"too heavy", readme + strings.Repeat(strings.Repeat("x", 100)+"\n", maxReadmeBytes/100), []string{"readme-size"}, []string{"KiB"}},
		{"wide line", readme + strings.Repeat("字", maxLineWidth/2+1) + "\n", []string{"readme-width"}, []string{"超过 300 列"}},
		{"missing help", strings.Replace(readme, "## 获取帮助\n[常见问题](docs/faq.md#常见问题)\n", "", 1), []string{"readme-structure"}, []string{"获取帮助", "docs/faq.md"}},
		{"few shots", strings.Replace(readme, shots, "", 1), []string{"readme-images"}, []string{"至少需要 4 张"}},
		{"missing alt", readme + "![](docs/images/s0.png)\n<img src=\"docs/images/s1.png\"\n  alt=\"\">\n", []string{"readme-alt"}, []string{"第 20 行 docs/images/s0.png", "第 21 行 docs/images/s1.png"}},
		{"missing image", readme + "![坏图](docs/images/gone.png)\n", []string{"links"}, []string{"docs/images/gone.png 不存在"}},
	} {
		variant := fstest.MapFS{}
		for k, v := range fsys {
			variant[k] = v
		}
		variant["README.md"] = &fstest.MapFile{Data: []byte(tc.readme)}
		got, err := checkNavigation(variant)
		if err != nil {
			t.Fatal(err)
		}
		var rules []string
		var text string
		for _, v := range got {
			if v.path == "README.md" {
				rules = append(rules, v.rule)
				text += v.msg
			}
		}
		if strings.Join(rules, ",") != strings.Join(tc.rules, ",") {
			t.Errorf("%s: README rules %v, want %v", tc.name, rules, tc.rules)
		}
		for _, want := range tc.texts {
			if !strings.Contains(text, want) {
				t.Errorf("%s: message %q missing %q", tc.name, text, want)
			}
		}
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Linux 安装包（deb / rpm）":             "linux-安装包deb--rpm",
		"9. 故障排查":                          "9-故障排查",
		"`webmux run` 与 **附着**":            "webmux-run-与-附着",
		"[链接](x.md) 标题":                    "链接-标题",
		"OpenSpec 工作流（可选）":                 "openspec-工作流可选",
		"snake_case & Kebab-Case!":         "snake_case--kebab-case",
		"<code>x</code> 混合会话（ACP 聊天 ↔ 终端）": "x-混合会话acp-聊天--终端",
	} {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}
