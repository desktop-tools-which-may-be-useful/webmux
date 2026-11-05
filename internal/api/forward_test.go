package api

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/forward"
	"github.com/rainchan/webmux/internal/session"
)

// upstreamRequest is what the dev server saw.
type upstreamRequest struct {
	path, host string
	header     http.Header
}

type gatewayEnv struct {
	t        *testing.T
	forwards *forward.Service
	public   *PublicOrigins
	server   *httptest.Server
	port     int

	mu   sync.Mutex
	seen []upstreamRequest
}

// newGatewayEnv starts WebMux with token "secret" and a dev server whose
// handler is upstream (a recorder answering 200 when nil).
func newGatewayEnv(t *testing.T, token string, upstream http.HandlerFunc) *gatewayEnv {
	t.Helper()
	env := &gatewayEnv{t: t}
	dev := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.mu.Lock()
		env.seen = append(env.seen, upstreamRequest{path: r.URL.RequestURI(), host: r.Host, header: r.Header.Clone()})
		env.mu.Unlock()
		if upstream != nil {
			upstream(w, r)
			return
		}
		_, _ = io.WriteString(w, "dev page")
	}))
	t.Cleanup(dev.Close)
	env.port = dev.Listener.Addr().(*net.TCPAddr).Port
	env.forwards = forward.New(forward.Config{ExcludePorts: []int{7681}, ProbeInterval: time.Hour})
	t.Cleanup(env.forwards.Close)
	env.public = &PublicOrigins{}
	m := session.NewManager(session.Options{})
	t.Cleanup(func() { m.Shutdown(context.Background()) })
	handler := New(testDeps(t, Deps{Config: config.Config{Token: token}, Manager: m, Workspace: testWorkspace(t, m, t.TempDir()), Forward: env.forwards, Public: env.public}))
	env.server = httptest.NewServer(handler)
	t.Cleanup(env.server.Close)
	return env
}

func (e *gatewayEnv) add(port int, options *forward.Options) {
	e.t.Helper()
	if _, err := e.forwards.Add(e.t.Context(), forward.Input{Port: port, Options: options}); err != nil {
		e.t.Fatal(err)
	}
}

func (e *gatewayEnv) last() upstreamRequest {
	e.t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.seen) == 0 {
		e.t.Fatal("upstream received no request")
	}
	return e.seen[len(e.seen)-1]
}

func (e *gatewayEnv) requests() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.seen)
}

func (e *gatewayEnv) path(rest string) string {
	return "/forward/" + strconv.Itoa(e.port) + "/" + rest
}

// do sends a request without following redirects.
func (e *gatewayEnv) do(method, path string, body io.Reader, edit func(*http.Request)) *http.Response {
	e.t.Helper()
	r, err := http.NewRequestWithContext(e.t.Context(), method, e.server.URL+path, body)
	if err != nil {
		e.t.Fatal(err)
	}
	if edit != nil {
		edit(r)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(r)
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { _ = res.Body.Close() })
	return res
}

func bearer(r *http.Request) { r.Header.Set("Authorization", "Bearer secret") }

func (e *gatewayEnv) ticket() string {
	e.t.Helper()
	res := e.do(http.MethodPost, "/api/forwards/tickets", nil, bearer)
	if res.StatusCode != http.StatusCreated {
		e.t.Fatalf("ticket status %d", res.StatusCode)
	}
	var ticket contract.ForwardTicket
	if err := json.NewDecoder(res.Body).Decode(&ticket); err != nil {
		e.t.Fatal(err)
	}
	return ticket.Code
}

func (e *gatewayEnv) redeem(code, next string, edit func(*http.Request)) *http.Response {
	e.t.Helper()
	form := url.Values{"code": {code}, "next": {next}}
	return e.do(http.MethodPost, "/forward/_auth", strings.NewReader(form.Encode()), func(r *http.Request) {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", e.server.URL)
		if edit != nil {
			edit(r)
		}
	})
}

// cookie redeems a fresh ticket and returns the forwarding cookie.
func (e *gatewayEnv) cookie() *http.Cookie {
	e.t.Helper()
	res := e.redeem(e.ticket(), e.path(""), nil)
	for _, c := range res.Cookies() {
		if c.Name == forwardCookie {
			return c
		}
	}
	e.t.Fatalf("no forwarding cookie (status %d)", res.StatusCode)
	return nil
}

func withCookie(c *http.Cookie) func(*http.Request) {
	return func(r *http.Request) { r.AddCookie(c) }
}

