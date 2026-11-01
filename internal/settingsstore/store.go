// Package settingsstore holds the fail-closed persistence skeleton shared by
// the web-editable entry settings (tunnel, Tailscale). Each caller supplies
// its own file schema, validation and fail-closed transformation.
package settingsstore

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/rainchan/webmux/internal/pathutil"
	"github.com/rainchan/webmux/internal/statefile"
)

// Document is a stored settings file. UnavailableErr names the caller's
// sentinel wrapped around every storage fault, including on a nil Store.
type Document interface{ UnavailableErr() error }

// Spec describes one settings document. T is the in-memory settings value,
// S the stored JSON document.
type Spec[T any, S Document] struct {
	ID statefile.ID
	// Decode validates a stored document against the TOML defaults.
	Decode func(f S, defaults T) (T, error)
	// Normalize validates a Save request; its error is returned unchanged.
	Normalize func(defaults, next T) (T, error)
	Encode    func(T) S
	// FailClosed disables exposure when storage is unavailable.
	FailClosed func(*T)
	// Closed describes the fault reported after Close.
	Closed string
}

// Store holds an exclusive lock for its lifetime. A fault disables writes
// and applies FailClosed without changing the original file.
type Store[T any, S Document] struct {
	spec              *Spec[T, S]
	mu                sync.Mutex
	store             *statefile.Handle[S]
	defaults, current T
	web               bool
	broken            error
}

// Open loads the override for stateDir. defaults is the validated TOML value.
func Open[T any, S Document](spec *Spec[T, S], stateDir string, defaults T) *Store[T, S] {
	s := &Store[T, S]{spec: spec, defaults: defaults, current: defaults}
	dir, err := pathutil.Expand(stateDir)
	if err == nil {
		dir, err = filepath.Abs(dir)
	}
	if err == nil {
		err = s.load(dir)
	}
	if err != nil {
		s.broken = err
		spec.FailClosed(&s.current)
	}
	return s
}

func (s *Store[T, S]) validate(f S) error {
	_, err := s.spec.Decode(f, s.defaults)
	return err
}

func (s *Store[T, S]) load(dir string) error {
	var f S
	var err error
	s.store, f, err = statefile.Open[S](dir, s.spec.ID, s.validate)
	if statefile.IsKind(err, statefile.Missing) {
		return nil
	}
	if err != nil {
		return err
	}
	s.current, err = s.spec.Decode(f, s.defaults)
	s.web = err == nil
	return err
}

// Inspect reads the stored override without taking the lock.
func Inspect[T any, S Document](spec *Spec[T, S], dir string, defaults T) (T, bool, error) {
	f, err := statefile.ReadSnapshot[S](dir, spec.ID, func(f S) error {
		_, err := spec.Decode(f, defaults)
		return err
	})
	if err != nil {
		return defaults, false, err
	}
	v, err := spec.Decode(f, defaults)
	return v, err == nil, err
}

// Effective returns the current settings and whether they came from the web.
func (s *Store[T, S]) Effective() (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current, s.web
}

func (s *Store[T, S]) unavailableLocked() error {
	if s.broken != nil {
		return fmt.Errorf("%w: %w", unavailable[S](), s.broken)
	}
	return nil
}

// Unavailable reports the storage fault, if any.
func (s *Store[T, S]) Unavailable() error {
	if s == nil {
		return unavailable[S]()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unavailableLocked()
}

func unavailable[S Document]() error {
	var zero S
	return zero.UnavailableErr()
}

// Save publishes before changing in-memory settings. A failed write leaves
// the previous effective settings intact and marks storage unavailable.
func (s *Store[T, S]) Save(next T) error {
	if s == nil {
		return unavailable[S]()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.unavailableLocked(); err != nil {
		return err
	}
	normalized, err := s.spec.Normalize(s.defaults, next)
	if err != nil {
		return err
	}
	if err := s.store.Commit(s.spec.Encode(normalized)); err != nil {
		s.broken = err
		return s.unavailableLocked()
	}
	s.current, s.web = normalized, true
	return nil
}

// Reset removes the override durably and returns the TOML settings.
func (s *Store[T, S]) Reset() (T, error) {
	var zero T
	if s == nil {
		return zero, unavailable[S]()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.unavailableLocked(); err != nil {
		return zero, err
	}
	if err := s.store.Remove(); err != nil {
		s.broken = err
		return zero, s.unavailableLocked()
	}
	s.current, s.web = s.defaults, false
	return s.current, nil
}

// Close releases the lock; later writes report the store unavailable.
func (s *Store[T, S]) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store.Close()
	s.broken = errors.New(s.spec.Closed)
}
