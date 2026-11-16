package gtasks

import (
	"context"
	"strings"
	"testing"
)

func TestCreateKeepsOrderAndNestsSubtasks(t *testing.T) {
	f, svc := newFake(t)
	created, failed := svc.Create(context.Background(), []NewTask{
		{Title: "first", Due: "2026-11-16"},
		{Title: "second", TaskList: "Work", Subtasks: []Subtask{{Title: "a"}, {Title: "b"}}},
		{Title: "third"},
		{Title: "bad", TaskList: "Nope"},
	}, "")

	if len(created) != 5 || len(failed) != 1 || !strings.Contains(failed[0].Error, `no task list named "Nope"`) {
		t.Fatalf("created %d, failed %v", len(created), failed)
	}
	in := f.inserts
	checks := []struct {
		name string
		ok   bool
	}{
		{"first goes to the top", in[0].previous == "" && in[0].list == "L1"},
		{"due is sent as midnight UTC", in[0].body.Due == "2026-11-16T00:00:00.000Z"},
		{"first task in Work goes to the top", in[1].previous == "" && in[1].list == "L2"},
		{"subtask is nested", in[2].parent == "t2" && in[2].previous == ""},
		{"second subtask follows the first", in[3].previous == "t3"},
		{"third follows first in My Tasks", in[4].previous == "t1"},
	}
	for _, c := range checks {
		if !c.ok {
			t.Errorf("%s: %+v", c.name, in)
		}
	}
}