func TestForwardAPI(t *testing.T) {
	env := newGatewayEnv(t, "secret", nil)
	post := func(body string) int {
		return env.do(http.MethodPost, "/api/forwards", strings.NewReader(body), bearer).StatusCode
	}
	for _, row := range []struct {
		body string
		want int
	}{
		{`{"port":0}`, 400},
		{`{"port":65536}`, 400},
		{`{"port":7681}`, 400},
		{`{"port":5173,"options":{"rewriteHost":true,"rewriteOrigin":true,"forwardedHeaders":true,"rewriteResponse":true,"prefix":"both"}}`, 400},
		{`{"port":5173}`, 201},
		{`{"port":5173}`, 409},
	} {
		if got := post(row.body); got != row.want {
			t.Errorf("POST %s = %d, want %d", row.body, got, row.want)
		}
	}
	for _, row := range []struct {
		method, path string
		want         int
	}{
		{http.MethodPatch, "/api/forwards/8000", 404},
		{http.MethodPatch, "/api/forwards/05173", 404},
		{http.MethodDelete, "/api/forwards/8000", 404},
		{http.MethodPatch, "/api/forwards/5173", 200},
		{http.MethodDelete, "/api/forwards/5173", 204},
	} {
		if got := env.do(row.method, row.path, strings.NewReader(`{"label":"web"}`), bearer).StatusCode; got != row.want {
			t.Errorf("%s %s = %d, want %d", row.method, row.path, got, row.want)
		}
	}
}

func TestForwardCookieCannotReachAPI(t *testing.T) {
	env := newGatewayEnv(t, "secret", nil)
	env.add(env.port, nil)
	cookie := env.cookie()
	for _, path := range []string{"/api/sessions", "/api/forwards", "/api/forwards/tickets"} {
		method := http.MethodGet
		if strings.HasSuffix(path, "tickets") {
			method = http.MethodPost
		}
		if got := env.do(method, path, nil, withCookie(cookie)).StatusCode; got != http.StatusUnauthorized {
			t.Errorf("%s %s with forwarding cookie = %d, want 401", method, path, got)
		}
	}
	_, res, err := websocket.Dial(t.Context(), "ws"+strings.TrimPrefix(env.server.URL, "http")+"/ws/sessions", &websocket.DialOptions{
		HTTPHeader: http.Header{"Cookie": {cookie.String()}}, Subprotocols: []string{wsProtocol},
	})
	if err == nil || res == nil || res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("/ws/sessions with forwarding cookie: %v %v", res, err)
	}
	if got := env.do(http.MethodGet, env.path(""), nil, withCookie(cookie)).StatusCode; got != http.StatusOK {
		t.Fatalf("gateway with cookie = %d", got)
	}
}

func TestForwardRedeem(t *testing.T) {
	env := newGatewayEnv(t, "secret", nil)
	env.add(env.port, nil)

	code := env.ticket()
	res := env.redeem(code, env.path("?page=2"), nil)
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != env.path("?page=2") {
		t.Fatalf("redeem = %d %q", res.StatusCode, res.Header.Get("Location"))
	}
	set := res.Header.Get("Set-Cookie")
	for _, attr := range []string{forwardCookie + "=", "Path=/forward/", "Max-Age=43200", "HttpOnly", "SameSite=Strict"} {
		if !strings.Contains(set, attr) {
			t.Errorf("Set-Cookie %q lacks %q", set, attr)
		}
	}
	if strings.Contains(set, "Secure") || strings.Contains(set, "secret") {
		t.Errorf("plain HTTP cookie %q", set)
	}
	if got := env.redeem(code, env.path(""), nil); got.StatusCode != http.StatusUnauthorized || len(got.Cookies()) != 0 {
		t.Fatalf("reused ticket = %d", got.StatusCode)
	}

	for _, next := range []string{"https://evil.example/", "//evil.example/", "/forward//evil.example/", "/forward/\\evil.example", "/api/sessions", "", "/forward/5173/\r\nX: y"} {
		ticket := env.ticket()
		if got := env.redeem(ticket, next, nil); got.StatusCode != http.StatusBadRequest || len(got.Cookies()) != 0 {
			t.Errorf("next %q = %d", next, got.StatusCode)
		}
	}
	for name, origin := range map[string]string{"cross site": "https://evil.example", "missing": ""} {
		ticket := env.ticket()
		got := env.redeem(ticket, env.path(""), func(r *http.Request) {
			r.Header.Del("Origin")
			if origin != "" {
				r.Header.Set("Origin", origin)
			}
		})
		if got.StatusCode != http.StatusForbidden || len(got.Cookies()) != 0 {
			t.Errorf("%s origin = %d", name, got.StatusCode)
		}
	}

	// A public HTTPS entry point proxies plain HTTP: the cookie is Secure.
	if err := env.public.Set("cloudflare", "https://abc.trycloudflare.com"); err != nil {
		t.Fatal(err)
	}
	ticket := env.ticket()
	got := env.redeem(ticket, env.path(""), func(r *http.Request) {
		r.Host = "abc.trycloudflare.com"
		r.Header.Set("Origin", "https://abc.trycloudflare.com")
	})
	if got.StatusCode != http.StatusSeeOther || !strings.Contains(got.Header.Get("Set-Cookie"), "Secure") {
		t.Fatalf("public redeem = %d %q", got.StatusCode, got.Header.Get("Set-Cookie"))
	}
}

