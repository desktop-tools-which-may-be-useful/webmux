// Package voice runs the external STDIN/STDOUT speech recognition command
// configured in [voice].
package voice

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ErrDisabled reports that no external voice command is configured.
var ErrDisabled = errors.New("voice recognition is not configured")

const (
	// DefaultReadyDelay is the time after process start before emitting an
	// implicit "ready" event if the subprocess has not printed one itself.
	DefaultReadyDelay = 150 * time.Millisecond
	// DefaultFinishTimeout bounds how long StopInput waits for the subprocess
	// to flush its final transcription after STDIN is closed.
	DefaultFinishTimeout = 15 * time.Second
)

// Options configures the external speech recognition service.
type Options struct {
	Cmd           []string
	Env           []string
	Cwd           string
	ReadyDelay    time.Duration
	FinishTimeout time.Duration
}

// EventType identifies a voice recognition event emitted by a Session.
type EventType string

const (
	EventReady   EventType = "ready"
	EventPartial EventType = "partial"
	EventFinal   EventType = "final"
	EventError   EventType = "error"
)

// Event is one recognition update from the subprocess.
type Event struct {
	Type  EventType
	Text  string
	Error string
}

// Service spawns speech recognition subprocesses per recording session.
type Service struct {
	mu            sync.RWMutex
	cmd           []string
	env           []string
	cwd           string
	readyDelay    time.Duration
	finishTimeout time.Duration
}

// New constructs a Service from Options.
func New(opts Options) *Service {
	readyDelay := opts.ReadyDelay
	if readyDelay <= 0 {
		readyDelay = DefaultReadyDelay
	}
	finishTimeout := opts.FinishTimeout
	if finishTimeout <= 0 {
		finishTimeout = DefaultFinishTimeout
	}
	return &Service{
		cmd:           normalizeCmd(opts.Cmd),
		env:           append([]string(nil), opts.Env...),
		cwd:           opts.Cwd,
		readyDelay:    readyDelay,
		finishTimeout: finishTimeout,
	}
}

// Enabled reports whether a voice recognition command is configured.
func (s *Service) Enabled() bool {
	if s == nil {
		return false
	}
	argv, _, _ := s.snapshot()
	return len(argv) > 0
}

// snapshot returns the command, environment and directory a new recording
// uses; a running recording keeps the ones it started with.
func (s *Service) snapshot() (argv, env []string, cwd string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cmd, s.env, s.cwd
}

// Reconfigure replaces the command, environment and directory that later
// recordings use. Recordings in progress are not affected.
func (s *Service) Reconfigure(opts Options) {
	cmd := normalizeCmd(opts.Cmd)
	env := append([]string(nil), opts.Env...)
	s.mu.Lock()
	s.cmd, s.env, s.cwd = cmd, env, opts.Cwd
	s.mu.Unlock()
}

func normalizeCmd(cmd []string) []string {
	if len(cmd) > 0 && strings.TrimSpace(cmd[0]) != "" {
		return append([]string(nil), cmd...)
	}
	return nil
}

// Session is one active recording subprocess.
type Session struct {
	cancel    context.CancelFunc
	stdinMu   sync.Mutex
	stdin     io.WriteCloser
	events    chan Event
	done      chan struct{}
	closeOnce sync.Once
	stopTimer *time.Timer
}

// Events returns the channel of recognition events. It is closed when the
// subprocess exits and all output has been processed.
func (s *Session) Events() <-chan Event {
	return s.events
}

// WriteAudio writes a chunk of 16kHz mono s16le PCM to the subprocess STDIN.
func (s *Session) WriteAudio(pcm []byte) error {
	if len(pcm) == 0 {
		return nil
	}
	s.stdinMu.Lock()
	defer s.stdinMu.Unlock()
	if s.stdin == nil {
		return io.ErrClosedPipe
	}
	_, err := s.stdin.Write(pcm)
	return err
}

// StopInput closes the subprocess STDIN (sending EOF) and starts the finish
// timeout timer so a hung subprocess cannot block the caller indefinitely.
func (s *Session) StopInput(timeout time.Duration) {
	s.stdinMu.Lock()
	if s.stdin != nil {
		_ = s.stdin.Close()
		s.stdin = nil
	}
	if s.stopTimer == nil && timeout > 0 {
		s.stopTimer = time.AfterFunc(timeout, s.cancel)
	}
	s.stdinMu.Unlock()
}

