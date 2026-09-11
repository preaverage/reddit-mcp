package server

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/preaverage/reddit-mcp/internal/auth"
	"github.com/preaverage/reddit-mcp/internal/browser"
	"github.com/preaverage/reddit-mcp/internal/reddit"
	"github.com/preaverage/reddit-mcp/internal/session"
)

const (
	defaultLoginTimeout = 5 * time.Minute

	envEnableWrites = "REDDIT_MCP_ENABLE_WRITES"
)

var errNoIDs = errors.New("ids is required")

type toolFunc[In any] func(ctx context.Context, c *reddit.Client, in In) (*mcp.CallToolResult, error)

func addTool[In any](s *Server, name, description string, fn toolFunc[In]) {
	tool := &mcp.Tool{Name: name, Description: description}

	mcp.AddTool(s.MCP, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		c, err := s.reddit()
		if err != nil {
			return nil, nil, err
		}

		res, err := fn(ctx, c, in)

		return res, nil, err
	})
}

func addPlainTool[In any](s *Server, name, description string, fn func(ctx context.Context, in In) (*mcp.CallToolResult, error)) {
	tool := &mcp.Tool{Name: name, Description: description}

	mcp.AddTool(s.MCP, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		res, err := fn(ctx, in)

		return res, nil, err
	})
}

func listOptions(so sortOpts, po pageOpts) reddit.ListOptions {
	return reddit.ListOptions{Sort: so.Sort, Time: so.Time, Limit: po.Limit, After: po.After}
}

func (s *Server) registerTools() {
	addTool(s, "get_me", "Profile of the logged-in Reddit account (karma, inbox count, etc.).",
		func(ctx context.Context, c *reddit.Client, in rawOpt) (*mcp.CallToolResult, error) {
			r, err := c.Me(ctx)
			return result(r, err, in.Raw)
		})

	addTool(s, "get_user", "Public profile of any Reddit user.",
		func(ctx context.Context, c *reddit.Client, in userIn) (*mcp.CallToolResult, error) {
			r, err := c.User(ctx, in.Username)
			return result(r, err, in.Raw)
		})

	addTool(s, "get_post", "Fetch a single post (title, body, score, metadata) by URL or id. Does not include comments.",
		func(ctx context.Context, c *reddit.Client, in postIn) (*mcp.CallToolResult, error) {
			r, err := c.Post(ctx, in.Post)
			return result(r, err, in.Raw)
		})

	addTool(s, "get_post_comments", "Fetch a post together with its comment tree. Truncated branches expose more_ids; expand them with get_more_comments.",
		func(ctx context.Context, c *reddit.Client, in commentsIn) (*mcp.CallToolResult, error) {
			r, err := c.Comments(ctx, in.Post, reddit.CommentsOptions{
				Sort:    in.Sort,
				Limit:   in.Limit,
				Depth:   in.Depth,
				Comment: in.Comment,
				Context: in.Context,
			})
			return result(r, err, in.Raw)
		})

	addTool(s, "get_more_comments", "Expand truncated comments (from a more_ids field). Returns a flat list; use parent_id and depth to place them.",
		func(ctx context.Context, c *reddit.Client, in moreIn) (*mcp.CallToolResult, error) {
			if len(in.IDs) == 0 {
				return nil, errNoIDs
			}

			r, err := c.MoreChildren(ctx, in.Post, in.IDs, in.Sort)
			return result(r, err, in.Raw)
		})

	addTool(s, "get_user_posts", "Posts submitted by a user. Omit username for your own posts.",
		func(ctx context.Context, c *reddit.Client, in userListIn) (*mcp.CallToolResult, error) {
			r, err := c.UserPosts(ctx, in.Username, listOptions(in.sortOpts, in.pageOpts))
			return result(r, err, in.Raw)
		})

	addTool(s, "get_user_comments", "Comments written by a user. Omit username for your own comments.",
		func(ctx context.Context, c *reddit.Client, in userListIn) (*mcp.CallToolResult, error) {
			r, err := c.UserComments(ctx, in.Username, listOptions(in.sortOpts, in.pageOpts))
			return result(r, err, in.Raw)
		})

	addTool(s, "get_saved_posts", "Posts the logged-in account has saved.",
		func(ctx context.Context, c *reddit.Client, in savedIn) (*mcp.CallToolResult, error) {
			r, err := c.Saved(ctx, listOptions(sortOpts{}, in.pageOpts))
			return result(r, err, in.Raw)
		})

	addTool(s, "get_subreddit_posts", "Posts from a subreddit feed, or the logged-in home feed when subreddit is omitted. Sorts: hot, new, top, rising, controversial, best (home only).",
		func(ctx context.Context, c *reddit.Client, in subListIn) (*mcp.CallToolResult, error) {
			r, err := c.SubredditPosts(ctx, in.Subreddit, listOptions(in.sortOpts, in.pageOpts))
			return result(r, err, in.Raw)
		})

	addTool(s, "get_subreddit", "Metadata about a subreddit (description, subscribers, type).",
		func(ctx context.Context, c *reddit.Client, in subIn) (*mcp.CallToolResult, error) {
			r, err := c.SubredditInfo(ctx, in.Subreddit)
			return result(r, err, in.Raw)
		})

	addTool(s, "search_posts", "Search Reddit posts, optionally within one subreddit. Sorts: relevance, hot, top, new, comments.",
		func(ctx context.Context, c *reddit.Client, in searchIn) (*mcp.CallToolResult, error) {
			r, err := c.Search(ctx, in.Query, reddit.SearchOptions{
				Subreddit: in.Subreddit,
				Sort:      in.Sort,
				Time:      in.Time,
				Limit:     in.Limit,
				After:     in.After,
				NSFW:      in.NSFW,
			})
			return result(r, err, in.Raw)
		})

	s.registerWriteTools()

	addPlainTool(s, "session_status", "Show whether a Reddit session is saved, for which user, and when the bearer token expires.",
		func(_ context.Context, _ emptyIn) (*mcp.CallToolResult, error) {
			return s.sessionStatus()
		})

	addPlainTool(s, "logout", "Log out: revoke the Reddit session server-side, then delete the saved session and browser profile. You will need to log in again afterwards.",
		func(ctx context.Context, _ emptyIn) (*mcp.CallToolResult, error) {
			res, err := auth.Logout(ctx)
			s.resetClient()
			if err != nil {
				return nil, err
			}

			return jsonResult(res), nil
		})

	addPlainTool(s, "login", "Open a browser window so the user can log in to Reddit. Returns straight away if the saved session already works, unless force is set.",
		func(ctx context.Context, in loginIn) (*mcp.CallToolResult, error) {
			return s.login(ctx, time.Duration(in.TimeoutSeconds)*time.Second, in.Force)
		})
}

