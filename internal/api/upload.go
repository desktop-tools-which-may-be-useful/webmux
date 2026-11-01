package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/files"
	"github.com/rainchan/webmux/internal/session"
)

const (
	maxUploadBytes     int64 = 100 << 20
	maxUploadBodyBytes       = maxUploadBytes + 1<<20
	uploadReadTimeout        = 5 * time.Minute
)

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	item := s.find(w, r)
	if item == nil {
		return
	}
	if item.Info().Status == session.StatusExited {
		failure(w, http.StatusConflict, errors.New("cannot upload to an exited session"))
		return
	}
	var upload *files.Upload
	received := receiveUpload(w, r, func(filename string, src io.Reader) bool {
		dir, err := s.Artifacts.EnsureUploads(item.Info().ID)
		if err != nil {
			artifactsFailure(w, err)
			return false
		}
		upload, err = s.Files.StoreUpload(dir, filename, src, maxUploadBytes)
		if err != nil {
			filesFailure(w, err)
			return false
		}
		return true
	})
	if received && item.Info().Status == session.StatusExited {
		failure(w, http.StatusConflict, errors.New("session exited during upload"))
		received = false
	}
	if !received {
		if upload != nil {
			upload.Discard()
		}
		return
	}
	reply(w, http.StatusCreated, contract.UploadResult{Path: upload.Path})
}

// receiveUpload reads a multipart body of exactly one part named "file",
// at most maxUploadBytes, and hands its filename and bytes to store. store
// answers its own failures; receiveUpload answers request failures. It
// reports whether store succeeded and the whole body was read, so the
// caller still owns the response and any cleanup of what store kept.
func receiveUpload(w http.ResponseWriter, r *http.Request, store func(filename string, src io.Reader) bool) bool {
	controller := http.NewResponseController(w)
	if err := controller.SetReadDeadline(time.Now().Add(uploadReadTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		failure(w, http.StatusInternalServerError, fmt.Errorf("set upload read deadline: %w", err))
		return false
	}
	// Interrupt a blocked network read on cancellation, and restore the deadline
	// before returning so this route's longer deadline cannot affect other routes.
	cancelled := make(chan struct{})
	stop := context.AfterFunc(r.Context(), func() {
		_ = controller.SetReadDeadline(time.Now())
		close(cancelled)
	})
	defer func() {
		if !stop() {
			<-cancelled
		}
		_ = controller.SetReadDeadline(time.Time{})
	}()

	if r.ContentLength > maxUploadBodyBytes {
		failure(w, http.StatusRequestEntityTooLarge, errors.New("upload request exceeds 101 MiB"))
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBodyBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		failure(w, http.StatusBadRequest, fmt.Errorf("read upload multipart body: %w", err))
		return false
	}
	part, err := reader.NextRawPart()
	if err != nil {
		uploadReadFailure(w, "expected exactly one file part named file", err)
		return false
	}
	disposition, params, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	if err != nil || disposition != "form-data" || params["name"] != "file" || params["filename"] == "" {
		failure(w, http.StatusBadRequest, errors.New("expected exactly one file part named file with a filename"))
		return false
	}
	if !store(params["filename"], uploadContextReader{r.Context(), part}) {
		return false
	}
	if _, err := reader.NextRawPart(); err != io.EOF {
		if err == nil {
			failure(w, http.StatusBadRequest, errors.New("expected exactly one file part named file; extra multipart part received"))
		} else {
			uploadReadFailure(w, "finish upload multipart body", err)
		}
		return false
	}
	// Multipart permits an epilogue. Drain it through the bounded body reader so
	// even chunked requests cannot bypass the total request size limit.
	if _, err := io.Copy(io.Discard, uploadContextReader{r.Context(), r.Body}); err != nil {
		uploadReadFailure(w, "finish upload request", err)
		return false
	}
	if err := r.Context().Err(); err != nil {
		uploadReadFailure(w, "upload cancelled", err)
		return false
	}
	return true
}

// ensureArtifacts creates the session's interaction artifacts folder when
// needed and returns its path; an exited session keeps its folder until it
// is removed.
func (s *Server) ensureArtifacts(w http.ResponseWriter, r *http.Request) {
	path, err := s.Artifacts.Ensure(r.PathValue("id"))
	if err != nil {
		artifactsFailure(w, err)
		return
	}
	reply(w, http.StatusOK, contract.ArtifactsFolder{Path: path})
}

type uploadContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r uploadContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p)
	if cancelled := r.ctx.Err(); cancelled != nil {
		return n, cancelled
	}
	return n, err
}

func uploadReadFailure(w http.ResponseWriter, operation string, err error) {
	failure(w, uploadReadStatus(err), fmt.Errorf("%s: %w", operation, err))
}
