package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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

	srv := New("test")
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
