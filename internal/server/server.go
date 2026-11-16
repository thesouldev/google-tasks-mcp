package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	tasks "google.golang.org/api/tasks/v1"

	"github.com/thesouldev/google-tasks-mcp/internal/auth"
	"github.com/thesouldev/google-tasks-mcp/internal/gtasks"
)

const instructions = `Google Tasks for one signed-in Google account.

- When the user gives several tasks at once, send them all in one create_tasks call, in the order given.
- If the user does not name a list, call list_task_lists and pick the list whose title fits the task. Use the default list when nothing fits. Say which list you used.
- Google Tasks stores a due date only, never a time. If the user gives a time, keep it in the title, e.g. "Call dentist (3pm)".
- Tasks with a due date show on Google Calendar as all-day items. Tasks without one do not.
- To complete, edit, or delete a task the user describes in words, find its id with list_tasks (use the text filter) first.
- Confirm with the user before delete_tasks, delete_task_list, or clear_completed.
- If a tool reports "waiting for Google sign-in", show the user the link, and retry once they say they have approved access.`

type Options struct {
	// Bounds how long a request waits on the browser. An unfinished sign-in hands
	// the link back to the user rather than holding the request open.
	SignInWait time.Duration
}

type Server struct {
	mcp  *mcp.Server
	wait time.Duration

	mu  sync.Mutex
	svc *gtasks.Service

	signMu  sync.Mutex
	pending *auth.SignIn
}

func New(version string, opts Options) *Server {
	if opts.SignInWait == 0 {
		opts.SignInWait = 2 * time.Minute
	}
	s := &Server{
		mcp:  mcp.NewServer(&mcp.Implementation{Name: "google-tasks", Version: version}, &mcp.ServerOptions{Instructions: instructions}),
		wait: opts.SignInWait,
	}
	s.registerTools()
	return s
}

func (s *Server) MCP() *mcp.Server { return s.mcp }

func (s *Server) Run(ctx context.Context) error {
	defer s.Close()
	return s.mcp.Run(ctx, &mcp.StdioTransport{})
}

// A sign-in left open after the client disconnects would hold its port until it times out.
func (s *Server) Close() {
	s.signMu.Lock()
	defer s.signMu.Unlock()
	if s.pending != nil {
		s.pending.Cancel()
	}
}

func (s *Server) service() (*gtasks.Service, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.svc != nil {
		return s.svc, nil
	}
	ts, err := auth.TokenSource(context.Background())
	if err != nil {
		return nil, err
	}
	api, err := tasks.NewService(context.Background(), option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	s.svc = gtasks.New(api)
	return s.svc, nil
}

func (s *Server) forgetService() {
	s.mu.Lock()
	s.svc = nil
	s.mu.Unlock()
}

// Google reports an expired or revoked refresh token as invalid_grant, and a
// Workspace reauthentication policy as invalid_rapt. Both recover by signing in
// again, so they start a sign-in instead of failing the request.
func needsSignIn(err error) bool {
	if errors.Is(err, auth.ErrSignInRequired) {
		return true
	}
	var rerr *oauth2.RetrieveError
	if errors.As(err, &rerr) && (rerr.ErrorCode == "invalid_grant" || rerr.ErrorCode == "invalid_rapt") {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "invalid_grant") || strings.Contains(msg, "invalid_rapt")
}

// Agents call tools in parallel; sharing one sign-in keeps it to one browser tab.
func (s *Server) ensureSignedIn(ctx context.Context) error {
	s.signMu.Lock()
	p := s.pending
	if p == nil {
		var err error
		if p, err = auth.BeginSignIn(); err != nil {
			s.signMu.Unlock()
			return err
		}
		s.pending = p
		auth.OpenBrowser(p.URL)
		go func() {
			<-p.Done()
			s.signMu.Lock()
			if s.pending == p {
				s.pending = nil
			}
			s.signMu.Unlock()
		}()
	}
	s.signMu.Unlock()

	select {
	case <-p.Done():
		return p.Err()
	case <-time.After(s.wait):
		return fmt.Errorf("waiting for Google sign-in. Approve access in the browser tab that opened, "+
			"or open this link: %s Then retry the request", p.URL)
	case <-ctx.Done():
		return ctx.Err()
	}
}

type validator interface{ validate() error }

func call[T any](ctx context.Context, s *Server, fn func(*gtasks.Service) (T, error)) (T, error) {
	var zero T
	svc, err := s.service()
	if err == nil {
		out, ferr := fn(svc)
		if ferr == nil {
			return out, nil
		}
		err = ferr
	}
	if !needsSignIn(err) {
		return zero, errors.New(gtasks.Message(err))
	}
	s.forgetService()
	if err := s.ensureSignedIn(ctx); err != nil {
		return zero, err
	}
	if svc, err = s.service(); err != nil {
		return zero, err
	}
	out, err := fn(svc)
	if err != nil {
		return zero, errors.New(gtasks.Message(err))
	}
	return out, nil
}

func addTool[In, Out any](s *Server, t *mcp.Tool, fn func(context.Context, *gtasks.Service, In) (Out, error)) {
	mcp.AddTool(s.mcp, t, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		if v, ok := any(in).(validator); ok {
			if err := v.validate(); err != nil {
				var zero Out
				return nil, zero, err
			}
		}
		out, err := call(ctx, s, func(svc *gtasks.Service) (Out, error) { return fn(ctx, svc, in) })
		return nil, out, err
	})
}
