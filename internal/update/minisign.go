package update

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// Signatures use minisign's legacy "Ed" algorithm: Ed25519 over the file
// itself, plus a global signature over (signature || trusted comment). The
// prehashed "ED" variant needs BLAKE2b, which the standard library lacks;
// checksum files are small, so prehashing buys nothing.
var algEd = [2]byte{'E', 'd'}

const (
	trustedPrefix   = "trusted comment: "
	untrustedPrefix = "untrusted comment: "
)

// Errors returned by Verify.
var (
	ErrBadSignature = errors.New("签名无效")
	ErrUnknownKey   = errors.New("未知签名密钥")
)

// PublicKey is one minisign Ed25519 public key.
type PublicKey struct {
	ID  [8]byte
	Key ed25519.PublicKey
}

// SecretKey is a signing key: its minisign key ID and Ed25519 seed. Its
// text form is base64(ID || seed), the value of the release CI secret.
type SecretKey struct {
	ID   [8]byte
	Seed []byte
}

// IDString formats a key ID as minisign prints it.
func IDString(id [8]byte) string {
	return fmt.Sprintf("%016X", binary.LittleEndian.Uint64(id[:]))
}

// ParsePublicKey accepts the base64 line of a minisign public key, or the
// whole two-line file.
func ParsePublicKey(text string) (PublicKey, error) {
	line := strings.TrimSpace(text)
	if i := strings.LastIndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[i+1:])
	}
	raw, err := base64.StdEncoding.DecodeString(line)
	if err != nil || len(raw) != 2+8+ed25519.PublicKeySize || !bytes.Equal(raw[:2], algEd[:]) {
		return PublicKey{}, errors.New("not a minisign Ed25519 public key")
	}
	var k PublicKey
	copy(k.ID[:], raw[2:10])
	k.Key = ed25519.PublicKey(bytes.Clone(raw[10:]))
	return k, nil
}

// String returns the public key's base64 line.
func (k PublicKey) String() string {
	raw := append(append(algEd[:], k.ID[:]...), k.Key...)
	return base64.StdEncoding.EncodeToString(raw)
}

// File returns the public key as a minisign .pub file.
func (k PublicKey) File() string {
	return untrustedPrefix + "minisign public key " + IDString(k.ID) + "\n" + k.String() + "\n"
}

// GenerateKey creates a new signing key.
func GenerateKey() (SecretKey, error) {
	var s SecretKey
	if _, err := rand.Read(s.ID[:]); err != nil {
		return s, err
	}
	s.Seed = make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(s.Seed); err != nil {
		return s, err
	}
	return s, nil
}

// ParseSecretKey decodes base64(ID || seed).
func ParseSecretKey(text string) (SecretKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
	if err != nil || len(raw) != 8+ed25519.SeedSize {
		return SecretKey{}, errors.New("signing key must be base64 of an 8-byte key ID and a 32-byte Ed25519 seed")
	}
	var s SecretKey
	copy(s.ID[:], raw[:8])
	s.Seed = bytes.Clone(raw[8:])
	return s, nil
}

// String encodes the secret key for the CI secret.
func (s SecretKey) String() string {
	return base64.StdEncoding.EncodeToString(append(s.ID[:], s.Seed...))
}

// Public derives the public key.
func (s SecretKey) Public() PublicKey {
	priv := ed25519.NewKeyFromSeed(s.Seed)
	return PublicKey{ID: s.ID, Key: priv.Public().(ed25519.PublicKey)}
}

// Sign returns a minisign signature file for message.
func (s SecretKey) Sign(message []byte, trusted string) ([]byte, error) {
	if strings.ContainsAny(trusted, "\r\n") {
		return nil, errors.New("trusted comment must be one line")
	}
	priv := ed25519.NewKeyFromSeed(s.Seed)
	sig := ed25519.Sign(priv, message)
	global := ed25519.Sign(priv, append(bytes.Clone(sig), trusted...))
	var b bytes.Buffer
	b.WriteString(untrustedPrefix + "signature from webmux release key " + IDString(s.ID) + "\n")
	b.WriteString(base64.StdEncoding.EncodeToString(append(append(algEd[:], s.ID[:]...), sig...)) + "\n")
	b.WriteString(trustedPrefix + trusted + "\n")
	b.WriteString(base64.StdEncoding.EncodeToString(global) + "\n")
	return b.Bytes(), nil
}

// Verify checks a minisign signature over message with one of keys and
// returns the authenticated trusted comment.
func Verify(message, signature []byte, keys []PublicKey) (string, error) {
	lines := strings.Split(strings.TrimRight(string(signature), "\r\n"), "\n")
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	if len(lines) != 4 || !strings.HasPrefix(lines[0], untrustedPrefix) || !strings.HasPrefix(lines[2], trustedPrefix) {
		return "", ErrBadSignature
	}
	raw, err := base64.StdEncoding.DecodeString(lines[1])
	if err != nil || len(raw) != 2+8+ed25519.SignatureSize || !bytes.Equal(raw[:2], algEd[:]) {
		return "", ErrBadSignature
	}
	var id [8]byte
	copy(id[:], raw[2:10])
	sig := raw[10:]
	var key ed25519.PublicKey
	for _, k := range keys {
		if k.ID == id {
			key = k.Key
			break
		}
	}
	if key == nil {
		return "", fmt.Errorf("%w %s", ErrUnknownKey, IDString(id))
	}
	global, err := base64.StdEncoding.DecodeString(lines[3])
	if err != nil || len(global) != ed25519.SignatureSize {
		return "", ErrBadSignature
	}
	trusted := strings.TrimPrefix(lines[2], trustedPrefix)
	if !ed25519.Verify(key, message, sig) || !ed25519.Verify(key, append(bytes.Clone(sig), trusted...), global) {
		return "", ErrBadSignature
	}
	return trusted, nil
}
