// Package forward keeps the in-memory registry of forwarded loopback ports,
// the forwarding credentials the gateway accepts, and the reachability of each
// forwarded port. It does not serve HTTP: internal/api proxies the traffic.
package forward

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"
)

var (
	// ErrInvalid reports a port outside 1-65535, WebMux's own port, or a
	// malformed label or option.
	ErrInvalid = errors.New("invalid forward")
	// ErrConflict reports a port that is already forwarded.
	ErrConflict = errors.New("port already forwarded")
	// ErrNotFound reports a port that is not forwarded.
	ErrNotFound = errors.New("forward not found")
)

// MaxLabelRunes bounds the display name of a forward.
const MaxLabelRunes = 64

// Prefix selects what the gateway sends upstream for /forward/{port}/....
type Prefix string

const (
	// PrefixKeep sends the path unchanged, for servers whose base is
	// /forward/{port}/.
	PrefixKeep Prefix = "keep"
	// PrefixStrip removes /forward/{port} before sending the path.
	PrefixStrip Prefix = "strip"
)

// Options are the compatibility rewrites of one forward.
type Options struct {
	RewriteHost      bool
	RewriteOrigin    bool
	ForwardedHeaders bool
	RewriteResponse  bool
	Prefix           Prefix
}

// DefaultOptions turns every rewrite on and keeps the prefix, which suits a
// dev server such as Vite started with --base /forward/{port}/.
func DefaultOptions() Options {
	return Options{RewriteHost: true, RewriteOrigin: true, ForwardedHeaders: true, RewriteResponse: true, Prefix: PrefixKeep}
}

// Status is the reachability of the forwarded loopback port.
type Status string

const (
	StatusListening   Status = "listening"
	StatusUnreachable Status = "unreachable"
)

// Forward is one forwarded port.
type Forward struct {
	Port int
	// Label is the optional display name.
	Label string
	// Session is the ID of the session the port was detected in, or "". It
	// is only a reference: the forward outlives the session.
	Session string
	// Process is the executable name reported with a detected port, or "".
	Process   string
	Options   Options
	Status    Status
	CreatedAt time.Time
}

// Path is the gateway path of the forward.
func (f Forward) Path() string { return "/forward/" + strconv.Itoa(f.Port) + "/" }

// Input adds a forward; nil Options selects DefaultOptions.
type Input struct {
	Port    int
	Label   string
	Session string
	Process string
	Options *Options
}

// Update changes a forward; nil fields stay unchanged.
type Update struct {
	Label   *string
	Options *Options
}

// Config parameterizes a Service; zero fields take production defaults.
type Config struct {
	// ExcludePorts are WebMux's own listening ports, which cannot be
	// forwarded.
	ExcludePorts []int
	// Now is the clock of credential expiry.
	Now func() time.Time
	// Reachable reports whether the loopback port accepts connections.
	Reachable func(ctx context.Context, port int) bool
	// ProbeInterval spaces the reachability probes of the background loop.
	ProbeInterval time.Duration
}

const (
	probeInterval = 3 * time.Second
	probeTimeout  = 300 * time.Millisecond
)

type entry struct {
	forward Forward
	// ctx ends when the forward is removed, which closes its proxied
	// connections.
	ctx    context.Context
	cancel context.CancelFunc
}

// Subscription is signalled after any change to the forward list. Signals
// coalesce, so a slow reader only sees the latest List and never blocks the
// service.
type Subscription struct {
	C chan struct{}
}

// Service is the forward registry. It is safe for concurrent use.
type Service struct {
	exclude   []int
	now       func() time.Time
	reachable func(ctx context.Context, port int) bool

	mu      sync.Mutex
	entries map[int]*entry
	subs    map[*Subscription]struct{}

	credentials *Credentials

	stop context.CancelFunc
	done chan struct{}
}

// New starts a Service and its probe loop; Close stops the loop.
func New(c Config) *Service {
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.Reachable == nil {
		c.Reachable = Reachable
	}
	if c.ProbeInterval <= 0 {
		c.ProbeInterval = probeInterval
	}
	ctx, stop := context.WithCancel(context.Background())
	s := &Service{
		exclude:     slices.Clone(c.ExcludePorts),
		now:         c.Now,
		reachable:   c.Reachable,
		entries:     map[int]*entry{},
		subs:        map[*Subscription]struct{}{},
		credentials: newCredentials(c.Now),
		stop:        stop,
		done:        make(chan struct{}),
	}
	go s.loop(ctx, c.ProbeInterval)
	return s
}

// Close stops the probe loop and ends every forward's connections.
func (s *Service) Close() {
	s.stop()
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.entries {
		e.cancel()
	}
}

// Credentials returns the forwarding credential store.
func (s *Service) Credentials() *Credentials { return s.credentials }

func (s *Service) validPort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%w: port must be 1-65535", ErrInvalid)
	}
	if slices.Contains(s.exclude, port) {
		return fmt.Errorf("%w: port %d is WebMux's own port", ErrInvalid, port)
	}
	return nil
}

func validOptions(o Options) error {
	if o.Prefix != PrefixKeep && o.Prefix != PrefixStrip {
		return fmt.Errorf("%w: prefix must be keep or strip", ErrInvalid)
	}
	return nil
}

func validLabel(label string) error {
	if utf8.RuneCountInString(label) > MaxLabelRunes {
		return fmt.Errorf("%w: label exceeds %d characters", ErrInvalid, MaxLabelRunes)
	}
	return nil
}