func TestGatewayRouting(t *testing.T) {
	env := newGatewayEnv(t, "secret", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Service-Worker-Allowed", "/")
		w.Header().Set("Referrer-Policy", "origin")
		_, _ = io.WriteString(w, "dev page")
	})
	env.add(env.port, nil)
	cookie := env.cookie()
	authed := withCookie(cookie)

	for _, path := range []string{"/forward", "/forward/", "/forward/0/", "/forward/65536/", "/forward/05173/", "/forward/abc/", "/forward/8080/"} {
		if got := env.do(http.MethodGet, path, nil, authed).StatusCode; got != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", path, got)
		}
	}
	if env.requests() != 0 {
		t.Fatal("a rejected path reached the upstream")
	}

	res := env.do(http.MethodGet, "/forward/"+strconv.Itoa(env.port)+"?x=1", nil, nil)
	if res.StatusCode != http.StatusPermanentRedirect || res.Header.Get("Location") != env.path("?x=1") {
		t.Fatalf("slash redirect = %d %q", res.StatusCode, res.Header.Get("Location"))
	}

	res = env.do(http.MethodGet, env.path("a?b=1"), nil, func(r *http.Request) { r.Header.Set("Sec-Fetch-Mode", "navigate") })
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/forward-auth?next="+url.QueryEscape(env.path("a?b=1")) {
		t.Fatalf("navigate without cookie = %d %q", res.StatusCode, res.Header.Get("Location"))
	}
	if got := env.do(http.MethodGet, env.path("api/data"), nil, nil).StatusCode; got != http.StatusUnauthorized {
		t.Fatalf("subresource without cookie = %d", got)
	}
	stale := &http.Cookie{Name: forwardCookie, Value: "stale"}
	if got := env.do(http.MethodGet, env.path(""), nil, withCookie(stale)).StatusCode; got != http.StatusUnauthorized {
		t.Fatalf("stale cookie = %d", got)
	}
	if env.requests() != 0 {
		t.Fatal("an unauthenticated request reached the upstream")
	}

	res = env.do(http.MethodGet, env.path("page"), nil, func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: "session", Value: "abc"})
		r.AddCookie(cookie)
		r.Header.Set("Authorization", "Basic dev")
	})
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || string(body) != "dev page" {
		t.Fatalf("proxied = %d %q", res.StatusCode, body)
	}
	seen := env.last()
	if got := seen.header.Values("Cookie"); len(got) != 1 || got[0] != "session=abc" {
		t.Errorf("upstream cookies %q", got)
	}
	if seen.header.Get("Authorization") != "Basic dev" {
		t.Errorf("upstream Authorization %q", seen.header.Get("Authorization"))
	}
	if res.Header.Get("Service-Worker-Allowed") != "" {
		t.Error("Service-Worker-Allowed reached the browser")
	}
	if got := res.Header.Values("Referrer-Policy"); len(got) != 1 || got[0] != "origin" {
		t.Errorf("Referrer-Policy %q, want the upstream's", got)
	}
	if res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("nosniff dropped")
	}
}

