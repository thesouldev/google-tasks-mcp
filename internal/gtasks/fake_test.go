package gtasks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"google.golang.org/api/option"
	tasks "google.golang.org/api/tasks/v1"
)

type insertCall struct {
	list, parent, previous string
	body                   tasks.Task
}

type fakeAPI struct {
	mu      sync.Mutex
	lists   []*tasks.TaskList
	store   map[string][]*tasks.Task
	inserts []insertCall
	patches []map[string]any
	n       int
}

func newFake(t *testing.T) (*fakeAPI, *Service) {
	f := &fakeAPI{
		lists: []*tasks.TaskList{{Id: "L1", Title: "My Tasks"}, {Id: "L2", Title: "Work"}},
		store: map[string][]*tasks.Task{
			"L1": {},
			"L2": {{Id: "w1", Title: "standup notes", Status: "needsAction", Position: "1"}},
		},
	}
	mux := http.NewServeMux()
	reply := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	notFound := func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":{"code":404,"message":"Not Found"}}`)
	}
	mux.HandleFunc("GET /tasks/v1/users/@me/lists", func(w http.ResponseWriter, r *http.Request) {
		reply(w, tasks.TaskLists{Items: f.lists})
	})
	mux.HandleFunc("GET /tasks/v1/users/@me/lists/{list}", func(w http.ResponseWriter, r *http.Request) {
		reply(w, f.lists[0])
	})
	mux.HandleFunc("GET /tasks/v1/lists/{list}/tasks", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		reply(w, tasks.Tasks{Items: f.store[r.PathValue("list")]})
	})
	mux.HandleFunc("GET /tasks/v1/lists/{list}/tasks/{task}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, t := range f.store[r.PathValue("list")] {
			if t.Id == r.PathValue("task") {
				reply(w, t)
				return
			}
		}
		notFound(w)
	})
	mux.HandleFunc("POST /tasks/v1/lists/{list}/tasks", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		var body tasks.Task
		_ = json.NewDecoder(r.Body).Decode(&body)
		q := r.URL.Query()
		f.inserts = append(f.inserts, insertCall{r.PathValue("list"), q.Get("parent"), q.Get("previous"), body})
		f.n++
		body.Id, body.Status, body.Parent = fmt.Sprintf("t%d", f.n), "needsAction", q.Get("parent")
		f.store[r.PathValue("list")] = append(f.store[r.PathValue("list")], &body)
		reply(w, body)
	})
	mux.HandleFunc("PATCH /tasks/v1/lists/{list}/tasks/{task}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		raw, _ := io.ReadAll(r.Body)
		var patch map[string]any
		_ = json.Unmarshal(raw, &patch)
		f.patches = append(f.patches, patch)
		for _, t := range f.store[r.PathValue("list")] {
			if t.Id == r.PathValue("task") {
				_ = json.Unmarshal(raw, t)
				reply(w, t)
				return
			}
		}
		notFound(w)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	api, err := tasks.NewService(context.Background(), option.WithEndpoint(srv.URL+"/"), option.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}
	return f, New(api)
}
