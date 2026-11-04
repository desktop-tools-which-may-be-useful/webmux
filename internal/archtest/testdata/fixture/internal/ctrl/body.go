package ctrl

import "net/http"

// readJSON is a registered body reader, so its reads are compliant.
func readJSON(r *http.Request) {
	_ = r.Body
	r.Body = http.NoBody
	_ = r.ParseForm()
}

// unusedReader is registered but reads nothing, which makes the entry stale.
func unusedReader(r *http.Request) { _ = r.Header }

type wrapped struct{ *http.Request }

func leak(r *http.Request, w wrapped) {
	_ = r.Body           // want R11
	r.Body = http.NoBody // want R11
	_ = r.ParseForm()    // want R11
	parse := r.FormFile  // want R11
	_, _, _ = parse("file")
	_ = r.FormValue("x")                // want R11
	_, _ = r.MultipartReader()          // want R11
	_ = r.PostFormValue("x")            // want R11
	_ = w.Body                          // want R11
	_ = w.Request.ParseMultipartForm(1) // want R11
	// Compliant: other request data and response bodies.
	_ = r.Header
	_ = r.Context()
	_ = r.URL.Query()
	_ = r.PathValue("id")
	var response http.Response
	_ = response.Body
}

var viaInitializer = func(r *http.Request) { _ = r.Body } // want R11
