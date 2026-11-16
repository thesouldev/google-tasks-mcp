package gtasks

import (
	"context"
	"strings"
	"testing"
)

func TestReopenClearsCompletedTimestamp(t *testing.T) {
	f, svc := newFake(t)
	f.store["L2"][0].Status = "completed"
	task, err := svc.Update(context.Background(), Ref{TaskID: "w1"}, Patch{Status: "needsAction"})
	if err != nil {
		t.Fatal(err)
	}
	if task.ListTitle != "Work" {
		t.Errorf("task found in %q, want Work", task.ListTitle)
	}
	patch := f.patches[0]
	if v, ok := patch["completed"]; !ok || v != nil {
		t.Errorf("patch = %v, want completed: null", patch)
	}
}

func TestSetStatusReportsFailuresPerTask(t *testing.T) {
	_, svc := newFake(t)
	done, failed := svc.SetStatus(context.Background(), []Ref{{TaskID: "w1"}, {TaskID: "missing"}}, "completed")
	if len(done) != 1 || len(failed) != 1 || !strings.Contains(failed[0].Error, "not found in any list") {
		t.Errorf("done %v, failed %v", done, failed)
	}
}
