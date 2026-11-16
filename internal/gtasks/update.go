package gtasks

import (
	"context"
	"sync"

	tasks "google.golang.org/api/tasks/v1"
)

func (s *Service) Update(ctx context.Context, ref Ref, p Patch) (Task, error) {
	list, _, err := s.locate(ctx, ref)
	if err != nil {
		return Task{}, err
	}
	body := &tasks.Task{}
	if p.Title != nil {
		body.Title = *p.Title
	}
	if p.Notes != nil {
		body.Notes = *p.Notes
		body.ForceSendFields = append(body.ForceSendFields, "Notes")
	}
	switch {
	case p.ClearDue:
		body.NullFields = append(body.NullFields, "Due")
	case p.Due != "":
		body.Due = toDueTimestamp(p.Due)
	}
	switch p.Status {
	case "completed":
		body.Status = "completed"
	case "needsAction":
		body.Status = "needsAction"
		body.NullFields = append(body.NullFields, "Completed")
	}
	t, err := s.api.Tasks.Patch(list.ID, ref.TaskID, body).Context(ctx).Do()
	if err != nil {
		return Task{}, err
	}
	return view(t, list), nil
}

func (s *Service) SetStatus(ctx context.Context, refs []Ref, status string) ([]Task, []Failure) {
	return eachRef(refs, func(r Ref) (Task, error) {
		return s.Update(ctx, r, Patch{Status: status})
	})
}

func (s *Service) Delete(ctx context.Context, refs []Ref) ([]Task, []Failure) {
	return eachRef(refs, func(r Ref) (Task, error) {
		list, t, err := s.locate(ctx, r)
		if err != nil {
			return Task{}, err
		}
		if err := s.api.Tasks.Delete(list.ID, r.TaskID).Context(ctx).Do(); err != nil {
			return Task{}, err
		}
		return view(t, list), nil
	})
}

func eachRef(refs []Ref, fn func(Ref) (Task, error)) ([]Task, []Failure) {
	results := make([]Task, len(refs))
	errs := make([]error, len(refs))
	var wg sync.WaitGroup
	for i, r := range refs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = fn(r)
		}()
	}
	wg.Wait()
	done := []Task{}
	failed := []Failure{}
	for i, err := range errs {
		if err != nil {
			failed = append(failed, Failure{Task: refs[i].TaskID, Error: Message(err)})
		} else {
			done = append(done, results[i])
		}
	}
	return done, failed
}

func (s *Service) Move(ctx context.Context, m Move) (Task, error) {
	list, _, err := s.locate(ctx, m.Ref)
	if err != nil {
		return Task{}, err
	}
	dest := list
	if m.DestinationTaskList != "" {
		if dest, err = s.ResolveList(ctx, m.DestinationTaskList); err != nil {
			return Task{}, err
		}
	}
	call := s.api.Tasks.Move(list.ID, m.TaskID).Context(ctx)
	if dest.ID != list.ID {
		call = call.DestinationTasklist(dest.ID)
	}
	if m.ParentID != "" {
		call = call.Parent(m.ParentID)
	}
	if m.PreviousID != "" {
		call = call.Previous(m.PreviousID)
	}
	t, err := call.Do()
	if err != nil {
		return Task{}, err
	}
	return view(t, dest), nil
}

func (s *Service) ClearCompleted(ctx context.Context, ref string) (ListRef, error) {
	list, err := s.ResolveList(ctx, ref)
	if err != nil {
		return ListRef{}, err
	}
	return list, s.api.Tasks.Clear(list.ID).Context(ctx).Do()
}
