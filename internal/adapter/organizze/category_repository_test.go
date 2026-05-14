package organizze

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
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

func TestCategoryRepository_Create(t *testing.T) {
	var gotBody domain.CreateCategoryParams
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/categories" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":42,"name":"Groceries","color":"#abcdef"}`)
	})
	repo := NewCategoryRepository(exec)
	c, err := repo.Create(context.Background(), domain.CreateCategoryParams{Name: "Groceries"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.ID != 42 || c.Name != "Groceries" {
		t.Errorf("got %+v", c)
	}
	if gotBody.Name != "Groceries" {
		t.Errorf("server received %+v", gotBody)
	}
}

func TestCategoryRepository_Update_SendsOnlySetFields(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/categories/42" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&raw)
		_, _ = io.WriteString(w, `{"id":42,"name":"Food"}`)
	})
	repo := NewCategoryRepository(exec)
	name := "Food"
	c, err := repo.Update(context.Background(), 42, domain.UpdateCategoryParams{Name: &name})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if c.Name != "Food" {
		t.Errorf("got %+v", c)
	}
	if _, has := raw["color"]; has {
		t.Errorf("absent fields must be omitted; body=%v", raw)
	}
}

func TestCategoryRepository_Delete_NoReplacement(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/categories/42" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		if r.URL.RawQuery != "" {
			t.Errorf("expected no query, got %q", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	repo := NewCategoryRepository(exec)
	if err := repo.Delete(context.Background(), 42, nil); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestCategoryRepository_Delete_WithReplacement(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/categories/42" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("replacement_id"); got != "99" {
			t.Errorf("replacement_id = %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	repo := NewCategoryRepository(exec)
	rep := int64(99)
	if err := repo.Delete(context.Background(), 42, &rep); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}
