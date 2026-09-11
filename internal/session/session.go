package session

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	EnvPath   = "REDDIT_MCP_SESSION"
	configDir = "reddit-mcp"
	fileName  = "session.json"
	dirPerm   = 0o700
	filePerm  = 0o600

	CookieSession = "reddit_session"
	CookieToken   = "token_v2"
	NoExpiry      = -1
)

var ErrNoSession = errors.New("no saved reddit session; run `reddit-mcp login` first")

type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HttpOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
}

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Session struct {
	Username        string    `json:"username"`
	UserAgent       string    `json:"user_agent"`
	Cookies         []Cookie  `json:"cookies"`
	DocumentHeaders []Header  `json:"document_headers"`
	FetchHeaders    []Header  `json:"fetch_headers"`
	SavedAt         time.Time `json:"saved_at"`
}

func Path() (string, error) {
	if p := os.Getenv(EnvPath); p != "" {
		return p, nil
	}

	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, configDir, fileName), nil
}

func Load() (*Session, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}

	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, err
	}

	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}

	return &s, nil
}

func (s *Session) Save() error {
	p, err := Path()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(p), dirPerm); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	s.SavedAt = time.Now()

	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(p), fileName+".*")
	if err != nil {
		return fmt.Errorf("create temp session file: %w", err)
	}

	if err := writeAndClose(tmp, b); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("write session: %w", err)
	}

	return os.Rename(tmp.Name(), p)
}

func writeAndClose(f *os.File, b []byte) error {
	if err := f.Chmod(filePerm); err != nil {
		f.Close()
		return err
	}

	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}

	return f.Close()
}

func (s *Session) Clone() *Session {
	cp := *s
	cp.Cookies = slices.Clone(s.Cookies)
	cp.DocumentHeaders = slices.Clone(s.DocumentHeaders)
	cp.FetchHeaders = slices.Clone(s.FetchHeaders)

	return &cp
}

func (s *Session) Cookie(name string) string {
	for _, c := range s.Cookies {
		if c.Name == name {
			return c.Value
		}
	}

	return ""
}

func (s *Session) SetCookie(c Cookie) {
	for i := range s.Cookies {
		if s.Cookies[i].Name == c.Name {
			s.Cookies[i] = c
			return
		}
	}

	s.Cookies = append(s.Cookies, c)
}

func (s *Session) Token() string {
	return s.Cookie(CookieToken)
}

func (s *Session) TokenExpiry() time.Time {
	return JWTExpiry(s.Token())
}

func (s *Session) LoggedIn() bool {
	return s.Cookie(CookieSession) != "" && s.Token() != ""
}

func JWTExpiry(tok string) time.Time {
	parts := strings.Split(tok, ".")
	if len(parts) < 2 {
		return time.Time{}
	}

	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return time.Time{}
	}

	var claims struct {
		Exp float64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp == 0 {
		return time.Time{}
	}

	return time.Unix(int64(claims.Exp), 0)
}

func Remove() error {
	p, err := Path()
	if err != nil {
		return err
	}

	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}
