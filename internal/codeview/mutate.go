package codeview

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/pathutil"
	"golang.org/x/sys/unix"
)

// Bounds for operations that walk a subtree. Copies and archives also share
// the MaxDownloadBytes byte limit; deletion and usage counting only walk.
const (
	MaxTransferEntries = 10000
	maxWalkEntries     = 100000
	// MaxCopyNodes bounds how many nodes one paste copies or moves.
	MaxCopyNodes = 100
)

// Transfer modes of Copy.
const (
	ModeCopy = "copy"
	ModeMove = "move"
)

// errSkip, returned by a walk visitor for a directory, skips its contents.
var errSkip = errors.New("skip directory")

// writable reports whether the service user may create entries in dir.
func writable(dir string) bool { return unix.Access(dir, unix.W_OK) == nil }

// fsError turns a failed system call into a user-facing kind without the
// low-level detail; errors already classified pass through.
func fsError(op string, err error) error {
	var e *Error
	switch {
	case errors.As(err, &e), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, unix.ENOENT):
		return problemf(ErrNotFound, "%s失败：路径不存在", op)
	case errors.Is(err, unix.EACCES), errors.Is(err, unix.EPERM):
		return problemf(ErrForbidden, "%s失败：没有权限", op)
	case errors.Is(err, unix.EROFS):
		return problemf(ErrForbidden, "%s失败：只读文件系统", op)
	case errors.Is(err, unix.EXDEV):
		return problemf(ErrConflict, "%s失败：不能跨文件系统移动", op)
	case errors.Is(err, unix.EEXIST), errors.Is(err, unix.ENOTEMPTY):
		return problemf(ErrConflict, "%s失败：目标已存在或目录非空", op)
	case errors.Is(err, unix.ENOTDIR), errors.Is(err, unix.ELOOP):
		return problemf(ErrConflict, "%s失败：路径已改变，请刷新", op)
	case errors.Is(err, unix.ENOSPC), errors.Is(err, unix.EDQUOT):
		return problemf(ErrUnavailable, "%s失败：磁盘空间不足", op)
	case errors.Is(err, unix.ENAMETOOLONG):
		return problemf(ErrInvalid, "%s失败：名称过长", op)
	}
	return &Error{kind: ErrInternal, message: op + "失败: " + err.Error()}
}

// cleanName validates a user-supplied entry name.
func cleanName(name string) (string, error) {
	n, err := pathutil.CleanName(name)
	if err != nil {
		return "", problemf(ErrInvalid, "名称无效：不能为空、. 或 ..、.git，不能包含 / \\ 或控制字符，且不超过 255 字节")
	}
	return n, nil
}

// dir opens directory p of the root.
func (b *binding) dir(p string) (*os.File, error) {
	f, err := b.open(p)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.IsDir() {
		_ = f.Close()
		return nil, problemf(ErrInvalid, "目标不是目录")
	}
	return f, nil
}

// entry opens the parent directory of a non-root node p and classifies the
// node itself without following it. The caller closes e.parent.
func (b *binding) entry(p string) (walkEntry, error) {
	if p == "" {
		return walkEntry{}, problemf(ErrForbidden, "根目录不可修改")
	}
	full := filepath.Join(b.Root, p)
	for _, g := range b.gitDirs {
		if full == g || strings.HasPrefix(full, g+"/") {
			return walkEntry{}, problemf(ErrInvalid, "Git 管理路径不可修改")
		}
	}
	parent, err := b.dir(parentOf(p))
	if err != nil {
		return walkEntry{}, err
	}
	e := walkEntry{parent: parent, rel: p, name: path.Base(p)}
	if e.kind, e.stat, err = statAt(int(parent.Fd()), e.name); err != nil {
		_ = parent.Close()
		return walkEntry{}, fsError("读取", err)
	}
	return e, nil
}

func parentOf(p string) string {
	if dir := path.Dir(p); dir != "." {
		return dir
	}
	return ""
}