// writesEnabled reads the opt-in switch. Only affirmative values count, so an
// installer that passes "false" for an unchecked box keeps writes off.
func writesEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(envEnableWrites))) {
	case "1", "true", "yes", "on":
		return true
	}

	return false
}

// registerWriteTools adds the tools that change things on Reddit. They stay
// out unless REDDIT_MCP_ENABLE_WRITES is set, so a default install can only
// read.
func (s *Server) registerWriteTools() {
	if !writesEnabled() {
		return
	}

	addTool(s, "submit_post", "Create a post in a subreddit, as either a text post or a link post. This publishes publicly under the logged-in account.",
		func(ctx context.Context, c *reddit.Client, in submitIn) (*mcp.CallToolResult, error) {
			r, err := c.Submit(ctx, reddit.SubmitOptions{
				Subreddit:            in.Subreddit,
				Title:                in.Title,
				Text:                 in.Text,
				URL:                  in.URL,
				NSFW:                 in.NSFW,
				Spoiler:              in.Spoiler,
				FlairID:              in.FlairID,
				FlairText:            in.FlairText,
				NoReplyNotifications: in.NoReplies,
			})
			return result(r, err, in.Raw)
		})

	addTool(s, "reply", "Comment on a post, or reply to another comment. This publishes publicly under the logged-in account.",
		func(ctx context.Context, c *reddit.Client, in replyIn) (*mcp.CallToolResult, error) {
			r, err := c.Reply(ctx, in.Parent, in.Text)
			return result(r, err, in.Raw)
		})

	addTool(s, "edit", "Rewrite the body of your own text post or comment. Link posts have no body and cannot be edited.",
		func(ctx context.Context, c *reddit.Client, in editIn) (*mcp.CallToolResult, error) {
			r, err := c.Edit(ctx, in.Target, in.Text)
			return result(r, err, in.Raw)
		})

	addTool(s, "delete", "Permanently delete your own post or comment. This cannot be undone.",
		func(ctx context.Context, c *reddit.Client, in deleteIn) (*mcp.CallToolResult, error) {
			r, err := c.Delete(ctx, in.Target)
			return result(r, err, in.Raw)
		})
}

func (s *Server) sessionStatus() (*mcp.CallToolResult, error) {
	sess, err := session.Load()
	if errors.Is(err, session.ErrNoSession) {
		return jsonResult(map[string]any{
			"logged_in": false,
			"hint":      "call the login tool or run `reddit-mcp login`",
		}), nil
	}
	if err != nil {
		return nil, err
	}

	exp := sess.TokenExpiry()

	return jsonResult(map[string]any{
		"logged_in":      sess.LoggedIn(),
		"writes_enabled": writesEnabled(),
		"username":       sess.Username,
		"user_agent":     sess.UserAgent,
		"token_expires":  exp,
		"token_valid":    time.Now().Before(exp),
		"cookies":        len(sess.Cookies),
		"saved_at":       sess.SavedAt,
	}), nil
}

func (s *Server) login(ctx context.Context, timeout time.Duration, force bool) (*mcp.CallToolResult, error) {
	if timeout <= 0 {
		timeout = defaultLoginTimeout
	}

	if !force {
		if c, err := s.reddit(); err == nil {
			if me, err := c.Me(ctx); err == nil {
				return jsonResult(map[string]any{
					"logged_in": true,
					"username":  me.Data.Name,
					"note":      "already signed in; pass force to sign in again",
				}), nil
			}
		}
	}

	exe, err := s.browserPath()
	if err != nil {
		return nil, err
	}

	sess, err := browser.Login(ctx, exe, timeout)
	if err != nil {
		return nil, err
	}
	if err := sess.Save(); err != nil {
		return nil, err
	}

	s.resetClient()

	c, err := s.reddit()
	if err != nil {
		return nil, err
	}

	me, err := c.Me(ctx)
	if err != nil {
		log.Printf("login saved but profile fetch failed: %v", err)
		return jsonResult(map[string]any{"logged_in": true, "warning": err.Error()}), nil
	}

	return jsonResult(map[string]any{"logged_in": true, "username": me.Data.Name}), nil
}
