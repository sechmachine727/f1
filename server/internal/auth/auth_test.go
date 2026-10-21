package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnsureCreatesOnce(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	first, err := s.Ensure()
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || first.Password == "" || !s.Verify(first.Password) {
		t.Fatalf("first ensure %#v", first)
	}
	second, err := New(dir).Ensure()
	if err != nil {
		t.Fatal(err)
	}
	if second.Created || second.Password != "" || !s.Verify(first.Password) {
		t.Fatalf("second ensure %#v", second)
	}
	if fi, err := os.Stat(filepath.Join(dir, "auth.json")); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("auth file mode: %v %v", fi, err)
	}
}

func TestResetInvalidatesSessions(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	first, err := s.Ensure()
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := s.IssueSession(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !s.ValidateSession(token, time.Now()) {
		t.Fatal("new session rejected")
	}
	if err := s.Reset("new-password-123"); err != nil {
		t.Fatal(err)
	}
	if s.ValidateSession(token, time.Now()) {
		t.Fatal("session survived reset")
	}
	if s.Verify(first.Password) || !s.Verify("new-password-123") {
		t.Fatal("password reset mismatch")
	}
}

func TestSessionExpiryAndTamper(t *testing.T) {
	s := New(t.TempDir())
	if _, err := s.Ensure(); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	token, _, err := s.IssueSession(now)
	if err != nil {
		t.Fatal(err)
	}
	if !s.ValidateSession(token, now.Add(time.Hour)) {
		t.Fatal("fresh session rejected")
	}
	if s.ValidateSession(token, now.Add(SessionTTL+time.Minute)) {
		t.Fatal("expired session accepted")
	}
	if s.ValidateSession(token+"x", now) {
		t.Fatal("tampered session accepted")
	}
}

func TestEnvironmentOwnsPassword(t *testing.T) {
	t.Setenv("F1TELE_AUTH_PASSWORD", "from-env-password")
	s := New(t.TempDir())
	r, err := s.Ensure()
	if err != nil {
		t.Fatal(err)
	}
	if r.Created || r.Password != "" || !s.Verify("from-env-password") || s.Verify("other") {
		t.Fatalf("env ensure %#v", r)
	}
	if err := s.Reset("replacement-password"); err != ErrEnvOwned {
		t.Fatalf("reset err %v", err)
	}
}

func TestPasswordValidation(t *testing.T) {
	for _, p := range []string{"", "short", strings.Repeat("x", MaxPassword+1), "has\ncontrol"} {
		if ValidatePassword(p) == nil {
			t.Fatalf("accepted %q", p)
		}
	}
}
