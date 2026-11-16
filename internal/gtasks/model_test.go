package gtasks

import "testing"

func TestDueDatesTravelAsPlainDates(t *testing.T) {
	if got := toDueTimestamp("2026-11-16"); got != "2026-11-16T00:00:00.000Z" {
		t.Errorf("toDueTimestamp = %q", got)
	}
	if got := toDate("2026-11-16T00:00:00.000Z"); got != "2026-11-16" {
		t.Errorf("toDate = %q", got)
	}
	if ValidDate("16-11-2026") == nil {
		t.Error("ValidDate accepted a day-first date")
	}
}
