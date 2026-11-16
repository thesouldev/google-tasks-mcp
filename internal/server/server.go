package server

import (
	"context"
	"errors"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
- Confirm with the user before delete_tasks, delete_task_list, or clear_completed.`

type Server struct {
	mcp *mcp.Server

	mu  sync.Mutex
	svc *gtasks.Service
}

func New(version string) *Server {
	s := &Server{
		mcp: mcp.NewServer(&mcp.Implementation{Name: "google-tasks", Version: version}, &mcp.ServerOptions{Instructions: instructions}),
	}
	s.registerTools()
	return s
}

func (s *Server) MCP() *mcp.Server { return s.mcp }

func (s *Server) Run(ctx context.Context) error {
	return s.mcp.Run(ctx, &mcp.StdioTransport{})
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

type validator interface{ validate() error }

func call[T any](s *Server, fn func(*gtasks.Service) (T, error)) (T, error) {
	var zero T
	svc, err := s.service()
	if err != nil {
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
		out, err := call(s, func(svc *gtasks.Service) (Out, error) { return fn(ctx, svc, in) })
		return nil, out, err
	})
}
