package gtasks

import (
	"context"
	"strings"
	"testing"
)

func TestResolveListByTitleOrDefault(t *testing.T) {
	_, svc := newFake(t)
	ctx := context.Background()
	if l, err := svc.ResolveList(ctx, "work"); err != nil || l.ID != "L2" {
		t.Errorf("ResolveList(work) = %v, %v", l, err)
	}
	if l, err := svc.ResolveList(ctx, ""); err != nil || l.ID != "L1" {
		t.Errorf("ResolveList() = %v, %v", l, err)
	}
	_, err := svc.ResolveList(ctx, "Groceries")
	if err == nil || !strings.Contains(err.Error(), `existing lists: "My Tasks", "Work"`) {
		t.Errorf("ResolveList(Groceries) error = %v", err)
	}
}
