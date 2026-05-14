package organizze

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

func TestTransferRepository_List_NoFilter(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transfers" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[]`)
	})
	repo := NewTransferRepository(exec)
	if _, err := repo.List(context.Background(), domain.ListTransfersFilter{}); err != nil {
		t.Fatalf("List: %v", err)
	}
}

func TestTransferRepository_List_WithDateRange(t *testing.T) {
	var got url.Values
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = io.WriteString(w, `[]`)
	})
	repo := NewTransferRepository(exec)
	_, err := repo.List(context.Background(), domain.ListTransfersFilter{
		StartDate: "2026-05-01",
		EndDate:   "2026-05-31",
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got.Get("start_date") != "2026-05-01" || got.Get("end_date") != "2026-05-31" {
		t.Errorf("query = %v", got)
	}
}
