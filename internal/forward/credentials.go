package forward

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

// ErrUnauthorized reports an unknown, used or expired ticket.
var ErrUnauthorized = errors.New("forwarding ticket is invalid or expired")

const (
	// TicketTTL bounds how long a ticket can be redeemed.
	TicketTTL = 60 * time.Second
	// CredentialTTL is the lifetime of a forwarding credential and of the
	// cookie that carries it.
	CredentialTTL = 12 * time.Hour
	secretBytes   = 32
)

// Ticket is a one-time code that a browser holding the main token exchanges
// for a forwarding credential.
type Ticket struct {
	Code      string
	ExpiresAt time.Time
}

// Credential authorizes the forwarding gateway only. It is random and
// independent of the main token.
type Credential struct {
	Value     string
	ExpiresAt time.Time
}

type secret struct {
	hash    [sha256.Size]byte
	expires time.Time
}

// Credentials stores tickets and credentials in memory as SHA-256 hashes, so
// a restart invalidates all of them.
type Credentials struct {
	now func() time.Time

	mu          sync.Mutex
	tickets     []secret
	credentials []secret
}

func newCredentials(now func() time.Time) *Credentials {
	return &Credentials{now: now}
}

func randomSecret() (string, [sha256.Size]byte, error) {
	b := make([]byte, secretBytes)
	if _, err := rand.Read(b); err != nil {
		return "", [sha256.Size]byte{}, err
	}
	value := base64.RawURLEncoding.EncodeToString(b)
	return value, sha256.Sum256([]byte(value)), nil
}

// prune drops expired secrets.
func prune(list []secret, now time.Time) []secret {
	kept := list[:0]
	for _, s := range list {
		if now.Before(s.expires) {
			kept = append(kept, s)
		}
	}
	clear(list[len(kept):])
	return kept
}

// find returns the index of the live secret matching value, comparing every
// stored hash in constant time.
func find(list []secret, value string) int {
	got := sha256.Sum256([]byte(value))
	match := -1
	for i := range list {
		if subtle.ConstantTimeCompare(list[i].hash[:], got[:]) == 1 {
			match = i
		}
	}
	return match
}

// IssueTicket creates a ticket valid for TicketTTL.
func (c *Credentials) IssueTicket() (Ticket, error) {
	code, hash, err := randomSecret()
	if err != nil {
		return Ticket{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	c.tickets = append(prune(c.tickets, now), secret{hash: hash, expires: now.Add(TicketTTL)})
	return Ticket{Code: code, ExpiresAt: now.Add(TicketTTL)}, nil
}

// Redeem consumes a ticket and issues a credential. Concurrent redemptions of
// one ticket succeed at most once.
func (c *Credentials) Redeem(code string) (Credential, error) {
	value, hash, err := randomSecret()
	if err != nil {
		return Credential{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	c.tickets = prune(c.tickets, now)
	i := find(c.tickets, code)
	if i < 0 {
		return Credential{}, ErrUnauthorized
	}
	c.tickets = append(c.tickets[:i], c.tickets[i+1:]...)
	expires := now.Add(CredentialTTL)
	c.credentials = append(prune(c.credentials, now), secret{hash: hash, expires: expires})
	return Credential{Value: value, ExpiresAt: expires}, nil
}

// Valid reports whether value is a live credential.
func (c *Credentials) Valid(value string) bool {
	if value == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.credentials = prune(c.credentials, c.now())
	return find(c.credentials, value) >= 0
}
