// Package auth owns the local web password, session cookie and first-run
// password lifecycle.
package auth

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	CookieName    = "f1tele_session"
	SessionTTL    = 30 * 24 * time.Hour
	MinPassword   = 8
	MaxPassword   = 256
	iterations    = 600_000
	keyLength     = 32
	passwordChars = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
)

var ErrEnvOwned = errors.New("F1TELE_AUTH_PASSWORD owns the password; remove it before resetting")

type fileData struct {
	Version    int    `json:"version"`
	Salt       string `json:"salt"`
	Hash       string `json:"hash"`
	Iterations int    `json:"iterations"`
	SessionKey string `json:"session_key"`
	EnvHash    string `json:"env_hash,omitempty"`
}

type Store struct {
	dataDir string
	path    string
	env     string
	mu      sync.Mutex
}

type EnsureResult struct {
	Created  bool   `json:"created"`
	Source   string `json:"source"`
	Password string `json:"password,omitempty"`
}

func New(dataDir string) *Store {
	return &Store{dataDir: dataDir, path: filepath.Join(dataDir, "auth.json"), env: os.Getenv("F1TELE_AUTH_PASSWORD")}
}

func (s *Store) Path() string { return s.path }

// Ensure creates the auth file when missing. The plaintext password is
// returned only on the call that created it.
func (s *Store) Ensure() (EnsureResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.env != "" {
		fingerprint := hashString(s.env)
		if cur, err := s.loadLocked(); err == nil && cur.EnvHash == fingerprint && s.verifyWith(cur, s.env) {
			return EnsureResult{Source: "env"}, nil
		}
		if err := s.writeLocked(s.env, fingerprint); err != nil {
			return EnsureResult{}, err
		}
		return EnsureResult{Source: "env"}, nil
	}
	if _, err := s.loadLocked(); err == nil {
		return EnsureResult{Source: "stored"}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return EnsureResult{}, err
	}
	password, err := GeneratePassword()
	if err != nil {
		return EnsureResult{}, err
	}
	if err := s.writeLocked(password, ""); err != nil {
		return EnsureResult{}, err
	}
	return EnsureResult{Created: true, Source: "generated", Password: password}, nil
}

func (s *Store) Verify(password string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.env != "" {
		a := sha256.Sum256([]byte(s.env))
		b := sha256.Sum256([]byte(password))
		return subtle.ConstantTimeCompare(a[:], b[:]) == 1
	}
	f, err := s.loadLocked()
	if err != nil {
		return false
	}
	return s.verifyWith(f, password)
}

// Reset replaces the password and rotates the signing key, invalidating every
// session. The previous password is not required.
func (s *Store) Reset(password string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.env != "" {
		return ErrEnvOwned
	}
	return s.writeLocked(password, "")
}

func (s *Store) IssueSession(now time.Time) (string, time.Time, error) {
	s.mu.Lock()
	key, err := s.sessionKeyLocked()
	s.mu.Unlock()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := now.Add(SessionTTL)
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", time.Time{}, err
	}
	payload := map[string]any{"iat": now.Unix(), "exp": expires.Unix(), "nonce": base64.RawURLEncoding.EncodeToString(nonce)}
	raw, _ := json.Marshal(payload)
	body := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), expires, nil
}

func (s *Store) ValidateSession(token string, now time.Time) bool {
	token = strings.TrimSpace(token)
	parts := strings.Split(token, ".")
	if len(parts) != 2 || len(token) > 2048 {
		return false
	}
	s.mu.Lock()
	key, err := s.sessionKeyLocked()
	s.mu.Unlock()
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(parts[0]))
	want := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(want, got) {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	var payload struct {
		IAT   int64  `json:"iat"`
		EXP   int64  `json:"exp"`
		Nonce string `json:"nonce"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Nonce == "" {
		return false
	}
	return payload.EXP > now.Unix() && payload.IAT <= now.Add(time.Minute).Unix()
}

func (s *Store) Cookie(token string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name: CookieName, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode,
		Expires: expires, MaxAge: int(SessionTTL.Seconds()),
	}
}

func ClearCookie() *http.Cookie {
	return &http.Cookie{Name: CookieName, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0)}
}

func (s *Store) sessionKeyLocked() ([]byte, error) {
	f, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	key, err := hex.DecodeString(f.SessionKey)
	if err != nil || len(key) != 32 {
		return nil, errors.New("auth session key is invalid")
	}
	return key, nil
}

func (s *Store) loadLocked() (fileData, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return fileData{}, err
	}
	var f fileData
	if err := json.Unmarshal(raw, &f); err != nil {
		return fileData{}, fmt.Errorf("parse auth file: %w", err)
	}
	if f.Version != 1 || f.Iterations < 100_000 || len(f.Salt) == 0 || len(f.Hash) == 0 || len(f.SessionKey) == 0 {
		return fileData{}, errors.New("auth file is incomplete")
	}
	return f, nil
}

func (s *Store) verifyWith(f fileData, password string) bool {
	salt, err := hex.DecodeString(f.Salt)
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(f.Hash)
	if err != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, f.Iterations, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

func (s *Store) writeLocked(password, envHash string) error {
	salt := make([]byte, 16)
	key := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	if _, err := rand.Read(key); err != nil {
		return err
	}
	derived, err := pbkdf2.Key(sha256.New, password, salt, iterations, keyLength)
	if err != nil {
		return err
	}
	f := fileData{Version: 1, Salt: hex.EncodeToString(salt), Hash: hex.EncodeToString(derived), Iterations: iterations, SessionKey: hex.EncodeToString(key), EnvHash: envHash}
	raw, _ := json.MarshalIndent(f, "", "  ")
	if err := os.MkdirAll(s.dataDir, 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func GeneratePassword() (string, error) {
	out := make([]byte, 20)
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i := range out {
		out[i] = passwordChars[int(buf[i])%len(passwordChars)]
	}
	return string(out), nil
}

func ValidatePassword(password string) error {
	if len(password) < MinPassword || len(password) > MaxPassword {
		return fmt.Errorf("password must be %d-%d characters", MinPassword, MaxPassword)
	}
	for _, r := range password {
		if r < 0x20 || r == 0x7f {
			return errors.New("password must not contain control characters")
		}
	}
	return nil
}

func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