func TestGatewayHidesForwardedPortsFromUnauthenticated(t *testing.T) {
	env := newGatewayEnv(t, "secret", nil)
	env.add(env.port, nil)
	for _, path := range []string{env.path("x"), "/forward/1/x"} {
		if got := env.do(http.MethodGet, path, nil, nil).StatusCode; got != http.StatusUnauthorized {
			t.Errorf("unauthenticated %s = %d, want 401 whether or not it is forwarded", path, got)
		}
	}
	// Plain HTTP to a LAN address carries no Sec-Fetch-*: an HTML page load
	// still reaches the sign-in page; other requests do not.
	res := env.do(http.MethodGet, env.path(""), nil, func(r *http.Request) { r.Header.Set("Accept", "text/html,application/xhtml+xml") })
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("HTML load without Sec-Fetch-Mode = %d", res.StatusCode)
	}
	res = env.do(http.MethodGet, env.path(""), nil, func(r *http.Request) {
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Sec-Fetch-Mode", "cors")
	})
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("fetch accepting HTML = %d", res.StatusCode)
	}
}

func TestForwardAuthPageConfirmsForeignLoads(t *testing.T) {
	env := newGatewayEnv(t, "secret", nil)
	next := "/forward-auth?next=" + url.QueryEscape("/forward/5173/")
	for _, row := range []struct {
		site     string
		redirect bool
	}{
		{"same-origin", false},
		{"none", false},
		{"cross-site", true},
		{"same-site", true},
		{"", true},
	} {
		res := env.do(http.MethodGet, next, nil, func(r *http.Request) {
			if row.site != "" {
				r.Header.Set("Sec-Fetch-Site", row.site)
			}
		})
		if res.Header.Get("X-Frame-Options") != "DENY" || !strings.Contains(res.Header.Get("Content-Security-Policy"), "frame-ancestors 'none'") {
			t.Errorf("site %q: page may be framed", row.site)
		}
		if !row.redirect {
			if res.StatusCode != http.StatusOK {
				t.Errorf("site %q = %d, want the page", row.site, res.StatusCode)
			}
			continue
		}
		want := "/forward-auth?confirm=1&next=" + url.QueryEscape("/forward/5173/")
		if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != want {
			t.Errorf("site %q = %d %q, want 303 %q", row.site, res.StatusCode, res.Header.Get("Location"), want)
		}
	}
	res := env.do(http.MethodGet, "/forward-auth?confirm=1&next=%2Fforward%2F5173%2F", nil, func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") })
	if res.StatusCode != http.StatusOK {
		t.Fatalf("confirmed foreign load = %d", res.StatusCode)
	}
}

func TestGatewayHostCheck(t *testing.T) {
	forwards := forward.New(forward.Config{ProbeInterval: time.Hour})
	t.Cleanup(forwards.Close)
	m := session.NewManager(session.Options{})
	t.Cleanup(func() { m.Shutdown(context.Background()) })
	handler := New(testDeps(t, Deps{Config: config.Config{Listen: "127.0.0.1:7681"}, Manager: m, Workspace: testWorkspace(t, m, t.TempDir()), Forward: forwards}))
	r := httptest.NewRequest(http.MethodGet, "http://rebound.example/forward/5173/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("rebound host = %d, want 403", w.Code)
	}
}

func TestGatewayWithoutToken(t *testing.T) {
	env := newGatewayEnv(t, "", nil)
	env.add(env.port, nil)
	if got := env.do(http.MethodGet, env.path(""), nil, nil).StatusCode; got != http.StatusOK {
		t.Fatalf("tokenless gateway = %d", got)
	}
}

func TestGatewayPrefixModes(t *testing.T) {
	env := newGatewayEnv(t, "secret", nil)
	cookie := withCookie(func() *http.Cookie { env.add(env.port, nil); return env.cookie() }())
	env.do(http.MethodGet, env.path("@vite/client?t=1"), nil, cookie)
	if got := env.last().path; got != env.path("@vite/client?t=1") {
		t.Fatalf("keep sent %q", got)
	}
	strip := forward.DefaultOptions()
	strip.Prefix = forward.PrefixStrip
	if _, err := env.forwards.Update(env.port, forward.Update{Options: &strip}); err != nil {
		t.Fatal(err)
	}
	env.do(http.MethodGet, env.path("docs/index.html?q=1"), nil, cookie)
	if got := env.last().path; got != "/docs/index.html?q=1" {
		t.Fatalf("strip sent %q", got)
	}
	env.do(http.MethodGet, env.path(""), nil, cookie)
	if got := env.last().path; got != "/" {
		t.Fatalf("strip root sent %q", got)
	}
}

func TestGatewayUnreachable(t *testing.T) {
	env := newGatewayEnv(t, "secret", nil)
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	env.add(port, nil)
	cookie := env.cookie()
	res := env.do(http.MethodGet, "/forward/"+strconv.Itoa(port)+"/", nil, withCookie(cookie))
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusBadGateway || !strings.Contains(string(body), strconv.Itoa(port)) || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/plain") {
		t.Fatalf("unreachable = %d %q", res.StatusCode, body)
	}
}

