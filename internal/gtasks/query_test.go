package gtasks

import (
	"strings"
	"testing"

	tasks "google.golang.org/api/tasks/v1"
)

func TestOrderTasksNestsSubtasksUnderParents(t *testing.T) {
	got := orderTasks([]*tasks.Task{
		{Id: "c2", Parent: "p1", Position: "2"},
		{Id: "p2", Position: "2"},
		{Id: "c1", Parent: "p1", Position: "1"},
		{Id: "p1", Position: "1"},
	})
	var ids []string
	for _, t := range got {
		ids = append(ids, t.Id)
	}
	if strings.Join(ids, ",") != "p1,c1,c2,p2" {
		t.Errorf("order = %v", ids)
	}
}

func TestMatchesDueRangeAndText(t *testing.T) {
	task := Task{Title: "Pay rent", Notes: "landlord", Due: "2026-11-16"}
	if !matches(task, Query{DueTo: "2026-11-16", Text: "LANDLORD"}) {
		t.Error("expected inclusive dueTo and case-insensitive text to match")
	}
	if matches(task, Query{DueFrom: "2026-11-17"}) {
		t.Error("dueFrom after the due date matched")
	}
	if matches(Task{Title: "no date"}, Query{DueTo: "2026-11-16"}) {
		t.Error("task without a due date matched a due range")
	}
}
