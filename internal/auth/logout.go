package auth

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/preaverage/reddit-mcp/internal/browser"
	"github.com/preaverage/reddit-mcp/internal/reddit"
	"github.com/preaverage/reddit-mcp/internal/session"
)

type LogoutResult struct {
	Revoked        bool   `json:"revoked"`
	RevokeError    string `json:"revoke_error,omitempty"`
	SessionRemoved bool   `json:"session_removed"`
	ProfileRemoved bool   `json:"profile_removed"`
}

func Logout(ctx context.Context) (LogoutResult, error) {
	var res LogoutResult

	res.Revoked, res.RevokeError = revoke(ctx)

	if err := session.Remove(); err != nil {
		return res, fmt.Errorf("remove session file: %w", err)
	}
	res.SessionRemoved = true

	if err := browser.RemoveProfile(); err != nil {
		return res, fmt.Errorf("remove browser profile: %w", err)
	}
	res.ProfileRemoved = true

	return res, nil
}

func revoke(ctx context.Context) (bool, string) {
	sess, err := session.Load()
	if errors.Is(err, session.ErrNoSession) {
		return false, ""
	}
	if err != nil {
		return false, err.Error()
	}

	c, err := reddit.New(sess, nil)
	if err != nil {
		return false, err.Error()
	}

	if err := c.Logout(ctx); err != nil {
		log.Printf("warning: server-side logout failed: %v", err)
		return false, err.Error()
	}

	return true, ""
}
