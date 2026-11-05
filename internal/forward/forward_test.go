package forward

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newService(t *testing.T, c Config) *Service {
	t.Helper()
	if c.Reachable == nil {
		c.Reachable = func(context.Context, int) bool { return false }
	}
	// The loop never ticks during a test; tests call Probe.
	c.ProbeInterval = time.Hour
	s := New(c)
	t.Cleanup(s.Close)
	return s
}

func TestAddValidatesPorts(t *testing.T) {
	s := newService(t, Config{ExcludePorts: []int{7681}})
	for _, port := range []int{0, -1, 65536, 7681} {
		if _, err := s.Add(t.Context(), Input{Port: port}); !errors.Is(err, ErrInvalid) {
			t.Errorf("Add(%d) = %v, want ErrInvalid", port, err)
		}
	}
	long := string(make([]rune, MaxLabelRunes+1))
	if _, err := s.Add(t.Context(), Input{Port: 5173, Label: long}); !errors.Is(err, ErrInvalid) {
		t.Errorf("long label = %v, want ErrInvalid", err)
	}
	if _, err := s.Add(t.Context(), Input{Port: 5173, Options: &Options{Prefix: "other"}}); !errors.Is(err, ErrInvalid) {
		t.Errorf("bad prefix = %v, want ErrInvalid", err)
	}
	for _, port := range []int{1, 65535} {
		if _, err := s.Add(t.Context(), Input{Port: port}); err != nil {
			t.Errorf("Add(%d) = %v", port, err)
		}
	}
}

func TestRegistryLifecycle(t *testing.T) {
	s := newService(t, Config{Reachable: func(_ context.Context, port int) bool { return port == 5173 }})
	sub := s.Subscribe()
	defer s.Unsubscribe(sub)

	f, err := s.Add(t.Context(), Input{Port: 5173, Session: "web", Process: "node"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Options != DefaultOptions() || f.Status != StatusListening || f.Path() != "/forward/5173/" {
		t.Fatalf("added %+v", f)
	}
	select {
	case <-sub.C:
	default:
		t.Fatal("add did not signal")
	}
	if _, err := s.Add(t.Context(), Input{Port: 5173, Options: &Options{Prefix: PrefixStrip}}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate add = %v", err)
	}
	if got, _, _ := s.Lookup(5173); got.Options.Prefix != PrefixKeep {
		t.Fatalf("duplicate add changed options: %+v", got.Options)
	}

	label := "web"
	strip := DefaultOptions()
	strip.Prefix = PrefixStrip
	if f, err = s.Update(5173, Update{Label: &label, Options: &strip}); err != nil || f.Label != "web" || f.Options.Prefix != PrefixStrip {
		t.Fatalf("update = %+v, %v", f, err)
	}
	if _, err := s.Update(8000, Update{Label: &label}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update missing = %v", err)
	}

	if _, err := s.Add(t.Context(), Input{Port: 3000}); err != nil {
		t.Fatal(err)
	}
	list := s.List()
	if len(list) != 2 || list[0].Port != 3000 || list[1].Port != 5173 || list[0].Status != StatusUnreachable {
		t.Fatalf("list = %+v", list)
	}

	_, life, _ := s.Lookup(5173)
	if err := s.Remove(5173); err != nil {
		t.Fatal(err)
	}
	if life.Err() == nil {
		t.Fatal("remove did not end the forward's context")
	}
	if _, _, ok := s.Lookup(5173); ok {
		t.Fatal("removed forward still found")
	}
	if err := s.Remove(5173); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second remove = %v", err)
	}
}

func TestProbeSignalsOnlyChanges(t *testing.T) {
	var up atomic.Bool
	s := newService(t, Config{Reachable: func(context.Context, int) bool { return up.Load() }})
	if _, err := s.Add(t.Context(), Input{Port: 5173}); err != nil {
		t.Fatal(err)
	}
	sub := s.Subscribe()
	defer s.Unsubscribe(sub)
	signalled := func() bool {
		select {
		case <-sub.C:
			return true
		default:
			return false
		}
	}

	s.Probe(t.Context())
	if signalled() {
		t.Fatal("unchanged status signalled")
	}
	up.Store(true)
	s.Probe(t.Context())
	if !signalled() || s.List()[0].Status != StatusListening {
		t.Fatalf("listening not published: %+v", s.List())
	}
	up.Store(false)
	s.Probe(t.Context())
	if !signalled() || s.List()[0].Status != StatusUnreachable {
		t.Fatalf("unreachable not published: %+v", s.List())
	}
}

func TestReachableFollowsRealListener(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	s := newService(t, Config{Reachable: Reachable})
	if _, err := s.Add(t.Context(), Input{Port: port}); err != nil {
		t.Fatal(err)
	}
	if got := s.List()[0].Status; got != StatusListening {
		t.Fatalf("status with listener = %s", got)
	}
	_ = ln.Close()
	s.Probe(t.Context())
	if got := s.List()[0].Status; got != StatusUnreachable {
		t.Fatalf("status after close = %s", got)
	}
}

func TestReachableFallsBackToIPv6(t *testing.T) {
	ln, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skip("no IPv6 loopback:", err)
	}
	defer func() { _ = ln.Close() }()
	if !Reachable(t.Context(), ln.Addr().(*net.TCPAddr).Port) {
		t.Fatal("IPv6-only listener not reachable")
	}
}

func TestTicketsAndCredentials(t *testing.T) {
	c := &clock{now: time.Unix(1_000_000, 0)}
	s := newService(t, Config{Now: c.Now})
	creds := s.Credentials()

	ticket, err := creds.IssueTicket()
	if err != nil {
		t.Fatal(err)
	}
	if len(ticket.Code) < 43 || !ticket.ExpiresAt.Equal(c.Now().Add(TicketTTL)) {
		t.Fatalf("ticket %+v", ticket)
	}
	cred, err := creds.Redeem(ticket.Code)
	if err != nil {
		t.Fatal(err)
	}
	if len(cred.Value) < 43 || cred.Value == ticket.Code || !creds.Valid(cred.Value) {
		t.Fatalf("credential %+v", cred)
	}
	if _, err := creds.Redeem(ticket.Code); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("reused ticket = %v", err)
	}
	if creds.Valid("") || creds.Valid(ticket.Code) || creds.Valid(cred.Value+"x") {
		t.Fatal("invalid value accepted")
	}

	late, _ := creds.IssueTicket()
	c.Advance(TicketTTL)
	if _, err := creds.Redeem(late.Code); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expired ticket = %v", err)
	}
	c.Advance(CredentialTTL - TicketTTL - time.Second)
	if !creds.Valid(cred.Value) {
		t.Fatal("credential expired early")
	}
	c.Advance(time.Second)
	if creds.Valid(cred.Value) {
		t.Fatal("credential outlived its TTL")
	}
	creds.mu.Lock()
	left := len(creds.tickets) + len(creds.credentials)
	creds.mu.Unlock()
	if left != 0 {
		t.Fatalf("%d expired entries kept", left)
	}
}

func TestConcurrentRedeemSucceedsOnce(t *testing.T) {
	s := newService(t, Config{})
	ticket, err := s.Credentials().IssueTicket()
	if err != nil {
		t.Fatal(err)
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			if _, err := s.Credentials().Redeem(ticket.Code); err == nil {
				wins.Add(1)
			}
		})
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("%d redemptions succeeded", wins.Load())
	}
}