// Close terminates the subprocess and waits for its reader goroutines to exit.
func (s *Session) Close() {
	s.closeOnce.Do(func() {
		s.stdinMu.Lock()
		if s.stopTimer != nil {
			s.stopTimer.Stop()
		}
		if s.stdin != nil {
			_ = s.stdin.Close()
			s.stdin = nil
		}
		s.stdinMu.Unlock()
		s.cancel()
		<-s.done
	})
}

type stdoutMessage struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Error string `json:"error"`
}

// Start launches a new recognition subprocess bound to ctx.
func (s *Service) Start(ctx context.Context) (*Session, error) {
	argv, env, cwd := s.snapshot()
	if len(argv) == 0 {
		return nil, ErrDisabled
	}
	runCtx, cancel := context.WithCancel(ctx)
	// #nosec G204 -- argv is the administrator-configured command, never shell-expanded.
	cmd := exec.CommandContext(runCtx, argv[0], argv[1:]...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		cancel()
		return nil, err
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &limitedBuffer{buf: &stderrBuf, limit: 4096}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		cancel()
		return nil, err
	}

	sess := &Session{
		cancel: cancel,
		stdin:  stdin,
		events: make(chan Event, 32),
		done:   make(chan struct{}),
	}

	go s.pump(runCtx, sess, cmd, stdout, &stderrBuf)
	return sess, nil
}

func (s *Service) FinishTimeout() time.Duration {
	if s == nil || s.finishTimeout <= 0 {
		return DefaultFinishTimeout
	}
	return s.finishTimeout
}

func (s *Service) pump(ctx context.Context, sess *Session, cmd *exec.Cmd, stdout io.Reader, stderrBuf *bytes.Buffer) {
	defer close(sess.done)
	defer close(sess.events)

	var readyOnce sync.Once
	sendEvent := func(ev Event) {
		select {
		case sess.events <- ev:
		case <-ctx.Done():
		}
	}
	emitReady := func() {
		readyOnce.Do(func() {
			sendEvent(Event{Type: EventReady})
		})
	}

	readyTimer := time.AfterFunc(s.readyDelay, func() {
		if ctx.Err() == nil {
			emitReady()
		}
	})
	defer readyTimer.Stop()

	var (
		plainLines []string
		finals     []string
		sawFinal   bool
		sawError   bool
	)

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "{") {
			var msg stdoutMessage
			if json.Unmarshal([]byte(line), &msg) == nil && msg.Type != "" {
				switch EventType(msg.Type) {
				case EventReady:
					readyTimer.Stop()
					emitReady()
				case EventPartial:
					readyTimer.Stop()
					emitReady()
					sendEvent(Event{Type: EventPartial, Text: msg.Text})
				case EventFinal:
					readyTimer.Stop()
					emitReady()
					sawFinal = true
					if msg.Text != "" {
						finals = append(finals, msg.Text)
					}
				case EventError:
					readyTimer.Stop()
					sawError = true
					errMsg := strings.TrimSpace(msg.Error)
					if errMsg == "" {
						errMsg = "voice recognition error"
					}
					sendEvent(Event{Type: EventError, Error: errMsg})
				}
				continue
			}
		}
		plainLines = append(plainLines, line)
	}

	waitErr := cmd.Wait()
	readyTimer.Stop()
	if ctx.Err() != nil && !sawFinal && !sawError {
		return
	}

	if sawError {
		return
	}
	if sawFinal {
		sendEvent(Event{Type: EventFinal, Text: strings.Join(finals, "")})
		return
	}
	if waitErr == nil {
		sendEvent(Event{Type: EventFinal, Text: strings.Join(plainLines, "\n")})
		return
	}
	errMsg := lastNonEmptyLine(stderrBuf.String())
	if errMsg == "" {
		errMsg = waitErr.Error()
	}
	sendEvent(Event{Type: EventError, Error: errMsg})
}

func lastNonEmptyLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			if len(line) > 240 {
				return line[:240]
			}
			return line
		}
	}
	return ""
}

type limitedBuffer struct {
	mu    sync.Mutex
	buf   *bytes.Buffer
	limit int
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	rem := l.limit - l.buf.Len()
	if rem > 0 {
		if len(p) > rem {
			l.buf.Write(p[:rem])
		} else {
			l.buf.Write(p)
		}
	}
	return len(p), nil
}
