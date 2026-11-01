package codeview

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/rainchan/webmux/internal/workspace"
)

// ErrChanged reports that a source changed after streaming started; callers
// must abort the response rather than finish it.
var ErrChanged = errors.New("内容已变化，请刷新")

// Transfer streams the exact source bytes of one snapshot side. It holds a
// transfer slot (not a service slot) until Close.
type Transfer struct {
	Name      string
	MediaType string
	Size      int64
	binding   *binding
	path      string
	file      *os.File
	identity  *identity
	oid       string
	release   func()
}

// Raw resolves a snapshot side into an opened, verified Transfer. The service
// slot is released before returning so slow clients only occupy a transfer slot.
func (s *Service) Raw(ctx context.Context, id, sid, side string) (*Transfer, error) {
	return holdTransfer(s, func() (*Transfer, error) { return s.openRaw(ctx, id, sid, side) }, func(t *Transfer, done func()) { t.release = done })
}

// NodeRaw opens the regular file nodeId of context cid for download, like
// Raw but without a snapshot: the transfer pins the file opened now.
func (s *Service) NodeRaw(ctx context.Context, id, cid, nid string) (*Transfer, error) {
	return holdTransfer(s, func() (*Transfer, error) {
		return withRoot(ctx, s, id, cid, func(b *binding) (*Transfer, error) {
			p, e := nodePath(nid)
			if e != nil {
				return nil, e
			}
			f, e := b.open(p)
			if e != nil {
				return nil, e
			}
			info, e := f.Stat()
			switch {
			case e != nil:
				_ = f.Close()
				return nil, e
			case !info.Mode().IsRegular():
				_ = f.Close()
				return nil, problemf(ErrInvalid, "目录请下载为 ZIP")
			case info.Size() > MaxDownloadBytes:
				_ = f.Close()
				return nil, problemf(ErrTooLarge, "超过 100 MiB 下载上限")
			}
			return &Transfer{Name: path.Base(p), Size: info.Size(), binding: b, path: p, file: f, identity: fileIdentity(info)}, nil
		})
	}, func(t *Transfer, done func()) { t.release = done })
}

// holdTransfer runs open while holding one of the transfer slots and hands
// the slot to the opened value through keep; a failed open frees it.
func holdTransfer[T any](s *Service, open func() (T, error), keep func(T, func())) (T, error) {
	select {
	case s.transfers <- struct{}{}:
	default:
		var zero T
		return zero, problemf(ErrBusy, "传输并发已满，请稍后重试")
	}
	done := func() { <-s.transfers }
	v, e := open()
	if e != nil {
		done()
		return v, e
	}
	keep(v, done)
	return v, nil
}
func (s *Service) openRaw(ctx context.Context, id, sid, side string) (*Transfer, error) {
	return withBinding(ctx, s, id, s.snapshotRoot(id, sid), func(b *binding) (*Transfer, error) {
		v, e := s.get(id, sid)
		if e != nil {
			return nil, e
		}
		var d Document
		if v.file != nil {
			d = v.file.Document
		} else if side == "old" {
			d = v.diff.Old
		} else if side == "new" {
			d = v.diff.New
		} else {
			return nil, problemf(ErrInvalid, "请选择 old/new 来源")
		}
		if d.Source == "empty" {
			return nil, problemf(ErrNotFound, "此侧内容不存在")
		}
		if d.Size > MaxDownloadBytes {
			return nil, problemf(ErrTooLarge, "超过 100 MiB 下载上限")
		}
		if !d.Downloadable {
			return nil, problemf(ErrNotFound, "此来源不是可下载的常规文件")
		}
		t := &Transfer{Name: path.Base(d.rawPath), MediaType: d.MediaType, Size: d.Size, binding: b, path: d.rawPath}
		if d.Source == "worktree" {
			if d.identity == nil {
				return nil, problemf(ErrConflict, "内容已变化，请刷新")
			}
			f, e := b.open(d.rawPath)
			if e != nil {
				if errors.Is(e, os.ErrNotExist) {
					return nil, problemf(ErrConflict, "内容已变化，请刷新")
				}
				return nil, e
			}
			info, e := f.Stat()
			if e != nil || !info.Mode().IsRegular() || *fileIdentity(info) != *d.identity {
				_ = f.Close()
				return nil, problemf(ErrConflict, "内容已变化，请刷新")
			}
			t.file, t.identity = f, d.identity
			return t, nil
		}
		if d.Source != "index" && d.Source != "head" {
			return nil, problemf(ErrInvalid, "无效文件来源")
		}
		raw, e := git(ctx, b.Root, "cat-file", "-s", d.OID)
		if e != nil {
			return nil, fmt.Errorf("读取对象大小: %w", e)
		}
		size, e := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
		if e != nil || size != d.Size {
			return nil, problemf(ErrConflict, "内容已变化，请刷新")
		}
		t.oid = d.OID
		return t, nil
	})
}

// Close releases the opened file and the transfer slot; safe to call twice.
func (t *Transfer) Close() {
	if t.file != nil {
		_ = t.file.Close()
		t.file = nil
	}
	if t.release != nil {
		t.release()
		t.release = nil
	}
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (c contextReader) Read(p []byte) (int, error) {
	if e := c.ctx.Err(); e != nil {
		return 0, e
	}
	return c.r.Read(p)
}

// WriteTo copies exactly Size bytes to w. A non-nil error after any byte was
// written means the response is incomplete and must be aborted; ErrChanged
// marks a source that no longer matches the snapshot.
func (t *Transfer) WriteTo(ctx context.Context, w io.Writer) error {
	if t.file == nil {
		out := &countingWriter{w: w}
		if e := workspace.StreamGit(ctx, t.binding.Root, out, "cat-file", "blob", t.oid); e != nil {
			return e
		}
		if out.n != t.Size {
			return ErrChanged
		}
		return nil
	}
	n, e := io.CopyN(w, contextReader{ctx, t.file}, t.Size)
	if e != nil && !errors.Is(e, io.EOF) {
		return e
	}
	if e != nil || n != t.Size {
		return ErrChanged
	}
	info, e := t.file.Stat()
	if e != nil || *fileIdentity(info) != *t.identity {
		return ErrChanged
	}
	current, e := t.binding.open(t.path)
	if e != nil {
		return ErrChanged
	}
	// Read-only descriptor cleanup cannot affect the captured contents.
	defer func() { _ = current.Close() }()
	now, e := current.Stat()
	if e != nil || !os.SameFile(info, now) || *fileIdentity(now) != *t.identity {
		return ErrChanged
	}
	return nil
}
