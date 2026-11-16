package gtasks

import (
	"context"

	tasks "google.golang.org/api/tasks/v1"
)

func (s *Service) insert(ctx context.Context, list ListRef, parent, previous, title, notes, due string) (*tasks.Task, error) {
	body := &tasks.Task{Title: title, Notes: notes}
	if due != "" {
		body.Due = toDueTimestamp(due)
	}
	call := s.api.Tasks.Insert(list.ID, body).Context(ctx)
	if parent != "" {
		call = call.Parent(parent)
	}
	if previous != "" {
		call = call.Previous(previous)
	}
	return call.Do()
}

// The API puts every new task first in its list. Inserting one at a time, each
// after the last, keeps a batch in the order it was given.
func (s *Service) Create(ctx context.Context, items []NewTask, defaultList string) ([]Task, []Failure) {
	created := []Task{}
	failed := []Failure{}
	lastTop := map[string]string{}
	for _, item := range items {
		listRef := item.TaskList
		if listRef == "" {
			listRef = defaultList
		}
		list, err := s.ResolveList(ctx, listRef)
		if err != nil {
			failed = append(failed, Failure{Task: item.Title, Error: Message(err)})
			continue
		}
		parent, err := s.insert(ctx, list, "", lastTop[list.ID], item.Title, item.Notes, item.Due)
		if err != nil {
			failed = append(failed, Failure{Task: item.Title, Error: Message(err)})
			continue
		}
		lastTop[list.ID] = parent.Id
		created = append(created, view(parent, list))

		prev := ""
		for _, sub := range item.Subtasks {
			child, err := s.insert(ctx, list, parent.Id, prev, sub.Title, sub.Notes, sub.Due)
			if err != nil {
				failed = append(failed, Failure{Task: sub.Title, Error: Message(err)})
				continue
			}
			prev = child.Id
			created = append(created, view(child, list))
		}
	}
	return created, failed
}
