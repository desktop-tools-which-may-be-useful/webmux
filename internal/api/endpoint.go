package api

import "net/http"

// failer answers a service error with its HTTP status and body.
type failer func(w http.ResponseWriter, err error)

// serve answers a JSON route that takes no body. The handler returns the
// contract value to send with status; archtest rule R8 checks T at every call.
func serve[T any](mux *http.ServeMux, pattern string, status int, fail failer, fn func(r *http.Request) (T, error)) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		v, err := fn(r)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, status, v)
	})
}

// serveIn is serve for a route that decodes a JSON body of type Req first.
func serveIn[Req, T any](mux *http.ServeMux, pattern string, status int, fail failer, fn func(r *http.Request, req Req) (T, error)) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		var req Req
		if !decode(w, r, &req) {
			return
		}
		v, err := fn(r, req)
		if err != nil {
			fail(w, err)
			return
		}
		reply(w, status, v)
	})
}

// serveDone answers 204 for a route that takes no body and returns nothing.
func serveDone(mux *http.ServeMux, pattern string, fail failer, fn func(r *http.Request) error) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if err := fn(r); err != nil {
			fail(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// serveInDone answers 204 for a route that decodes a JSON body of type Req.
func serveInDone[Req any](mux *http.ServeMux, pattern string, fail failer, fn func(r *http.Request, req Req) error) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		var req Req
		if !decode(w, r, &req) {
			return
		}
		if err := fn(r, req); err != nil {
			fail(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// badRequest marks an error a handler found in the request itself, which the
// failer answers as 400 instead of classifying it as a service error.
type badRequest struct{ err error }

func (e *badRequest) Error() string { return e.err.Error() }
func (e *badRequest) Unwrap() error { return e.err }
