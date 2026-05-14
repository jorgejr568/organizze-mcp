package organizze

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestCategoryRepository_List(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/categories" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[{"id":10,"name":"Food","parent_id":null}]`)
	})
	repo := NewCategoryRepository(exec)
	cats, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(cats) != 1 || cats[0].Name != "Food" {
		t.Errorf("got %+v", cats)
	}
}

func TestCategoryRepository_Get(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/categories/10" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":10,"name":"Food"}`)
	})
	repo := NewCategoryRepository(exec)
	cat, err := repo.Get(context.Background(), 10)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if cat.ID != 10 {
		t.Errorf("got %+v", cat)
	}
}
