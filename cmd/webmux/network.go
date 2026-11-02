package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/rainchan/webmux/internal/api"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/listensettings"
)

// startupListen resolves the address to bind at startup. An explicit
// --listen wins for this process; otherwise the saved network exposure
// switch, which defaults to the configured address, decides.
func startupListen(configured, source string, store *listensettings.Service) string {
	if source == "cli" {
		return configured
	}
	saved, _ := store.Effective()
	_, port, err := net.SplitHostPort(configured)
	if err != nil {
		return configured
	}
	listen, err := listensettings.Address(configured, port, saved.Public)
	if err != nil {
		return configured
	}
	return listen
}

// networkController owns the TCP listener so the network exposure switch can
// rebind it at runtime. Accepted connections, and therefore WebSockets and
// persistent sessions, survive a rebind: only the listening socket changes.
type networkController struct {
	server     *http.Server
	store      *listensettings.Service
	hosts      *api.Listen
	configured string // TOML or --listen address
	port       string // bound port, fixed for the process
	logger     *slog.Logger
	// onListen publishes the new bound address (runtime record).
	onListen func(addr string)
	ended    chan<- error

	mu      sync.Mutex
	ln      net.Listener // nil only after a failed rebind and revert
	closing bool
	serving sync.WaitGroup
}

type networkOptions struct {
	Server     *http.Server
	Store      *listensettings.Service
	Hosts      *api.Listen
	Configured string
	Listener   net.Listener
	Logger     *slog.Logger
	Ended      chan<- error
}

func newNetworkController(o networkOptions) *networkController {
	_, port, _ := net.SplitHostPort(o.Listener.Addr().String())
	return &networkController{
		server: o.Server, store: o.Store, hosts: o.Hosts, configured: o.Configured,
		port: port, logger: o.Logger, ended: o.Ended, ln: o.Listener,
		onListen: func(string) {},
	}
}

// Serve starts accepting on the initial listener.
func (n *networkController) Serve() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.serveLocked(n.ln)
}

// serveLocked serves ln until it is closed. Only the current listener ending
// on its own (or through server shutdown) ends the process; a listener that
// was replaced by a rebind ends silently.
func (n *networkController) serveLocked(ln net.Listener) {
	n.serving.Add(1)
	go func() {
		defer n.serving.Done()
		err := n.server.Serve(ln)
		n.mu.Lock()
		current := n.ln == ln
		n.mu.Unlock()
		if current {
			n.ended <- err
		}
	}()
}

// Close marks shutdown; http.Server.Shutdown closes the listener.
func (n *networkController) Close() {
	n.mu.Lock()
	n.closing = true
	n.mu.Unlock()
}

// Wait joins the accept loops after http.Server.Shutdown.
func (n *networkController) Wait() { n.serving.Wait() }

// Status describes the current listener and the saved switch.
func (n *networkController) Status() contract.NetworkStatus {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.statusLocked()
}

func (n *networkController) statusLocked() contract.NetworkStatus {
	s := contract.NetworkStatus{}
	if n.ln != nil {
		s.Listen = n.ln.Addr().String()
		s.Public = listensettings.Wildcard(s.Listen)
	}
	_, s.Saved = n.store.Effective()
	if err := n.store.Unavailable(); err != nil {
		s.Error = err.Error()
	}
	return s
}

// SetPublic rebinds the listener on every interface (true) or on the
// configured address (false) and persists the choice. Turning exposure on
// requires the choice to persist; turning it off takes effect even when
// storage fails, and reports that it was not saved.
func (n *networkController) SetPublic(public bool) (contract.NetworkStatus, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.closing {
		return n.statusLocked(), errors.New("WebMux is shutting down")
	}
	if public {
		if err := n.store.Unavailable(); err != nil {
			return n.statusLocked(), err
		}
	}
	target, err := listensettings.Address(n.configured, n.port, public)
	if err != nil {
		return n.statusLocked(), err
	}
	previous := ""
	if n.ln != nil {
		previous = n.ln.Addr().String()
	}
	if err := n.rebindLocked(target, previous); err != nil {
		return n.statusLocked(), err
	}
	if err := n.store.Save(listensettings.Settings{Public: public}); err != nil {
		if public {
			if revert, _ := listensettings.Address(n.configured, n.port, false); revert != "" {
				if rerr := n.rebindLocked(revert, ""); rerr != nil {
					n.logger.Error("network exposure revert failed", "error", rerr)
				}
			}
		}
		return n.statusLocked(), err
	}
	n.logger.Info("network exposure changed", "public", public, "address", n.ln.Addr())
	return n.statusLocked(), nil
}

// rebindLocked replaces the listener with one bound to target. Linux refuses
// to bind a wildcard while the same port is bound on loopback, so the old
// socket closes first; on failure the previous address (when given) is
// restored, and otherwise the server keeps only its local socket until the
// next successful switch.
func (n *networkController) rebindLocked(target, previous string) error {
	if n.ln != nil && sameAddress(n.ln.Addr().String(), target) {
		return nil
	}
	old := n.ln
	n.ln = nil
	if old != nil {
		_ = old.Close()
	}
	ln, err := net.Listen("tcp", target)
	if err != nil {
		bindErr := fmt.Errorf("listen on %s: %w", target, err)
		if previous == "" {
			return bindErr
		}
		restored, rerr := net.Listen("tcp", previous)
		if rerr != nil {
			n.logger.Error("TCP listener lost; only the local socket remains", "error", rerr)
			return errors.Join(bindErr, fmt.Errorf("restore %s: %w", previous, rerr))
		}
		ln = restored
		err = bindErr
	}
	n.ln = ln
	n.hosts.Set(ln.Addr().String())
	n.onListen(ln.Addr().String())
	n.serveLocked(ln)
	return err
}

func sameAddress(a, b string) bool {
	ah, ap, aerr := net.SplitHostPort(a)
	bh, bp, berr := net.SplitHostPort(b)
	if aerr != nil || berr != nil || ap != bp {
		return false
	}
	aip, bip := net.ParseIP(ah), net.ParseIP(bh)
	if aip != nil && bip != nil {
		// Go reports a dual-stack wildcard as [::] even when asked for 0.0.0.0.
		return aip.Equal(bip) || aip.IsUnspecified() && bip.IsUnspecified()
	}
	return ah == bh
}
