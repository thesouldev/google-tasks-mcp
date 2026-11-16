package server

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	dir := t.TempDir()
	creds := `{"installed":{"client_id":"id","client_secret":"secret"}}`
	if err := os.WriteFile(filepath.Join(dir, "credentials.json"), []byte(creds), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_TASKS_MCP_CONFIG_DIR", dir)
	t.Setenv("GOOGLE_TASKS_MCP_NO_BROWSER", "1")

	srv := New("test", Options{SignInWait: 100 * time.Millisecond})
	t.Cleanup(srv.Close)
	clientT, serverT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.MCP().Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

var consentURL = regexp.MustCompile(`https://accounts\.google\.com\S+`)

func callForURL(t *testing.T, cs *mcp.ClientSession, tool string) *url.URL {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: map[string]any{}})
	if err != nil {
		t.Error(err)
		return &url.URL{}
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !res.IsError || !strings.Contains(text, "waiting for Google sign-in") {
		t.Errorf("%s returned %q", tool, text)
	}
	u, err := url.Parse(consentURL.FindString(text))
	if err != nil {
		t.Error(err)
		return &url.URL{}
	}
	return u
}

func TestFirstCallStartsOneSharedSignIn(t *testing.T) {
	cs := connect(t)

	var wg sync.WaitGroup
	urls := make([]*url.URL, 2)
	for i, tool := range []string{"list_task_lists", "list_tasks"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			urls[i] = callForURL(t, cs, tool)
		}()
	}
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}

	q := urls[0].Query()
	if q.Get("scope") != "https://www.googleapis.com/auth/tasks" || q.Get("code_challenge_method") != "S256" {
		t.Errorf("consent URL query = %v", q)
	}
	if urls[0].String() != urls[1].String() {
		t.Error("parallel calls started separate sign-ins")
	}

	res, err := http.Get(q.Get("redirect_uri") + "/?code=x&state=forged")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("forged callback status = %d", res.StatusCode)
	}

	time.Sleep(50 * time.Millisecond)
	again := callForURL(t, cs, "list_task_lists")
	if again.Query().Get("state") == q.Get("state") {
		t.Error("a failed sign-in was reused")
	}
}

func TestDestructiveToolsAreMarked(t *testing.T) {
	cs := connect(t)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"delete_tasks": true, "clear_completed": true, "delete_task_list": true}
	for _, tool := range res.Tools {
		a := tool.Annotations
		destructive := !a.ReadOnlyHint && (a.DestructiveHint == nil || *a.DestructiveHint)
		if destructive != want[tool.Name] {
			t.Errorf("%s destructive = %v", tool.Name, destructive)
		}
	}
	if len(res.Tools) != 11 {
		t.Errorf("got %d tools, want 11", len(res.Tools))
	}
}