func TestGatewayStreamsEvents(t *testing.T) {
	release := make(chan struct{})
	env := newGatewayEnv(t, "secret", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: one\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
		_, _ = io.WriteString(w, "data: two\n\n")
	})
	env.add(env.port, nil)
	res := env.do(http.MethodGet, env.path("events"), nil, withCookie(env.cookie()))
	lines := bufio.NewReader(res.Body)
	first, err := lines.ReadString('\n')
	if err != nil || first != "data: one\n" {
		t.Fatalf("first event %q %v (the gateway buffered the stream)", first, err)
	}
	close(release)
}

func TestGatewayWebSocket(t *testing.T) {
	env := newGatewayEnv(t, "secret", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		for {
			kind, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			if c.Write(r.Context(), kind, append([]byte("echo "), data...)) != nil {
				return
			}
		}
	})
	env.add(env.port, nil)
	cookie := env.cookie()
	wsURL := "ws" + strings.TrimPrefix(env.server.URL, "http") + env.path("hmr")
	dial := func(origin string) (*websocket.Conn, *http.Response, error) {
		return websocket.Dial(t.Context(), wsURL, &websocket.DialOptions{HTTPHeader: http.Header{"Cookie": {cookie.String()}, "Origin": {origin}}})
	}

	before := env.requests()
	if _, res, err := dial("https://evil.example"); err == nil || res == nil || res.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-site upgrade: %v %v", res, err)
	}
	if env.requests() != before {
		t.Fatal("cross-site upgrade reached the upstream")
	}

	c, _, err := dial(env.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.CloseNow() }()
	if seen := env.last(); seen.header.Get("Origin") != "http://localhost:"+strconv.Itoa(env.port) {
		t.Errorf("upstream Origin %q", seen.header.Get("Origin"))
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, []byte("ping")); err != nil {
		t.Fatal(err)
	}
	if _, data, err := c.Read(ctx); err != nil || string(data) != "echo ping" {
		t.Fatalf("echo %q %v", data, err)
	}

	if err := env.forwards.Remove(env.port); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.Read(ctx); err == nil || ctx.Err() != nil {
		t.Fatalf("connection survived removal: %v", err)
	}
	if got := env.do(http.MethodGet, env.path(""), nil, withCookie(cookie)).StatusCode; got != http.StatusNotFound {
		t.Fatalf("removed forward = %d", got)
	}
}

