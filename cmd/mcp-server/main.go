package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/preaverage/reddit-mcp/internal/auth"
	"github.com/preaverage/reddit-mcp/internal/browser"
	"github.com/preaverage/reddit-mcp/internal/reddit"
	"github.com/preaverage/reddit-mcp/internal/server"
	"github.com/preaverage/reddit-mcp/internal/session"
)

const (
	version = "1.0.0"

	cmdServe   = "serve"
	cmdLogin   = "login"
	cmdWhoami  = "whoami"
	cmdRefresh = "refresh"
	cmdLogout  = "logout"
	cmdHelp    = "help"

	defaultLoginTimeout = 5 * time.Minute
	usage               = "usage: reddit-mcp [serve|login|whoami|refresh|logout]"
)

type command func(ctx context.Context, args []string) error

var commands = map[string]command{
	cmdServe:   serve,
	cmdLogin:   login,
	cmdWhoami:  whoami,
	cmdRefresh: refresh,
	cmdLogout:  logout,
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(0)

	name, args := parseArgs(os.Args[1:])

	if name == cmdHelp {
		fmt.Fprintln(os.Stderr, usage)
		return
	}

	run, ok := commands[name]
	if !ok {
		log.Fatalf("unknown command %q\n%s", name, usage)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, args); err != nil {
		log.Fatal(err)
	}
}

func parseArgs(args []string) (string, []string) {
	if len(args) == 0 || args[0][0] == '-' {
		if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
			return cmdHelp, nil
		}

		return cmdServe, args
	}

	return args[0], args[1:]
}

func camoufox() (string, error) {
	path, err := browser.Install()
	if err != nil {
		return "", fmt.Errorf("install camoufox: %w", err)
	}

	return path, nil
}

func serve(ctx context.Context, _ []string) error {
	return server.New(version).Run(ctx)
}

func browserRefresh(ctx context.Context) (*session.Session, error) {
	exe, err := camoufox()
	if err != nil {
		return nil, err
	}

	return browser.Refresh(ctx, exe)
}

func login(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet(cmdLogin, flag.ContinueOnError)
	timeout := fs.Duration("timeout", defaultLoginTimeout, "how long to wait for the login")
	force := fs.Bool("force", false, "sign in again even when the saved session still works")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if !*force {
		if name, ok := workingSession(ctx); ok {
			log.Printf("already signed in as %s; pass -force to sign in again", name)
			return nil
		}
	}

	exe, err := camoufox()
	if err != nil {
		return err
	}

	sess, err := browser.Login(ctx, exe, *timeout)
	if err != nil {
		return err
	}

	return saveAndReport(ctx, sess)
}

func refresh(ctx context.Context, _ []string) error {
	exe, err := camoufox()
	if err != nil {
		return err
	}

	sess, err := browser.Refresh(ctx, exe)
	if err != nil {
		return err
	}

	return saveAndReport(ctx, sess)
}

func logout(ctx context.Context, _ []string) error {
	res, err := auth.Logout(ctx)
	if err != nil {
		return err
	}

	switch {
	case res.Revoked:
		log.Println("reddit session revoked and local state removed")
	case res.RevokeError != "":
		log.Printf("local state removed; server-side revoke failed: %s", res.RevokeError)
	default:
		log.Println("no saved session; local state removed")
	}

	return nil
}

// workingSession names the account the saved session belongs to, when that
// session still gets an answer out of Reddit.
func workingSession(ctx context.Context) (string, bool) {
	sess, err := session.Load()
	if err != nil {
		return "", false
	}

	c, err := reddit.New(sess, nil)
	if err != nil {
		return "", false
	}

	me, err := c.Me(ctx)
	if err != nil {
		return "", false
	}

	return me.Data.Name, true
}

func saveAndReport(ctx context.Context, sess *session.Session) error {
	if err := sess.Save(); err != nil {
		return err
	}

	path, _ := session.Path()
	log.Printf("session saved to %s", path)

	// A cancelled run has nothing left to verify, and the session is already safe.
	if ctx.Err() != nil {
		return nil
	}

	return whoami(ctx, nil)
}

func whoami(ctx context.Context, _ []string) error {
	sess, err := session.Load()
	if err != nil {
		return err
	}

	c, err := reddit.New(sess, browserRefresh)
	if err != nil {
		return err
	}

	me, err := c.Me(ctx)
	if err != nil {
		return err
	}

	b, _ := json.MarshalIndent(me.Data, "", "  ")
	fmt.Fprintln(os.Stderr, string(b))
	log.Printf("token_v2 expires %s", c.Session().TokenExpiry().Local().Format(time.RFC1123))

	return nil
}
