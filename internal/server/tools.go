package server

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thesouldev/google-tasks-mcp/internal/gtasks"
)

func ptr[T any](v T) *T { return &v }

var (
	readOnly    = &mcp.ToolAnnotations{ReadOnlyHint: true}
	additive    = &mcp.ToolAnnotations{DestructiveHint: ptr(false)}
	idempotent  = &mcp.ToolAnnotations{DestructiveHint: ptr(false), IdempotentHint: true}
	destructive = &mcp.ToolAnnotations{DestructiveHint: ptr(true)}
)

func today() string { return time.Now().Format(time.DateOnly) }

func checkDates(fields map[string]string) error {
	for name, value := range fields {
		if value == "" {
			continue
		}
		if err := gtasks.ValidDate(value); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

type listsOut struct {
	Today         string           `json:"today"`
	DefaultListID string           `json:"defaultListId"`
	Lists         []gtasks.ListRef `json:"lists"`
}

type listTasksIn struct {
	TaskList         string `json:"taskList,omitempty" jsonschema:"list title or id; omit to search every list"`
	IncludeCompleted bool   `json:"includeCompleted,omitempty" jsonschema:"include completed tasks"`
	CompletedSince   string `json:"completedSince,omitempty" jsonschema:"only tasks completed on or after this local date, YYYY-MM-DD"`
	DueFrom          string `json:"dueFrom,omitempty" jsonschema:"only tasks due on or after this date, YYYY-MM-DD"`
	DueTo            string `json:"dueTo,omitempty" jsonschema:"only tasks due on or before this date, YYYY-MM-DD; use today's date for today and overdue"`
	Text             string `json:"text,omitempty" jsonschema:"case-insensitive match against title and notes"`
}

func (in listTasksIn) validate() error {
	return checkDates(map[string]string{"completedSince": in.CompletedSince, "dueFrom": in.DueFrom, "dueTo": in.DueTo})
}

type listTasksOut struct {
	Today string        `json:"today"`
	Count int           `json:"count"`
	Tasks []gtasks.Task `json:"tasks"`
}

type createTasksIn struct {
	TaskList string           `json:"taskList,omitempty" jsonschema:"default list for tasks that do not name one; omit for the user's default list"`
	Tasks    []gtasks.NewTask `json:"tasks" jsonschema:"tasks to create, in order"`
}

func (in createTasksIn) validate() error {
	if len(in.Tasks) == 0 || len(in.Tasks) > 100 {
		return fmt.Errorf("tasks: give between 1 and 100 tasks")
	}
	for _, t := range in.Tasks {
		if t.Title == "" {
			return fmt.Errorf("tasks: every task needs a title")
		}
		if err := checkDates(map[string]string{"due": t.Due}); err != nil {
			return err
		}
		for _, sub := range t.Subtasks {
			if err := checkDates(map[string]string{"due": sub.Due}); err != nil {
				return err
			}
		}
	}
	return nil
}

type createTasksOut struct {
	Created []gtasks.Task    `json:"created"`
	Failed  []gtasks.Failure `json:"failed"`
}

type updateTaskIn struct {
	gtasks.Ref
	Title    *string `json:"title,omitempty"`
	Notes    *string `json:"notes,omitempty" jsonschema:"new notes; an empty string clears them"`
	Due      string  `json:"due,omitempty" jsonschema:"new due date, YYYY-MM-DD"`
	ClearDue bool    `json:"clearDue,omitempty" jsonschema:"remove the due date"`
	Status   string  `json:"status,omitempty" jsonschema:"needsAction or completed"`
}

func (in updateTaskIn) validate() error {
	if in.Status != "" && in.Status != "needsAction" && in.Status != "completed" {
		return fmt.Errorf("status: use needsAction or completed")
	}
	return checkDates(map[string]string{"due": in.Due})
}

type refsIn struct {
	Tasks []gtasks.Ref `json:"tasks"`
}

func (in refsIn) validate() error {
	if len(in.Tasks) == 0 || len(in.Tasks) > 100 {
		return fmt.Errorf("tasks: give between 1 and 100 tasks")
	}
	return nil
}

type completeIn struct {
	refsIn
	Reopen bool `json:"reopen,omitempty" jsonschema:"mark the tasks as not done instead"`
}

type updatedOut struct {
	Updated []gtasks.Task    `json:"updated"`
	Failed  []gtasks.Failure `json:"failed"`
}

type deletedOut struct {
	Deleted []gtasks.Task    `json:"deleted"`
	Failed  []gtasks.Failure `json:"failed"`
}

type moveIn struct {
	gtasks.Ref
	DestinationTaskList string `json:"destinationTaskList,omitempty" jsonschema:"list title or id to move into"`
	ParentID            string `json:"parentId,omitempty" jsonschema:"task id of the new parent"`
	PreviousID          string `json:"previousId,omitempty" jsonschema:"task id of the sibling to place it after"`
}

type listIn struct {
	TaskList string `json:"taskList,omitempty" jsonschema:"list title or id; omit for the user's default list"`
}

type namedListIn struct {
	TaskList string `json:"taskList" jsonschema:"list title or id"`
}

type titleIn struct {
	Title string `json:"title"`
}

type renameIn struct {
	TaskList string `json:"taskList" jsonschema:"current list title or id"`
	Title    string `json:"title"`
}

type clearedOut struct {
	Cleared gtasks.ListRef `json:"cleared"`
}

type deletedListOut struct {
	Deleted gtasks.ListRef `json:"deleted"`
}

func (s *Server) registerTools() {
	addTool(s, &mcp.Tool{
		Name:        "list_task_lists",
		Description: "List the user's Google Tasks lists, which one is the default, and today's local date.",
		Annotations: readOnly,
	}, func(ctx context.Context, svc *gtasks.Service, _ struct{}) (listsOut, error) {
		lists, def, err := svc.Lists(ctx)
		return listsOut{Today: today(), DefaultListID: def, Lists: lists}, err
	})

	addTool(s, &mcp.Tool{
		Name: "list_tasks",
		Description: "List tasks across all lists, or one list. Open tasks only unless includeCompleted or " +
			"completedSince is set. Subtasks follow their parent and carry parentId. Due ranges are inclusive " +
			"and skip tasks with no due date.",
		Annotations: readOnly,
	}, func(ctx context.Context, svc *gtasks.Service, in listTasksIn) (listTasksOut, error) {
		found, err := svc.Query(ctx, gtasks.Query(in))
		return listTasksOut{Today: today(), Count: len(found), Tasks: found}, err
	})

	addTool(s, &mcp.Tool{
		Name: "create_tasks",
		Description: "Create one or more tasks in one call, kept in the given order at the top of their list. " +
			"Each task can name its own list and carry one level of subtasks.",
		Annotations: additive,
	}, func(ctx context.Context, svc *gtasks.Service, in createTasksIn) (createTasksOut, error) {
		if _, _, err := svc.Lists(ctx); err != nil {
			return createTasksOut{}, err
		}
		created, failed := svc.Create(ctx, in.Tasks, in.TaskList)
		return createTasksOut{Created: created, Failed: failed}, nil
	})

	addTool(s, &mcp.Tool{
		Name:        "update_task",
		Description: "Change a task's title, notes, due date, or status.",
		Annotations: idempotent,
	}, func(ctx context.Context, svc *gtasks.Service, in updateTaskIn) (gtasks.Task, error) {
		return svc.Update(ctx, in.Ref, gtasks.Patch{
			Title: in.Title, Notes: in.Notes, Due: in.Due, ClearDue: in.ClearDue, Status: in.Status,
		})
	})

	addTool(s, &mcp.Tool{
		Name:        "complete_tasks",
		Description: "Mark one or more tasks as done. Set reopen to mark them as not done instead.",
		Annotations: idempotent,
	}, func(ctx context.Context, svc *gtasks.Service, in completeIn) (updatedOut, error) {
		if _, _, err := svc.Lists(ctx); err != nil {
			return updatedOut{}, err
		}
		status := "completed"
		if in.Reopen {
			status = "needsAction"
		}
		updated, failed := svc.SetStatus(ctx, in.Tasks, status)
		return updatedOut{Updated: updated, Failed: failed}, nil
	})

	addTool(s, &mcp.Tool{
		Name:        "delete_tasks",
		Description: "Permanently delete one or more tasks. Confirm with the user first.",
		Annotations: destructive,
	}, func(ctx context.Context, svc *gtasks.Service, in refsIn) (deletedOut, error) {
		if _, _, err := svc.Lists(ctx); err != nil {
			return deletedOut{}, err
		}
		deleted, failed := svc.Delete(ctx, in.Tasks)
		return deletedOut{Deleted: deleted, Failed: failed}, nil
	})

	addTool(s, &mcp.Tool{
		Name: "move_task",
		Description: "Move a task to another list, under a parent (making it a subtask), or after a sibling. " +
			"Omit parentId to make it top-level. Omit previousId to place it first. Repeating tasks cannot change lists.",
		Annotations: additive,
	}, func(ctx context.Context, svc *gtasks.Service, in moveIn) (gtasks.Task, error) {
		return svc.Move(ctx, gtasks.Move{
			Ref: in.Ref, DestinationTaskList: in.DestinationTaskList, ParentID: in.ParentID, PreviousID: in.PreviousID,
		})
	})

	addTool(s, &mcp.Tool{
		Name:        "clear_completed",
		Description: "Hide all completed tasks in a list from the default view. Confirm with the user first.",
		Annotations: destructive,
	}, func(ctx context.Context, svc *gtasks.Service, in listIn) (clearedOut, error) {
		list, err := svc.ClearCompleted(ctx, in.TaskList)
		return clearedOut{Cleared: list}, err
	})

	addTool(s, &mcp.Tool{
		Name:        "create_task_list",
		Description: "Create a new task list.",
		Annotations: additive,
	}, func(ctx context.Context, svc *gtasks.Service, in titleIn) (gtasks.ListRef, error) {
		return svc.CreateList(ctx, in.Title)
	})

	addTool(s, &mcp.Tool{
		Name:        "rename_task_list",
		Description: "Rename an existing task list.",
		Annotations: idempotent,
	}, func(ctx context.Context, svc *gtasks.Service, in renameIn) (gtasks.ListRef, error) {
		return svc.RenameList(ctx, in.TaskList, in.Title)
	})

	addTool(s, &mcp.Tool{
		Name:        "delete_task_list",
		Description: "Permanently delete a task list and every task in it. Confirm with the user first.",
		Annotations: destructive,
	}, func(ctx context.Context, svc *gtasks.Service, in namedListIn) (deletedListOut, error) {
		list, err := svc.DeleteList(ctx, in.TaskList)
		return deletedListOut{Deleted: list}, err
	})
}
