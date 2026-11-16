package gtasks

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/api/googleapi"
	tasks "google.golang.org/api/tasks/v1"
)

type ListRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	Due         string `json:"due,omitempty"`
	Notes       string `json:"notes,omitempty"`
	CompletedAt string `json:"completedAt,omitempty"`
	ParentID    string `json:"parentId,omitempty"`
	ListID      string `json:"listId"`
	ListTitle   string `json:"listTitle"`
}

type Subtask struct {
	Title string `json:"title"`
	Notes string `json:"notes,omitempty"`
	Due   string `json:"due,omitempty" jsonschema:"due date as YYYY-MM-DD"`
}

type NewTask struct {
	Title    string    `json:"title"`
	Notes    string    `json:"notes,omitempty"`
	Due      string    `json:"due,omitempty" jsonschema:"due date as YYYY-MM-DD"`
	TaskList string    `json:"taskList,omitempty" jsonschema:"list title or id for this task only"`
	Subtasks []Subtask `json:"subtasks,omitempty"`
}

type Ref struct {
	TaskID   string `json:"taskId" jsonschema:"task id from list_tasks"`
	TaskList string `json:"taskList,omitempty" jsonschema:"list title or id holding the task; omit to search all lists"`
}

type Query struct {
	TaskList         string
	IncludeCompleted bool
	CompletedSince   string
	DueFrom          string
	DueTo            string
	Text             string
}

type Patch struct {
	Title    *string
	Notes    *string
	Due      string
	ClearDue bool
	Status   string
}

type Move struct {
	Ref
	DestinationTaskList string
	ParentID            string
	PreviousID          string
}

type Failure struct {
	Task  string `json:"task"`
	Error string `json:"error"`
}

// The Tasks API keeps only the date part of a due date and discards the time,
// so dates travel as plain YYYY-MM-DD and are sent as midnight UTC.
func toDueTimestamp(date string) string { return date + "T00:00:00.000Z" }

func toDate(ts string) string {
	if len(ts) < 10 {
		return ts
	}
	return ts[:10]
}

func ValidDate(s string) error {
	if _, err := time.Parse(time.DateOnly, s); err != nil {
		return fmt.Errorf("%q is not a date, use YYYY-MM-DD", s)
	}
	return nil
}

func Message(err error) string {
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		return fmt.Sprintf("Google Tasks API %d: %s", gerr.Code, gerr.Message)
	}
	return err.Error()
}

func isNotFound(err error) bool {
	var gerr *googleapi.Error
	return errors.As(err, &gerr) && gerr.Code == http.StatusNotFound
}

func view(t *tasks.Task, list ListRef) Task {
	return Task{
		ID:          t.Id,
		Title:       t.Title,
		Status:      t.Status,
		Due:         toDate(t.Due),
		Notes:       t.Notes,
		CompletedAt: derefString(t.Completed),
		ParentID:    t.Parent,
		ListID:      list.ID,
		ListTitle:   list.Title,
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
