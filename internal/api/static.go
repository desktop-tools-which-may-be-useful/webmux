package api

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"math"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"

	assets "github.com/rainchan/webmux/web"
)

// The inventory is fixed by the embedded build. Request paths and query strings
// never add entries, and each file's representations are initialized once.
var staticBuild, staticBuildErr = embeddedStaticFiles()

type staticFiles struct {
	root  fs.FS
	files map[string]*staticAsset
}

// staticAsset is one served path. web/scripts/embed-assets.ts embeds
// compressible files only as name.br and name.gz, compressed at build time, so
// serving never compresses; the identity body is inflated from gzip on demand.
type staticAsset struct {
	once        sync.Once
	name        string // embedded identity file; empty when only variants exist
	brotliName  string
	gzipName    string
	data        []byte
	brotli      []byte
	gzip        []byte
	contentType string
	err         error

	inflateOnce sync.Once
	inflateErr  error
}

func embeddedStaticFiles() (*staticFiles, error) {
	root, err := fs.Sub(assets.Assets, "dist")
	if err != nil {
		return nil, err
	}
	return newStaticFiles(root)
}

func newStaticFiles(root fs.FS) (*staticFiles, error) {
	files := &staticFiles{root: root, files: make(map[string]*staticAsset)}
	asset := func(name string) *staticAsset {
		a := files.files[name]
		if a == nil {
			a = &staticAsset{contentType: staticContentType(name)}
			files.files[name] = a
		}
		return a
	}
	err := fs.WalkDir(root, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		ext := path.Ext(name)
		base := strings.TrimSuffix(name, ext)
		switch {
		case ext == ".br" && compressibleStaticType(staticContentType(base)):
			asset(base).brotliName = name
		case ext == ".gz" && compressibleStaticType(staticContentType(base)):
			asset(base).gzipName = name
		default:
			asset(name).name = name
		}
		return nil
	})
	return files, err
}

func (files *staticFiles) get(name string) (*staticAsset, error) {
	asset, ok := files.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	asset.once.Do(func() {
		read := func(name string) []byte {
			if name == "" || asset.err != nil {
				return nil
			}
			var data []byte
			data, asset.err = fs.ReadFile(files.root, name)
			return data
		}
		asset.data = read(asset.name)
		asset.gzip = read(asset.gzipName)
		asset.brotli = read(asset.brotliName)
		if asset.err == nil && asset.name == "" && asset.gzip == nil {
			asset.err = fs.ErrNotExist
		}
	})
	return asset, asset.err
}

// identity returns the uncompressed body, inflating the gzip variant once
// when the build embedded no identity file.
func (asset *staticAsset) identity() ([]byte, error) {
	if asset.name != "" {
		return asset.data, nil
	}
	asset.inflateOnce.Do(func() {
		var zr *gzip.Reader
		zr, asset.inflateErr = gzip.NewReader(bytes.NewReader(asset.gzip))
		if asset.inflateErr != nil {
			return
		}
		asset.data, asset.inflateErr = io.ReadAll(zr)
	})
	return asset.data, asset.inflateErr
}

func staticContentType(name string) string {
	switch path.Ext(name) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".webmanifest":
		return "application/manifest+json"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".woff2":
		return "font/woff2"
	case ".wasm":
		// WebAssembly.instantiateStreaming rejects any other type.
		return "application/wasm"
	case ".txt":
		return "text/plain; charset=utf-8"
	default:
		return ""
	}
}

func compressibleStaticType(contentType string) bool {
	return strings.HasPrefix(contentType, "text/") || contentType == "application/manifest+json" ||
		contentType == "image/svg+xml" || contentType == "application/wasm"
}

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if staticBuildErr != nil {
		http.Error(w, "frontend build unavailable", http.StatusServiceUnavailable)
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}
	if strings.HasPrefix(name, ".") || strings.Contains(name, "/.") {
		http.NotFound(w, r)
		return
	}
	asset, err := staticBuild.get(name)
	if err != nil {
		if strings.HasPrefix(name, "assets/") || path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		name = "index.html"
		asset, err = staticBuild.get(name)
	}
	if err != nil {
		http.Error(w, "run make web before building", http.StatusServiceUnavailable)
		return
	}
	if asset.contentType != "" {
		w.Header().Set("Content-Type", asset.contentType)
	}
	// Font filenames include a content hash, so they can be cached as immutable.
	if strings.HasPrefix(name, "assets/") || path.Ext(name) == ".woff2" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	asset.write(w, r)
}

func (asset *staticAsset) write(w http.ResponseWriter, r *http.Request) {
	q := staticEncodingQualities(strings.Join(r.Header.Values("Accept-Encoding"), ","))
	if asset.gzip != nil || asset.brotli != nil {
		w.Header().Add("Vary", "Accept-Encoding")
	}
	// Highest quality wins; ties prefer the smaller body (br, gzip, identity).
	body, encoding, best := []byte(nil), "", q.identity
	if asset.gzip != nil && q.gzip > 0 && q.gzip >= best {
		body, encoding, best = asset.gzip, "gzip", q.gzip
	}
	if asset.brotli != nil && q.br > 0 && q.br >= best {
		body, encoding = asset.brotli, "br"
	}
	if encoding != "" {
		w.Header().Set("Content-Encoding", encoding)
		_, _ = w.Write(body)
		return
	}
	if q.identity <= 0 {
		// A rejected representation must not inherit the immutable asset cache policy.
		w.Header().Del("Cache-Control")
		http.Error(w, "no acceptable content encoding", http.StatusNotAcceptable)
		return
	}
	data, err := asset.identity()
	if err != nil {
		w.Header().Del("Cache-Control")
		http.Error(w, "corrupt embedded asset", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(data)
}

type encodingQualities struct{ br, gzip, identity float64 }

func staticEncodingQualities(header string) encodingQualities {
	result := encodingQualities{identity: 1}
	qualities := make(map[string]float64, 4)
	for _, item := range strings.Split(header, ",") {
		parts := strings.Split(item, ";")
		coding := strings.ToLower(strings.TrimSpace(parts[0]))
		if coding != "br" && coding != "gzip" && coding != "identity" && coding != "*" {
			continue
		}
		quality := 1.0
		for _, parameter := range parts[1:] {
			key, value, ok := strings.Cut(parameter, "=")
			if strings.EqualFold(strings.TrimSpace(key), "q") {
				parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
				if !ok || err != nil || math.IsNaN(parsed) || parsed < 0 || parsed > 1 {
					quality = 0
				} else {
					quality = parsed
				}
			}
		}
		qualities[coding] = quality
	}
	if wildcard, ok := qualities["*"]; ok {
		result.br, result.gzip = wildcard, wildcard
		if wildcard == 0 {
			result.identity = 0
		}
	}
	if quality, ok := qualities["br"]; ok {
		result.br = quality
	}
	if quality, ok := qualities["gzip"]; ok {
		result.gzip = quality
	}
	if quality, ok := qualities["identity"]; ok {
		result.identity = quality
	}
	return result
}
