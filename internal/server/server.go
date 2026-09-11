package server

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/preaverage/reddit-mcp/internal/browser"
	"github.com/preaverage/reddit-mcp/internal/reddit"
	"github.com/preaverage/reddit-mcp/internal/session"
)

const serverName = "reddit-mcp"

type Server struct {
	client atomic.Pointer[reddit.Client]

	MCP *mcp.Server
}

func New(version string) *Server {
	s := &Server{}
	s.MCP = mcp.NewServer(&mcp.Implementation{Name: serverName, Version: version}, nil)
	s.registerTools()

	return s
}

func (s *Server) Run(ctx context.Context) error {
	return s.MCP.Run(ctx, &mcp.StdioTransport{})
}

func (s *Server) browserPath() (string, error) {
	path, err := browser.Install()
	if err != nil {
		return "", fmt.Errorf("camoufox unavailable: %w", err)
	}

	return path, nil
}

func (s *Server) reauth(ctx context.Context) (*session.Session, error) {
	exe, err := s.browserPath()
	if err != nil {
		return nil, err
	}

	return browser.Refresh(ctx, exe)
}

func (s *Server) reddit() (*reddit.Client, error) {
	if c := s.client.Load(); c != nil {
		return c, nil
	}

	sess, err := session.Load()
	if err != nil {
		return nil, err
	}

	c, err := reddit.New(sess, s.reauth)
	if err != nil {
		return nil, err
	}

	if s.client.CompareAndSwap(nil, c) {
		return c, nil
	}

	if existing := s.client.Load(); existing != nil {
		return existing, nil
	}

	return c, nil
}

func (s *Server) resetClient() {
	s.client.Store(nil)
}