// Add forwards a port. Its status is probed once before it is listed.
func (s *Service) Add(ctx context.Context, in Input) (Forward, error) {
	if err := s.validPort(in.Port); err != nil {
		return Forward{}, err
	}
	if err := validLabel(in.Label); err != nil {
		return Forward{}, err
	}
	options := DefaultOptions()
	if in.Options != nil {
		options = *in.Options
	}
	if err := validOptions(options); err != nil {
		return Forward{}, err
	}
	s.mu.Lock()
	_, exists := s.entries[in.Port]
	s.mu.Unlock()
	if exists {
		return Forward{}, fmt.Errorf("%w: %d", ErrConflict, in.Port)
	}
	status := StatusUnreachable
	if s.reachable(ctx, in.Port) {
		status = StatusListening
	}
	f := Forward{
		Port: in.Port, Label: in.Label, Session: in.Session, Process: in.Process,
		Options: options, Status: status, CreatedAt: s.now(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Another request may have added the port while this one probed.
	if _, exists := s.entries[in.Port]; exists {
		return Forward{}, fmt.Errorf("%w: %d", ErrConflict, in.Port)
	}
	life, cancel := context.WithCancel(context.Background())
	s.entries[in.Port] = &entry{forward: f, ctx: life, cancel: cancel}
	s.notifyLocked()
	return f, nil
}

// Update changes the label or options of a forward; later requests use them.
func (s *Service) Update(port int, u Update) (Forward, error) {
	if u.Label != nil {
		if err := validLabel(*u.Label); err != nil {
			return Forward{}, err
		}
	}
	if u.Options != nil {
		if err := validOptions(*u.Options); err != nil {
			return Forward{}, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[port]
	if !ok {
		return Forward{}, fmt.Errorf("%w: %d", ErrNotFound, port)
	}
	if u.Label != nil {
		e.forward.Label = *u.Label
	}
	if u.Options != nil {
		e.forward.Options = *u.Options
	}
	s.notifyLocked()
	return e.forward, nil
}

// Remove stops forwarding a port and closes its proxied connections.
func (s *Service) Remove(port int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[port]
	if !ok {
		return fmt.Errorf("%w: %d", ErrNotFound, port)
	}
	delete(s.entries, port)
	e.cancel()
	s.notifyLocked()
	return nil
}

// List returns the forwards in ascending port order.
func (s *Service) List() []Forward {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Forward, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, e.forward)
	}
	slices.SortFunc(out, func(a, b Forward) int { return a.Port - b.Port })
	return out
}

// Lookup returns a forward and the context that ends when it is removed.
func (s *Service) Lookup(port int) (Forward, context.Context, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[port]
	if !ok {
		return Forward{}, nil, false
	}
	return e.forward, e.ctx, true
}

// Subscribe registers for change signals; pair it with Unsubscribe.
func (s *Service) Subscribe() *Subscription {
	sub := &Subscription{C: make(chan struct{}, 1)}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subs[sub] = struct{}{}
	return sub
}

// Unsubscribe stops the signals of sub.
func (s *Service) Unsubscribe(sub *Subscription) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subs, sub)
}

func (s *Service) notifyLocked() {
	for sub := range s.subs {
		select {
		case sub.C <- struct{}{}:
		default:
		}
	}
}

func (s *Service) loop(ctx context.Context, interval time.Duration) {
	defer close(s.done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Probe(ctx)
		}
	}
}

// Probe checks every forwarded port once and signals subscribers when any
// status changed. It holds no lock while dialling.
func (s *Service) Probe(ctx context.Context) {
	s.mu.Lock()
	ports := make([]int, 0, len(s.entries))
	for port := range s.entries {
		ports = append(ports, port)
	}
	s.mu.Unlock()
	statuses := make(map[int]Status, len(ports))
	for _, port := range ports {
		statuses[port] = StatusUnreachable
		if s.reachable(ctx, port) {
			statuses[port] = StatusListening
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for port, status := range statuses {
		// A port removed, or removed and added again, while probing keeps
		// the status its current entry already has.
		if e, ok := s.entries[port]; ok && e.forward.Status != status {
			e.forward.Status = status
			changed = true
		}
	}
	if changed {
		s.notifyLocked()
	}
}

// Reachable dials the loopback port over IPv4, then IPv6.
func Reachable(ctx context.Context, port int) bool {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	conn, err := dialLoopback(ctx, port)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// Dial connects to a forwarded port for the gateway; a port that is not
// forwarded is ErrNotFound and is never dialled.
func (s *Service) Dial(ctx context.Context, port int) (net.Conn, error) {
	if _, _, ok := s.Lookup(port); !ok {
		return nil, fmt.Errorf("%w: %d", ErrNotFound, port)
	}
	return dialLoopback(ctx, port)
}

// dialLoopback connects to the port on 127.0.0.1, falling back to [::1] for
// servers that bind only the IPv6 loopback. It never dials another host.
func dialLoopback(ctx context.Context, port int) (net.Conn, error) {
	var d net.Dialer
	p := strconv.Itoa(port)
	conn, err := d.DialContext(ctx, "tcp4", net.JoinHostPort("127.0.0.1", p))
	if err == nil {
		return conn, nil
	}
	if conn, err6 := d.DialContext(ctx, "tcp6", net.JoinHostPort("::1", p)); err6 == nil {
		return conn, nil
	}
	return nil, err
}