func TestGatewayRewrites(t *testing.T) {
	var reply func(w http.ResponseWriter)
	env := newGatewayEnv(t, "secret", func(w http.ResponseWriter, r *http.Request) {
		if reply != nil {
			reply(w)
		}
	})
	env.add(env.port, nil)
	cookie := env.cookie()
	gatewayHost := strings.TrimPrefix(env.server.URL, "http://")
	local := "localhost:" + strconv.Itoa(env.port)
	prefix := "/forward/" + strconv.Itoa(env.port)

	set := func(o forward.Options) {
		t.Helper()
		if _, err := env.forwards.Update(env.port, forward.Update{Options: &o}); err != nil {
			t.Fatal(err)
		}
	}
	with := func(edit func(*forward.Options)) forward.Options {
		o := forward.DefaultOptions()
		edit(&o)
		return o
	}
	send := func() (*http.Response, upstreamRequest) {
		res := env.do(http.MethodGet, env.path("p"), nil, func(r *http.Request) {
			r.AddCookie(cookie)
			r.Header.Set("Origin", env.server.URL)
			r.Header.Set("Referer", env.server.URL+env.path("index.html"))
			r.Header.Set("X-Forwarded-Host", "spoofed.example")
			r.Header.Set("X-Forwarded-Prefix", "/spoofed")
		})
		return res, env.last()
	}

	t.Run("host", func(t *testing.T) {
		set(forward.DefaultOptions())
		if _, seen := send(); seen.host != local {
			t.Errorf("on: Host %q", seen.host)
		}
		set(with(func(o *forward.Options) { o.RewriteHost = false }))
		if _, seen := send(); seen.host != gatewayHost {
			t.Errorf("off: Host %q, want %q", seen.host, gatewayHost)
		}
	})
	t.Run("origin", func(t *testing.T) {
		set(forward.DefaultOptions())
		_, seen := send()
		if seen.header.Get("Origin") != "http://"+local || seen.header.Get("Referer") != "http://"+local+env.path("index.html") {
			t.Errorf("on: Origin %q Referer %q", seen.header.Get("Origin"), seen.header.Get("Referer"))
		}
		set(with(func(o *forward.Options) { o.RewriteOrigin = false }))
		_, seen = send()
		if seen.header.Get("Origin") != env.server.URL || seen.header.Get("Referer") != env.server.URL+env.path("index.html") {
			t.Errorf("off: Origin %q Referer %q", seen.header.Get("Origin"), seen.header.Get("Referer"))
		}
	})
	t.Run("forwarded headers", func(t *testing.T) {
		set(forward.DefaultOptions())
		_, seen := send()
		if seen.header.Get("X-Forwarded-Host") != gatewayHost || seen.header.Get("X-Forwarded-Proto") != "http" || seen.header.Get("X-Forwarded-Prefix") != prefix {
			t.Errorf("on: %v", seen.header)
		}
		set(with(func(o *forward.Options) { o.ForwardedHeaders = false }))
		_, seen = send()
		if seen.header.Get("X-Forwarded-Host") != "" || seen.header.Get("X-Forwarded-Prefix") != "" {
			t.Errorf("off: forwarded headers %v", seen.header)
		}
	})
	t.Run("response", func(t *testing.T) {
		reply = func(w http.ResponseWriter) {
			w.Header().Set("Location", "http://"+local+"/login?x=1")
			w.Header().Set("Access-Control-Allow-Origin", "http://"+local)
			w.Header().Add("Set-Cookie", "sid=1; Path=/; Domain=localhost; HttpOnly")
			w.Header().Add("Set-Cookie", "pref=2; Path="+prefix+"/app")
			w.WriteHeader(http.StatusFound)
		}
		defer func() { reply = nil }()
		set(with(func(o *forward.Options) { o.Prefix = forward.PrefixStrip }))
		res, _ := send()
		if got := res.Header.Get("Location"); got != prefix+"/login?x=1" {
			t.Errorf("on: Location %q", got)
		}
		if got := res.Header.Get("Access-Control-Allow-Origin"); got != env.server.URL {
			t.Errorf("on: ACAO %q", got)
		}
		cookies := res.Header.Values("Set-Cookie")
		if len(cookies) != 2 || cookies[0] != "sid=1; Path="+prefix+"/; HttpOnly" || cookies[1] != "pref=2; Path="+prefix+"/app" {
			t.Errorf("on: Set-Cookie %q", cookies)
		}

		set(with(func(o *forward.Options) { o.RewriteResponse = false }))
		res, _ = send()
		if got := res.Header.Get("Location"); got != "http://"+local+"/login?x=1" {
			t.Errorf("off: Location %q", got)
		}
		if got := res.Header.Get("Access-Control-Allow-Origin"); got != "http://"+local {
			t.Errorf("off: ACAO %q", got)
		}
		if got := res.Header.Values("Set-Cookie"); len(got) != 2 || got[0] != "sid=1; Path=/; Domain=localhost; HttpOnly" {
			t.Errorf("off: Set-Cookie %q", got)
		}
	})
}

func TestRewriteLocation(t *testing.T) {
	for _, row := range []struct {
		in    string
		strip bool
		want  string
	}{
		{"http://localhost:5173/login", true, "/forward/5173/login"},
		{"http://127.0.0.1:5173/forward/5173/x?y=1", false, "/forward/5173/x?y=1"},
		{"/login", true, "/forward/5173/login"},
		{"/forward/5173/login", false, "/forward/5173/login"},
		{"https://example.com/", true, "https://example.com/"},
		{"http://localhost:5174/", true, "http://localhost:5174/"},
		{"//example.com/", true, "//example.com/"},
		{"next", true, "next"},
	} {
		if got := rewriteLocation(row.in, 5173, "/forward/5173", row.strip); got != row.want {
			t.Errorf("rewriteLocation(%q, strip=%v) = %q, want %q", row.in, row.strip, got, row.want)
		}
	}
}

func TestForwardPortParsing(t *testing.T) {
	for raw, want := range map[string]bool{"1": true, "65535": true, "5173": true, "0": false, "05173": false, "65536": false, "-1": false, "+1": false, "1e3": false, "": false, "123456": false} {
		if _, ok := forwardPort(raw); ok != want {
			t.Errorf("forwardPort(%q) ok=%v, want %v", raw, ok, want)
		}
	}
}
