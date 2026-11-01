package api

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/rainchan/webmux/internal/codeview"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/wire"
)

// serveCode answers one code view JSON route within the 10 s bound. Each
// route returns its concrete contract type; archtest rule R8 checks T at
// every call.
func serveCode[T any](mux *http.ServeMux, pattern string, fn func(context.Context, *http.Request) (T, error)) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		value, err := fn(ctx, r)
		if err != nil {
			failure(w, codeStatus(err), err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, value)
	})
}

func (s *Server) codeRoutes(mux *http.ServeMux) {
	base := " /api/sessions/{id}/code"
	serveCode(mux, "GET"+base+"/context", func(ctx context.Context, r *http.Request) (contract.CodeContext, error) {
		v, err := s.Code.Context(ctx, r.PathValue("id"), r.URL.Query().Get("root"))
		return wire.CodeContext(v), err
	})
	serveCode(mux, "GET"+base+"/tree", func(ctx context.Context, r *http.Request) (contract.CodeListing, error) {
		q := r.URL.Query()
		v, err := s.Code.Tree(ctx, r.PathValue("id"), q.Get("contextId"), q.Get("nodeId"), q.Get("includeIgnored") == "true", q.Get("cursor"))
		return wire.CodeListing(v), err
	})
	serveCode(mux, "GET"+base+"/files", func(ctx context.Context, r *http.Request) (contract.CodeListing, error) {
		q := r.URL.Query()
		v, err := s.Code.Search(ctx, r.PathValue("id"), q.Get("contextId"), q.Get("q"), q.Get("includeIgnored") == "true")
		return wire.CodeListing(v), err
	})
	serveCode(mux, "GET"+base+"/locate", func(ctx context.Context, r *http.Request) (contract.CodeLocation, error) {
		q := r.URL.Query()
		v, err := s.Code.Locate(ctx, r.PathValue("id"), q.Get("contextId"), q.Get("path"))
		return wire.CodeLocation(v), err
	})
	serveCode(mux, "GET"+base+"/file", func(ctx context.Context, r *http.Request) (contract.CodeFile, error) {
		q := r.URL.Query()
		source := q.Get("source")
		if source == "" {
			source = "worktree"
		}
		v, err := s.Code.FileSource(ctx, r.PathValue("id"), q.Get("contextId"), q.Get("nodeId"), source)
		return wire.CodeFile(v), err
	})
	serveCode(mux, "GET"+base+"/changes", func(ctx context.Context, r *http.Request) (contract.CodeChanges, error) {
		q := r.URL.Query()
		v, err := s.Code.Changes(ctx, r.PathValue("id"), q.Get("contextId"), codeview.ChangeQuery{Cursor: q.Get("cursor"), Q: q.Get("q"), NodeID: q.Get("nodeId"), Group: q.Get("group")})
		return wire.CodeChanges(v), err
	})
	serveCode(mux, "GET"+base+"/diff", func(ctx context.Context, r *http.Request) (contract.CodeDiff, error) {
		v, err := s.Code.Diff(ctx, r.PathValue("id"), r.URL.Query().Get("contextId"), r.URL.Query().Get("changeId"))
		return wire.CodeDiff(v), err
	})
	serveCode(mux, "GET"+base+"/snapshots/{snapshotId}/file", func(ctx context.Context, r *http.Request) (contract.CodeFile, error) {
		v, err := s.Code.SnapshotFile(ctx, r.PathValue("id"), r.PathValue("snapshotId"), r.URL.Query().Get("side"))
		return wire.CodeFile(v), err
	})
	// Raw bytes stream outside the 10 s JSON wrapper; GET also serves HEAD.
	mux.HandleFunc("GET"+base+"/snapshots/{snapshotId}/raw", s.codeRaw)
	mux.HandleFunc("GET"+base+"/node", s.codeNodeRaw)
	mux.HandleFunc("GET"+base+"/archive", s.codeArchive)
	serveCode(mux, "GET"+base+"/usage", func(ctx context.Context, r *http.Request) (contract.CodeUsage, error) {
		q := r.URL.Query()
		v, err := s.Code.Usage(ctx, r.PathValue("id"), q.Get("contextId"), q.Get("nodeId"))
		return wire.CodeUsage(v), err
	})
	mux.HandleFunc("POST"+base+"/rename", func(w http.ResponseWriter, r *http.Request) {
		var req contract.CodeRenameRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), codeWriteTimeout)
		defer cancel()
		m, err := s.Code.Rename(ctx, r.PathValue("id"), req.Context, req.NodeID, req.Name)
		codeMutation(w, http.StatusOK, m, err)
	})
	mux.HandleFunc("POST"+base+"/mkdir", func(w http.ResponseWriter, r *http.Request) {
		var req contract.CodeMkdirRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), codeWriteTimeout)
		defer cancel()
		m, err := s.Code.Mkdir(ctx, r.PathValue("id"), req.Context, req.Parent, req.Name)
		codeMutation(w, http.StatusCreated, m, err)
	})
	mux.HandleFunc("POST"+base+"/copy", func(w http.ResponseWriter, r *http.Request) {
		var req contract.CodeCopyRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), codeWriteTimeout)
		defer cancel()
		m, err := s.Code.Copy(ctx, r.PathValue("id"), req.Context, req.NodeIDs, req.Target, req.Mode)
		codeMutation(w, http.StatusOK, m, err)
	})
	mux.HandleFunc("DELETE"+base+"/node", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		ctx, cancel := context.WithTimeout(r.Context(), codeWriteTimeout)
		defer cancel()
		if err := s.Code.Delete(ctx, r.PathValue("id"), q.Get("contextId"), q.Get("nodeId"), q.Get("recursive") == "1"); err != nil {
			failure(w, codeStatus(err), err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST"+base+"/upload", s.codeUpload)
	mux.HandleFunc("POST"+base+"/check", func(w http.ResponseWriter, r *http.Request) {
		var req contract.CodeCheckRequest
		if !decode(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		v, e := s.Code.Check(ctx, r.PathValue("id"), wire.CodeCheckRequest(req))
		if e != nil {
			failure(w, codeStatus(e), e)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, wire.CodeCheck(v))
	})
}

// codeWriteTimeout bounds one file operation, including a recursive copy or
// deletion; uploads and downloads are bounded per chunk instead.
const codeWriteTimeout = time.Minute

func codeMutation(w http.ResponseWriter, status int, m codeview.Mutation, err error) {
	if err != nil {
		failure(w, codeStatus(err), err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	reply(w, status, wire.CodeMutation(m))
}

// rawChunkDeadline bounds how long one streamed chunk may stall.
const rawChunkDeadline = 30 * time.Second

// rawLookupTimeout bounds resolving a raw transfer, not streaming it; a
// variable so tests can prove a slow reader outlives it without waiting 10 s.
var rawLookupTimeout = 10 * time.Second

func (s *Server) codeRaw(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), rawLookupTimeout)
	t, err := s.Code.Raw(ctx, r.PathValue("id"), r.PathValue("snapshotId"), r.URL.Query().Get("side"))
	cancel()
	sendTransfer(w, r, t, err, r.URL.Query().Get("download") == "1")
}

// codeNodeRaw downloads one regular file of the tree as an attachment.
func (s *Server) codeNodeRaw(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ctx, cancel := context.WithTimeout(r.Context(), rawLookupTimeout)
	t, err := s.Code.NodeRaw(ctx, r.PathValue("id"), q.Get("contextId"), q.Get("nodeId"))
	cancel()
	sendTransfer(w, r, t, err, true)
}

func sendTransfer(w http.ResponseWriter, r *http.Request, t *codeview.Transfer, err error, download bool) {
	if err != nil {
		failure(w, codeStatus(err), err)
		return
	}
	defer t.Close()
	h := w.Header()
	contentType, disposition := "application/octet-stream", "attachment"
	if t.MediaType != "" {
		contentType = t.MediaType
		if !download {
			disposition = "inline"
		}
	}
	h.Set("Content-Type", contentType)
	h.Set("Content-Length", strconv.FormatInt(t.Size, 10))
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Disposition", contentDisposition(disposition, t.Name))
	if t.MediaType == "image/svg+xml" || t.MediaType == "application/pdf" {
		h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	}
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	out := &deadlineWriter{w: w, rc: http.NewResponseController(w)}
	if err := t.WriteTo(r.Context(), out); err != nil {
		// Headers promised Content-Length; abort so clients see a failure.
		panic(http.ErrAbortHandler)
	}
}

// codeArchive streams a directory as a ZIP built while it is sent, so the
// response has no Content-Length; a failure after the headers aborts it.
func (s *Server) codeArchive(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ctx, cancel := context.WithTimeout(r.Context(), rawLookupTimeout)
	a, err := s.Code.Archive(ctx, r.PathValue("id"), q.Get("contextId"), q.Get("nodeId"), q.Get("includeIgnored") == "true")
	cancel()
	if err != nil {
		failure(w, codeStatus(err), err)
		return
	}
	defer a.Close()
	h := w.Header()
	h.Set("Content-Type", "application/zip")
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Disposition", contentDisposition("attachment", a.Name))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	if err := a.WriteTo(r.Context(), &deadlineWriter{w: w, rc: http.NewResponseController(w)}); err != nil {
		panic(http.ErrAbortHandler)
	}
}

// codeUpload stores one multipart file in a tree directory. The file is
// written under a temporary name and renamed only after the whole request
// was read, so a failed upload leaves nothing behind.
func (s *Server) codeUpload(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var upload *codeview.Upload
	defer func() {
		if upload != nil {
			upload.Close()
		}
	}()
	received := receiveUpload(w, r, func(filename string, src io.Reader) bool {
		ctx, cancel := context.WithTimeout(r.Context(), rawLookupTimeout)
		u, err := s.Code.Upload(ctx, r.PathValue("id"), q.Get("contextId"), q.Get("nodeId"), filename)
		cancel()
		if err != nil {
			failure(w, codeStatus(err), err)
			return false
		}
		upload = u
		if err := u.Write(src, maxUploadBytes); err != nil {
			var known *codeview.Error
			if errors.As(err, &known) {
				failure(w, codeStatus(err), err)
			} else {
				uploadReadFailure(w, "read uploaded file", err)
			}
			return false
		}
		return true
	})
	if !received {
		return
	}
	m, err := upload.Commit()
	codeMutation(w, http.StatusCreated, m, err)
}

// contentDisposition encodes the source basename (RFC 2231 for non-ASCII) and
// falls back to "download" for control characters or unencodable names.
func contentDisposition(disposition, name string) string {
	safe := utf8.ValidString(name) && name != ""
	for _, c := range name {
		if c < 0x20 || c == 0x7f {
			safe = false
		}
	}
	if safe {
		if v := mime.FormatMediaType(disposition, map[string]string{"filename": name}); v != "" {
			return v
		}
	}
	return mime.FormatMediaType(disposition, map[string]string{"filename": "download"})
}

// deadlineWriter extends the connection write deadline before every chunk.
type deadlineWriter struct {
	w  http.ResponseWriter
	rc *http.ResponseController
}

func (d *deadlineWriter) Write(p []byte) (int, error) {
	if err := d.rc.SetWriteDeadline(time.Now().Add(rawChunkDeadline)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return 0, err
	}
	return d.w.Write(p)
}
