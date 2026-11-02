package main

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/api"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/listensettings"
)

func TestStartupListenHonorsSavedSwitchAndFlag(t *testing.T) {
	state := t.TempDir()
	store := listensettings.Open(state, "127.0.0.1:7681")
	if got := startupListen("127.0.0.1:7681", "toml", store); got != "127.0.0.1:7681" {
		t.Fatalf("default %q", got)
	}
	if err := store.Save(listensettings.Settings{Public: true}); err != nil {
		t.Fatal(err)
	}
	if got := startupListen("127.0.0.1:7681", "toml", store); got != "0.0.0.0:7681" {
		t.Fatalf("saved public %q", got)
	}
	if got := startupListen("127.0.0.1:9000", "cli", store); got != "127.0.0.1:9000" {
		t.Fatalf("explicit flag %q", got)
	}
	store.Close()
}

func TestNetworkControllerRebindsWithoutDroppingConnections(t *testing.T) {
	state := t.TempDir()
	store := listensettings.Open(state, "127.0.0.1:0")
	defer store.Close()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "ok")
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	ended := make(chan error, 1)
	hosts := api.NewListen(config.Config{Listen: ln.Addr().String()})
	n := newNetworkController(networkOptions{
		Server: server, Store: store, Hosts: hosts, Configured: "127.0.0.1:0",
		Listener: ln, Logger: slog.New(slog.DiscardHandler), Ended: ended,
	})
	var published []string
	n.onListen = func(addr string) { published = append(published, addr) }
	n.Serve()
	t.Cleanup(func() {
		n.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		n.Wait()
	})
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	loopback := "127.0.0.1:" + port

	// A kept-alive connection opened before the switch must keep working.
	conn, err := net.Dial("tcp", loopback)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	reader := bufio.NewReader(conn)
	get := func() {
		t.Helper()
		if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: x\r\n\r\n"); err != nil {
			t.Fatal(err)
		}
		res, err := http.ReadResponse(reader, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}
	get()

	status, err := n.SetPublic(true)
	if err != nil || !status.Public || !status.Saved {
		t.Fatalf("public: %+v %v", status, err)
	}
	if len(published) != 1 || !listensettings.Wildcard(published[0]) {
		t.Fatalf("record not updated: %v", published)
	}
	get()
	if res, err := http.Get("http://" + loopback + "/"); err != nil {
		t.Fatalf("new listener: %v", err)
	} else {
		_ = res.Body.Close()
	}
	if saved, web, err := listensettings.Inspect(state, "127.0.0.1:0"); err != nil || !saved.Public || !web {
		t.Fatalf("not persisted: %+v %v %v", saved, web, err)
	}

	status, err = n.SetPublic(false)
	if err != nil || status.Public || status.Listen != loopback {
		t.Fatalf("private: %+v %v", status, err)
	}
	get()
	select {
	case err := <-ended:
		t.Fatalf("rebind ended the server: %v", err)
	default:
	}
}

func TestNetworkControllerRefusesPublicWithoutStorage(t *testing.T) {
	store := listensettings.Open(t.TempDir(), "127.0.0.1:0")
	store.Close() // later writes report the store unavailable
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.NotFoundHandler(), ReadHeaderTimeout: 5 * time.Second}
	ended := make(chan error, 1)
	n := newNetworkController(networkOptions{
		Server: server, Store: store, Hosts: api.NewListen(config.Config{Listen: ln.Addr().String()}),
		Configured: "127.0.0.1:0", Listener: ln, Logger: slog.New(slog.DiscardHandler), Ended: ended,
	})
	n.Serve()
	defer func() {
		n.Close()
		_ = server.Close()
		n.Wait()
	}()
	status, err := n.SetPublic(true)
	if err == nil || status.Public || status.Error == "" {
		t.Fatalf("public without storage: %+v %v", status, err)
	}
}
