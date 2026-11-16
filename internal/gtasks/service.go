package gtasks

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	tasks "google.golang.org/api/tasks/v1"
)

const listCacheTTL = time.Minute

type Service struct {
	api *tasks.Service

	mu        sync.Mutex
	lists     []ListRef
	defaultID string
	fetchedAt time.Time
}

func New(api *tasks.Service) *Service { return &Service{api: api} }

func (s *Service) Lists(ctx context.Context) ([]ListRef, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lists != nil && time.Since(s.fetchedAt) < listCacheTTL {
		return s.lists, s.defaultID, nil
	}
	var lists []ListRef
	err := s.api.Tasklists.List().MaxResults(100).Pages(ctx, func(page *tasks.TaskLists) error {
		for _, l := range page.Items {
			lists = append(lists, ListRef{ID: l.Id, Title: l.Title})
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	def, err := s.api.Tasklists.Get("@default").Context(ctx).Do()
	if err != nil {
		return nil, "", err
	}
	s.lists, s.defaultID, s.fetchedAt = lists, def.Id, time.Now()
	return lists, def.Id, nil
}

func (s *Service) forgetLists() {
	s.mu.Lock()
	s.lists = nil
	s.mu.Unlock()
}

func (s *Service) ResolveList(ctx context.Context, ref string) (ListRef, error) {
	lists, defaultID, err := s.Lists(ctx)
	if err != nil {
		return ListRef{}, err
	}
	want := strings.TrimSpace(ref)
	if want == "" || want == "@default" {
		want = defaultID
	}
	for _, l := range lists {
		if l.ID == want {
			return l, nil
		}
	}
	for _, l := range lists {
		if strings.EqualFold(l.Title, want) {
			return l, nil
		}
	}
	names := make([]string, len(lists))
	for i, l := range lists {
		names[i] = fmt.Sprintf("%q", l.Title)
	}
	return ListRef{}, fmt.Errorf("no task list named %q, existing lists: %s", want, strings.Join(names, ", "))
}

func (s *Service) listsFor(ctx context.Context, ref string) ([]ListRef, error) {
	if ref != "" {
		l, err := s.ResolveList(ctx, ref)
		return []ListRef{l}, err
	}
	lists, _, err := s.Lists(ctx)
	return lists, err
}

func (s *Service) CreateList(ctx context.Context, title string) (ListRef, error) {
	l, err := s.api.Tasklists.Insert(&tasks.TaskList{Title: title}).Context(ctx).Do()
	if err != nil {
		return ListRef{}, err
	}
	s.forgetLists()
	return ListRef{ID: l.Id, Title: l.Title}, nil
}

func (s *Service) RenameList(ctx context.Context, ref, title string) (ListRef, error) {
	list, err := s.ResolveList(ctx, ref)
	if err != nil {
		return ListRef{}, err
	}
	l, err := s.api.Tasklists.Patch(list.ID, &tasks.TaskList{Title: title}).Context(ctx).Do()
	if err != nil {
		return ListRef{}, err
	}
	s.forgetLists()
	return ListRef{ID: l.Id, Title: l.Title}, nil
}

func (s *Service) DeleteList(ctx context.Context, ref string) (ListRef, error) {
	list, err := s.ResolveList(ctx, ref)
	if err != nil {
		return ListRef{}, err
	}
	if err := s.api.Tasklists.Delete(list.ID).Context(ctx).Do(); err != nil {
		return ListRef{}, err
	}
	s.forgetLists()
	return list, nil
}
