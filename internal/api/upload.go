package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
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
	upload, received := s.storeUpload(w, r, func() (string, error) { return s.Artifacts.EnsureUploads(item.Info().ID) })
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

// storeUpload receives one file into the folder dir returns, answering
// every failure. The caller discards the file when it reports false.
func (s *Server) storeUpload(w http.ResponseWriter, r *http.Request, dir func() (string, error)) (*files.Upload, bool) {
	var upload *files.Upload
	received := receiveUpload(w, r, func(filename string, src io.Reader) bool {
		folder, err := dir()
		if err != nil {
			artifactsFailure(w, err)
			return false
		}
		upload, err = s.Files.StoreUpload(folder, filename, src, maxUploadBytes)
		if err != nil {
			filesFailure(w, err)
			return false
		}
		return true
	})
	return upload, received
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

// readUpload serves one uploaded file from the session's uploads/ folder.
func (s *Server) readUpload(w http.ResponseWriter, r *http.Request) {
	file, err := s.Artifacts.ReadUpload(r.PathValue("id"), r.URL.Query().Get("path"))
	if err != nil {
		artifactsFailure(w, err)
		return
	}
	h := w.Header()
	contentType := "application/octet-stream"
	if file.MediaType != "" {
		contentType = file.MediaType
	}
	h.Set("Content-Type", contentType)
	h.Set("Content-Length", strconv.Itoa(len(file.Data)))
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Disposition", contentDisposition("inline", file.Name))
	if file.MediaType == "image/svg+xml" || file.MediaType == "application/pdf" {
		h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	}
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(file.Data)
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
