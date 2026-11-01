package codeview

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

// Archive streams one directory as a ZIP file. It holds the opened directory
// and a transfer slot (not a service slot) until Close.
type Archive struct {
	Name    string
	dir     *os.File
	rel     string
	prefix  string
	ignored ignoredPaths
	release func()
}

// Archive opens directory nid of context cid for a streamed ZIP download of
// its regular files and directories, skipping .git entries, nested
// repositories, symlinks, special files and, unless include, Git-ignored
// paths. The uncompressed total is checked against MaxDownloadBytes and
// MaxTransferEntries before any byte is sent.
func (s *Service) Archive(ctx context.Context, id, cid, nid string, include bool) (*Archive, error) {
	return holdTransfer(s, func() (*Archive, error) {
		return withRoot(ctx, s, id, cid, func(b *binding) (*Archive, error) {
			p, e := nodePath(nid)
			if e != nil {
				return nil, e
			}
			a := &Archive{rel: p, ignored: ignoredPaths{}}
			if b.Ignore && !include {
				ignored, limited, e := b.searchIgnored(ctx)
				if e != nil {
					return nil, e
				}
				if limited {
					return nil, problemf(ErrTooLarge, "忽略项过多，无法打包")
				}
				a.ignored = ignored
			}
			if a.dir, e = b.dir(p); e != nil {
				return nil, e
			}
			a.prefix = path.Base(p)
			if p == "" {
				a.prefix = filepath.Base(b.Root)
			}
			a.Name = a.prefix + ".zip"
			if e := a.budget(ctx); e != nil {
				_ = a.dir.Close()
				return nil, e
			}
			return a, nil
		})
	}, func(a *Archive, done func()) { a.release = done })
}

// visit walks the archived entries: directories and regular files only.
// Each visit reads a fresh descriptor of the directory, from its start.
func (a *Archive) visit(ctx context.Context, fn func(walkEntry) error) error {
	dir, err := openDirAt(a.dir, ".", a.dir.Name())
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	return walk(ctx, dir, a.rel, func(e walkEntry) error {
		switch {
		case e.kind != "file" && e.kind != "directory", a.ignored.contains(e.rel):
			return errSkip
		}
		return fn(e)
	})
}

func (a *Archive) budget(ctx context.Context) error {
	entries, bytes := 0, int64(0)
	err := a.visit(ctx, func(e walkEntry) error {
		if entries++; entries > MaxTransferEntries {
			return problemf(ErrTooLarge, "超过 %d 个条目的打包上限", MaxTransferEntries)
		}
		if e.kind == "file" {
			bytes += e.stat.Size
		}
		if bytes > MaxDownloadBytes {
			return problemf(ErrTooLarge, "超过 100 MiB 打包上限")
		}
		return nil
	})
	return fsErrorOrNil("打包", err)
}

// WriteTo writes the ZIP to w. Files are read as they are reached, so one
// that changed may differ from the pre-check; the byte limit still holds.
// Any error means the response is incomplete and must be aborted.
func (a *Archive) WriteTo(ctx context.Context, w io.Writer) error {
	zw := zip.NewWriter(w)
	budget := int64(MaxDownloadBytes)
	err := a.visit(ctx, func(e walkEntry) error {
		name := a.prefix + "/" + strings.TrimPrefix(strings.TrimPrefix(e.rel, a.rel), "/")
		h := &zip.FileHeader{Name: name, Modified: time.Unix(e.stat.Mtim.Unix()), NonUTF8: !utf8.ValidString(name)}
		h.SetMode(os.FileMode(e.stat.Mode & 0o777))
		if e.kind == "directory" {
			h.Name += "/"
			h.SetMode(os.ModeDir | os.FileMode(e.stat.Mode&0o777))
			_, err := zw.CreateHeader(h)
			return err
		}
		h.Method = zip.Deflate
		fd, err := unix.Openat(int(e.parent.Fd()), e.name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err != nil {
			return err
		}
		f := os.NewFile(uintptr(fd), e.name)
		defer func() { _ = f.Close() }()
		if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
			return ErrChanged
		}
		out, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		n, err := io.Copy(out, contextReader{ctx, io.LimitReader(f, budget+1)})
		if err != nil {
			return err
		}
		if budget -= n; budget < 0 {
			return problemf(ErrTooLarge, "超过 100 MiB 打包上限")
		}
		return nil
	})
	if err != nil {
		return err
	}
	return zw.Close()
}

// Close releases the directory and the transfer slot; safe to call twice.
func (a *Archive) Close() {
	if a.dir != nil {
		_ = a.dir.Close()
		a.dir = nil
	}
	if a.release != nil {
		a.release()
		a.release = nil
	}
}

// Upload is a file being received into a code directory under a hidden
// temporary name. Commit gives it its final name; Close discards it unless
// committed and releases the transfer slot.
type Upload struct {
	binding   *binding
	dir       *os.File
	parent    string
	name      string
	temp      string
	file      *os.File
	committed bool
	release   func()
}

// Upload starts receiving filename into directory nid of context cid.
// Only the base name of filename is kept.
func (s *Service) Upload(ctx context.Context, id, cid, nid, filename string) (*Upload, error) {
	name, err := cleanName(path.Base(strings.ReplaceAll(filename, `\`, "/")))
	if err != nil {
		return nil, err
	}
	return holdTransfer(s, func() (*Upload, error) {
		return withRoot(ctx, s, id, cid, func(b *binding) (*Upload, error) {
			if err := b.requireWrite(); err != nil {
				return nil, err
			}
			p, err := nodePath(nid)
			if err != nil {
				return nil, err
			}
			dir, err := b.dir(p)
			if err != nil {
				return nil, err
			}
			u := &Upload{binding: b, dir: dir, parent: p, name: name}
			u.temp, err = reserve(".webmux-upload-"+random(), func(c string) error {
				fd, err := unix.Openat(int(dir.Fd()), c, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o644)
				if err == nil {
					u.file = os.NewFile(uintptr(fd), c)
				}
				return err
			})
			if err != nil {
				_ = dir.Close()
				return nil, fsError("上传", err)
			}
			return u, nil
		})
	}, func(u *Upload, done func()) { u.release = done })
}

// Write copies at most limit bytes of src into the upload.
func (u *Upload) Write(src io.Reader, limit int64) error {
	n, err := io.Copy(u.file, io.LimitReader(src, limit+1))
	if err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) && pathErr.Op == "write" {
			return fsError("上传", err)
		}
		return err
	}
	if n > limit {
		return problemf(ErrTooLarge, "超过 %d MiB 上传上限", limit>>20)
	}
	return nil
}

// Commit closes the received file and renames it to its name, or the
// smallest free " (n)" variant; existing entries are never replaced.
func (u *Upload) Commit() (Mutation, error) {
	if err := u.file.Close(); err != nil {
		return Mutation{}, fsError("上传", err)
	}
	fd := int(u.dir.Fd())
	placed, err := moveAt(fd, u.temp, fd, u.name, false)
	if err != nil {
		return Mutation{}, fsError("上传", err)
	}
	u.committed = true
	return Mutation{Nodes: []Node{u.binding.node(join(u.parent, placed), "file")}}, nil
}

// Close removes an uncommitted upload and releases the transfer slot; safe
// to call twice.
func (u *Upload) Close() {
	if u.dir != nil {
		_ = u.file.Close()
		if !u.committed {
			removeAt(int(u.dir.Fd()), u.temp, false)
		}
		_ = u.dir.Close()
		u.dir = nil
	}
	if u.release != nil {
		u.release()
		u.release = nil
	}
}