func join(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

// statAt classifies name in directory fd without following links: file,
// directory, boundary (a nested repository), symlink or special.
func statAt(fd int, name string) (string, unix.Stat_t, error) {
	var st unix.Stat_t
	if err := unix.Fstatat(fd, name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return "", st, err
	}
	switch st.Mode & unix.S_IFMT {
	case unix.S_IFREG:
		return "file", st, nil
	case unix.S_IFLNK:
		return "symlink", st, nil
	case unix.S_IFDIR:
		child, err := unix.Openat(fd, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err != nil {
			return "", st, err
		}
		defer func() { _ = unix.Close(child) }()
		if boundary(child) {
			return "boundary", st, nil
		}
		return "directory", st, nil
	}
	return "special", st, nil
}

func openDirAt(dir *os.File, name, full string) (*os.File, error) {
	fd, err := unix.Openat(int(dir.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), full), nil
}

func (b *binding) node(p, kind string) Node {
	n := Node{ID: nodeID(p), Path: displayPath(p), Name: displayPath(path.Base(p)), Kind: kind, Referenceable: utf8.ValidString(p)}
	if kind == "file" {
		n.MediaType = nodeMediaType(n.Name)
	}
	return n
}

// walkEntry is one entry below a walked directory. Kind is "git" for any
// case of a .git entry, which is never classified or opened.
type walkEntry struct {
	parent *os.File
	rel    string
	name   string
	kind   string
	stat   unix.Stat_t
}

// walk visits every entry below directory dir (root-relative path rel) depth
// first, parents before children. A directory visitor may return errSkip.
func walk(ctx context.Context, dir *os.File, rel string, visit func(walkEntry) error) error {
	for {
		list, err := dir.ReadDir(256)
		for _, de := range list {
			if e := ctx.Err(); e != nil {
				return e
			}
			e := walkEntry{parent: dir, name: de.Name(), rel: join(rel, de.Name()), kind: "git"}
			if !strings.EqualFold(e.name, ".git") {
				var se error
				e.kind, e.stat, se = statAt(int(dir.Fd()), e.name)
				if se != nil {
					return se
				}
			}
			switch ve := visit(e); {
			case errors.Is(ve, errSkip):
				continue
			case ve != nil:
				return ve
			}
			if e.kind != "directory" {
				continue
			}
			child, oe := openDirAt(dir, e.name, e.rel)
			if oe != nil {
				return oe
			}
			we := walk(ctx, child, e.rel, visit)
			_ = child.Close()
			if we != nil {
				return we
			}
		}
		if errors.Is(err, io.EOF) || (err == nil && len(list) == 0) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// usage counts node p: one entry for itself plus, for a directory, every
// entry below it, stopping after limit entries.
func (b *binding) usage(ctx context.Context, p string, limit int) (Usage, error) {
	var u Usage
	if p != "" {
		e, err := b.entry(p)
		if err != nil {
			return u, err
		}
		_ = e.parent.Close()
		u.Entries = 1
		if e.kind != "directory" {
			if e.kind == "file" {
				u.Bytes = e.stat.Size
			}
			return u, nil
		}
	}
	dir, err := b.dir(p)
	if err != nil {
		return u, err
	}
	defer func() { _ = dir.Close() }()
	stop := errors.New("usage limit")
	err = walk(ctx, dir, p, func(e walkEntry) error {
		if u.Entries >= limit {
			u.Limited = true
			return stop
		}
		u.Entries++
		if e.kind == "file" {
			u.Bytes += e.stat.Size
		}
		return nil
	})
	if errors.Is(err, stop) {
		err = nil
	}
	return u, fsErrorOrNil("统计", err)
}

func fsErrorOrNil(op string, err error) error {
	if err == nil {
		return nil
	}
	return fsError(op, err)
}

// Usage counts the entries and bytes at and below node nid, for confirming
// deletion of a directory.
func (s *Service) Usage(ctx context.Context, id, cid, nid string) (Usage, error) {
	return withRoot(ctx, s, id, cid, func(b *binding) (Usage, error) {
		p, err := nodePath(nid)
		if err != nil {
			return Usage{}, err
		}
		return b.usage(ctx, p, maxWalkEntries)
	})
}

// reserve claims the first free name derived from want in a directory:
// create makes a candidate exclusively and fails with EEXIST when taken.
func reserve(want string, create func(string) error) (string, error) {
	for n := 1; n <= pathutil.MaxNameAttempts; n++ {
		candidate := pathutil.SuffixedName(want, n)
		if len(candidate) > pathutil.MaxNameBytes {
			return "", problemf(ErrInvalid, "名称过长")
		}
		err := create(candidate)
		if err == nil {
			return candidate, nil
		}
		if !errors.Is(err, unix.EEXIST) {
			return "", err
		}
	}
	return "", problemf(ErrConflict, "同名条目过多")
}

// placeholder exclusively creates an empty file or directory to reserve name.
func placeholder(dirfd int, name string, directory bool) error {
	if directory {
		return unix.Mkdirat(dirfd, name, 0o700)
	}
	fd, err := unix.Openat(dirfd, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
	if err == nil {
		_ = unix.Close(fd)
	}
	return err
}

func removeAt(dirfd int, name string, directory bool) {
	flags := 0
	if directory {
		flags = unix.AT_REMOVEDIR
	}
	_ = unix.Unlinkat(dirfd, name, flags)
}

// moveAt renames srcfd/name into dstfd under the first free name derived
// from want. Existing entries are never replaced: a placeholder of the same
// type reserves the name, and the rename replaces only that placeholder.
func moveAt(srcfd int, name string, dstfd int, want string, directory bool) (string, error) {
	placed, err := reserve(want, func(c string) error { return placeholder(dstfd, c, directory) })
	if err != nil {
		return "", err
	}
	if err := unix.Renameat(srcfd, name, dstfd, placed); err != nil {
		removeAt(dstfd, placed, directory)
		return "", err
	}
	return placed, nil
}

// writableKind refuses nodes that file operations never change.
func writableKind(kind string) error {
	switch kind {
	case "file", "directory":
		return nil
	case "boundary":
		return problemf(ErrInvalid, "嵌套仓库边界不可修改")
	case "symlink":
		return problemf(ErrInvalid, "符号链接只能删除")
	}
	return problemf(ErrInvalid, "特殊文件不可修改")
}

// requireWrite refuses changes to a root the service user cannot write.
func (b *binding) requireWrite() error {
	if !b.Write {
		return problemf(ErrForbidden, "浏览根不可写")
	}
	return nil
}

// Rename gives node nid the new name in its own directory; a taken name
// gets the smallest free " (n)" suffix instead of replacing an entry.
func (s *Service) Rename(ctx context.Context, id, cid, nid, name string) (Mutation, error) {
	return withRoot(ctx, s, id, cid, func(b *binding) (Mutation, error) {
		if err := b.requireWrite(); err != nil {
			return Mutation{}, err
		}
		p, err := nodePath(nid)
		if err != nil {
			return Mutation{}, err
		}
		want, err := cleanName(name)
		if err != nil {
			return Mutation{}, err
		}
		e, err := b.entry(p)
		if err != nil {
			return Mutation{}, err
		}
		defer func() { _ = e.parent.Close() }()
		if err := writableKind(e.kind); err != nil {
			return Mutation{}, err
		}
		if want == e.name {
			return Mutation{Nodes: []Node{b.node(p, e.kind)}}, nil
		}
		pfd := int(e.parent.Fd())
		placed, err := moveAt(pfd, e.name, pfd, want, e.kind == "directory")
		if err != nil {
			return Mutation{}, fsError("重命名", err)
		}
		return Mutation{Nodes: []Node{b.node(join(parentOf(p), placed), e.kind)}}, nil
	})
}

// Mkdir creates a directory named name in directory nid.
func (s *Service) Mkdir(ctx context.Context, id, cid, nid, name string) (Mutation, error) {
	return withRoot(ctx, s, id, cid, func(b *binding) (Mutation, error) {
		if err := b.requireWrite(); err != nil {
			return Mutation{}, err
		}
		p, err := nodePath(nid)
		if err != nil {
			return Mutation{}, err
		}
		want, err := cleanName(name)
		if err != nil {
			return Mutation{}, err
		}
		dir, err := b.dir(p)
		if err != nil {
			return Mutation{}, err
		}
		defer func() { _ = dir.Close() }()
		placed, err := reserve(want, func(c string) error { return unix.Mkdirat(int(dir.Fd()), c, 0o777) })
		if err != nil {
			return Mutation{}, fsError("新建文件夹", err)
		}
		return Mutation{Nodes: []Node{b.node(join(p, placed), "directory")}}, nil
	})
}

// Delete removes node nid. A non-empty directory needs recursive, and is
// refused when any entry below it is a .git entry or a nested repository.
// A symlink is removed itself, never its target.
func (s *Service) Delete(ctx context.Context, id, cid, nid string, recursive bool) error {
	_, err := withRoot(ctx, s, id, cid, func(b *binding) (struct{}, error) {
		if err := b.requireWrite(); err != nil {
			return struct{}{}, err
		}
		p, err := nodePath(nid)
		if err != nil {
			return struct{}{}, err
		}
		e, err := b.entry(p)
		if err != nil {
			return struct{}{}, err
		}
		defer func() { _ = e.parent.Close() }()
		pfd, name := int(e.parent.Fd()), e.name
		switch e.kind {
		case "file", "symlink":
			return struct{}{}, fsErrorOrNil("删除", unix.Unlinkat(pfd, name, 0))
		case "directory":
		default:
			return struct{}{}, writableKind(e.kind)
		}
		err = unix.Unlinkat(pfd, name, unix.AT_REMOVEDIR)
		if !errors.Is(err, unix.ENOTEMPTY) && !errors.Is(err, unix.EEXIST) {
			return struct{}{}, fsErrorOrNil("删除", err)
		}
		if !recursive {
			return struct{}{}, problemf(ErrConflict, "目录非空，需要确认递归删除")
		}
		dir, err := openDirAt(e.parent, name, p)
		if err != nil {
			return struct{}{}, fsError("删除", err)
		}
		entries := 0
		err = walk(ctx, dir, p, func(e walkEntry) error {
			entries++
			switch {
			case entries > maxWalkEntries:
				return problemf(ErrTooLarge, "条目超过 %d，不能一次删除", maxWalkEntries)
			case e.kind == "git" || e.kind == "boundary":
				return problemf(ErrConflict, "目录包含 .git 或嵌套仓库 %s，不能删除", displayPath(e.rel))
			}
			return nil
		})
		if err == nil {
			// The scan consumed dir's read position; remove through a fresh one.
			var fresh *os.File
			if fresh, err = openDirAt(dir, ".", p); err == nil {
				err = removeContents(ctx, fresh)
				_ = fresh.Close()
			}
		}
		_ = dir.Close()
		if err == nil {
			err = unix.Unlinkat(pfd, name, unix.AT_REMOVEDIR)
		}
		return struct{}{}, fsErrorOrNil("删除", err)
	})
	return err
}

// removeContents empties directory dir. Names are read before anything is
// removed so deletion cannot disturb the directory stream; callers bound the
// size by a prior scan. A .git entry or a nested repository appearing after
// that scan stops the removal.
func removeContents(ctx context.Context, dir *os.File) error {
	list, err := dir.ReadDir(-1)
	if err != nil {
		return err
	}
	fd := int(dir.Fd())
	for _, de := range list {
		if e := ctx.Err(); e != nil {
			return e
		}
		name := de.Name()
		if strings.EqualFold(name, ".git") {
			return problemf(ErrConflict, "删除期间出现 .git，已停止")
		}
		kind, _, err := statAt(fd, name)
		switch {
		case err != nil:
			return err
		case kind == "boundary":
			return problemf(ErrConflict, "删除期间出现嵌套仓库，已停止")
		case kind != "directory":
			if err := unix.Unlinkat(fd, name, 0); err != nil {
				return err
			}
			continue
		}
		child, err := openDirAt(dir, name, name)
		if err != nil {
			return err
		}
		err = removeContents(ctx, child)
		_ = child.Close()
		if err != nil {
			return err
		}
		if err := unix.Unlinkat(fd, name, unix.AT_REMOVEDIR); err != nil {
			return err
		}
	}
	return nil
}

// Copy copies (ModeCopy) or moves (ModeMove) nodes nids into directory
// target of the same context. Every node keeps its name, or gets the
// smallest free " (n)" suffix; nothing is replaced. A copy of a directory
// recreates its files, directories and symlinks, skipping .git entries,
// nested repositories and special files, within MaxDownloadBytes and
// MaxTransferEntries in total. A move is a rename, so it fails on another
// filesystem; moving a node into its own directory leaves it in place.
func (s *Service) Copy(ctx context.Context, id, cid string, nids []string, target, mode string) (Mutation, error) {
	return withRoot(ctx, s, id, cid, func(b *binding) (Mutation, error) {
		if err := b.requireWrite(); err != nil {
			return Mutation{}, err
		}
		if mode != ModeCopy && mode != ModeMove {
			return Mutation{}, problemf(ErrInvalid, "请选择 copy/move")
		}
		if len(nids) == 0 || len(nids) > MaxCopyNodes {
			return Mutation{}, problemf(ErrInvalid, "一次可粘贴 1–%d 项", MaxCopyNodes)
		}
		dst, err := nodePath(target)
		if err != nil {
			return Mutation{}, err
		}
		paths := make([]string, len(nids))
		for i, nid := range nids {
			if paths[i], err = nodePath(nid); err != nil {
				return Mutation{}, err
			}
			if paths[i] == "" {
				return Mutation{}, problemf(ErrForbidden, "根目录不可复制或移动")
			}
			if dst == paths[i] || strings.HasPrefix(dst, paths[i]+"/") {
				return Mutation{}, problemf(ErrInvalid, "不能粘贴到自身或其子目录")
			}
		}
		dir, err := b.dir(dst)
		if err != nil {
			return Mutation{}, err
		}
		defer func() { _ = dir.Close() }()
		if mode == ModeCopy {
			if err := b.copyBudget(ctx, paths); err != nil {
				return Mutation{}, err
			}
		}
		op := map[string]string{ModeCopy: "复制", ModeMove: "移动"}[mode]
		var out Mutation
		budget := int64(MaxDownloadBytes)
		for _, p := range paths {
			e, err := b.entry(p)
			if err != nil {
				return out, err
			}
			if err := writableKind(e.kind); err != nil {
				_ = e.parent.Close()
				return out, err
			}
			var placed string
			switch {
			case mode == ModeMove && parentOf(p) == dst:
				placed = e.name
			case mode == ModeMove:
				placed, err = moveAt(int(e.parent.Fd()), e.name, int(dir.Fd()), e.name, e.kind == "directory")
			default:
				placed, err = copyAt(ctx, e.parent, e.name, e.kind, dir, &budget)
			}
			_ = e.parent.Close()
			if err != nil {
				return out, fsError(op, err)
			}
			out.Nodes = append(out.Nodes, b.node(join(dst, placed), e.kind))
		}
		return out, nil
	})
}

// copyBudget refuses copies above the byte or entry bound before any write.
func (b *binding) copyBudget(ctx context.Context, paths []string) error {
	var total Usage
	for _, p := range paths {
		u, err := b.usage(ctx, p, MaxTransferEntries+1)
		if err != nil {
			return err
		}
		total.Entries += u.Entries
		total.Bytes += u.Bytes
		if total.Entries > MaxTransferEntries {
			return problemf(ErrTooLarge, "超过 %d 个条目的复制上限", MaxTransferEntries)
		}
		if total.Bytes > MaxDownloadBytes {
			return problemf(ErrTooLarge, "超过 100 MiB 复制上限")
		}
	}
	return nil
}

// copyAt copies src/name (a file or directory) into dst under a free name.
// A failed copy removes what it created.
func copyAt(ctx context.Context, src *os.File, name, kind string, dst *os.File, budget *int64) (string, error) {
	if kind == "file" {
		var out *os.File
		placed, err := reserve(name, func(c string) error {
			fd, err := unix.Openat(int(dst.Fd()), c, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
			if err == nil {
				out = os.NewFile(uintptr(fd), c)
			}
			return err
		})
		if err != nil {
			return "", err
		}
		if err := copyFile(ctx, src, name, out, budget); err != nil {
			removeAt(int(dst.Fd()), placed, false)
			return "", err
		}
		return placed, nil
	}
	placed, err := reserve(name, func(c string) error { return unix.Mkdirat(int(dst.Fd()), c, 0o700) })
	if err != nil {
		return "", err
	}
	if err := copyDir(ctx, src, name, dst, placed, budget); err != nil {
		if out, oe := openDirAt(dst, placed, placed); oe == nil {
			_ = removeContents(context.WithoutCancel(ctx), out)
			_ = out.Close()
		}
		removeAt(int(dst.Fd()), placed, true)
		return "", err
	}
	return placed, nil
}

// copyFile copies the regular file src/name into out, closes out and gives
// it the source's permission bits.
func copyFile(ctx context.Context, src *os.File, name string, out *os.File, budget *int64) error {
	defer func() { _ = out.Close() }()
	fd, err := unix.Openat(int(src.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	in := os.NewFile(uintptr(fd), name)
	defer func() { _ = in.Close() }()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return problemf(ErrConflict, "复制期间文件已替换")
	}
	n, err := io.Copy(out, contextReader{ctx, io.LimitReader(in, *budget+1)})
	if err != nil {
		return err
	}
	if *budget -= n; *budget < 0 {
		return problemf(ErrTooLarge, "超过 100 MiB 复制上限")
	}
	if err := out.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	return out.Close()
}

// copyDir fills the new directory dst/placed with the contents of src/name.
func copyDir(ctx context.Context, src *os.File, name string, dst *os.File, placed string, budget *int64) error {
	from, err := openDirAt(src, name, name)
	if err != nil {
		return err
	}
	defer func() { _ = from.Close() }()
	info, err := from.Stat()
	if err != nil {
		return err
	}
	to, err := openDirAt(dst, placed, placed)
	if err != nil {
		return err
	}
	defer func() { _ = to.Close() }()
	entries := 0
	for {
		list, err := from.ReadDir(256)
		for _, de := range list {
			if e := ctx.Err(); e != nil {
				return e
			}
			child := de.Name()
			if strings.EqualFold(child, ".git") {
				continue
			}
			kind, _, se := statAt(int(from.Fd()), child)
			if se != nil {
				return se
			}
			if entries++; entries > MaxTransferEntries {
				return problemf(ErrTooLarge, "超过 %d 个条目的复制上限", MaxTransferEntries)
			}
			switch kind {
			case "file":
				out, oe := unix.Openat(int(to.Fd()), child, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
				if oe != nil {
					return oe
				}
				if ce := copyFile(ctx, from, child, os.NewFile(uintptr(out), child), budget); ce != nil {
					return ce
				}
			case "directory":
				if me := unix.Mkdirat(int(to.Fd()), child, 0o700); me != nil {
					return me
				}
				if ce := copyDir(ctx, from, child, to, child, budget); ce != nil {
					return ce
				}
			case "symlink":
				if le := copyLink(int(from.Fd()), child, int(to.Fd())); le != nil {
					return le
				}
			}
		}
		if errors.Is(err, io.EOF) || (err == nil && len(list) == 0) {
			break
		}
		if err != nil {
			return err
		}
	}
	return to.Chmod(info.Mode().Perm())
}

// copyLink recreates a symlink with the same target; the target is never read.
func copyLink(from int, name string, to int) error {
	buf := make([]byte, unix.PathMax)
	n, err := unix.Readlinkat(from, name, buf)
	if err != nil {
		return err
	}
	if n >= len(buf) {
		return problemf(ErrInvalid, "链接目标过长")
	}
	return unix.Symlinkat(string(buf[:n]), to, name)
}
