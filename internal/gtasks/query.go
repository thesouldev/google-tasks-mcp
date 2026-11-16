package gtasks

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	tasks "google.golang.org/api/tasks/v1"
)

// Matches the order shown in the Google Tasks apps, so a listing reads the same as the app.
func orderTasks(raw []*tasks.Task) []*tasks.Task {
	byPos := func(ts []*tasks.Task) {
		sort.SliceStable(ts, func(i, j int) bool { return ts[i].Position < ts[j].Position })
	}
	ids := make(map[string]bool, len(raw))
	for _, t := range raw {
		ids[t.Id] = true
	}
	var top []*tasks.Task
	children := map[string][]*tasks.Task{}
	for _, t := range raw {
		if t.Parent != "" && ids[t.Parent] {
			children[t.Parent] = append(children[t.Parent], t)
		} else {
			top = append(top, t)
		}
	}
	byPos(top)
	out := make([]*tasks.Task, 0, len(raw))
	for _, p := range top {
		kids := children[p.Id]
		byPos(kids)
		out = append(append(out, p), kids...)
	}
	return out
}

func matches(t Task, q Query) bool {
	if q.DueFrom != "" && (t.Due == "" || t.Due < q.DueFrom) {
		return false
	}
	if q.DueTo != "" && (t.Due == "" || t.Due > q.DueTo) {
		return false
	}
	if q.CompletedSince != "" {
		since, err := time.ParseInLocation(time.DateOnly, q.CompletedSince, time.Local)
		done, perr := time.Parse(time.RFC3339, t.CompletedAt)
		if err != nil || perr != nil || done.Before(since) {
			return false
		}
	}
	if q.Text != "" {
		hay := strings.ToLower(t.Title + "\n" + t.Notes)
		if !strings.Contains(hay, strings.ToLower(q.Text)) {
			return false
		}
	}
	return true
}

func (s *Service) rawTasks(ctx context.Context, list ListRef, includeCompleted bool) ([]*tasks.Task, error) {
	var items []*tasks.Task
	err := s.api.Tasks.List(list.ID).
		MaxResults(100).
		ShowCompleted(includeCompleted).
		ShowHidden(includeCompleted).
		ShowAssigned(true).
		Pages(ctx, func(page *tasks.Tasks) error {
			items = append(items, page.Items...)
			return nil
		})
	return orderTasks(items), err
}

func (s *Service) Query(ctx context.Context, q Query) ([]Task, error) {
	lists, err := s.listsFor(ctx, q.TaskList)
	if err != nil {
		return nil, err
	}
	includeCompleted := q.IncludeCompleted || q.CompletedSince != ""
	perList := make([][]*tasks.Task, len(lists))
	errs := make([]error, len(lists))
	var wg sync.WaitGroup
	for i, l := range lists {
		wg.Add(1)
		go func() {
			defer wg.Done()
			perList[i], errs[i] = s.rawTasks(ctx, l, includeCompleted)
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	out := []Task{}
	for i, raw := range perList {
		for _, t := range raw {
			if v := view(t, lists[i]); matches(v, q) {
				out = append(out, v)
			}
		}
	}
	return out, nil
}

func (s *Service) locate(ctx context.Context, ref Ref) (ListRef, *tasks.Task, error) {
	lists, err := s.listsFor(ctx, ref.TaskList)
	if err != nil {
		return ListRef{}, nil, err
	}
	for _, l := range lists {
		t, err := s.api.Tasks.Get(l.ID, ref.TaskID).Context(ctx).Do()
		if err == nil {
			return l, t, nil
		}
		if !isNotFound(err) || ref.TaskList != "" {
			return ListRef{}, nil, err
		}
	}
	return ListRef{}, nil, fmt.Errorf("task %s not found in any list", ref.TaskID)
}
