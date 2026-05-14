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

func TestCategoryRepository_Delete_SendsReplacementIDInBody(t *testing.T) {
	var gotPath, gotCT string
	var gotBody []byte
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":6,"name":"Marketing","color":"8dd47f","parent_id":null}`)
	})
	repo := NewCategoryRepository(exec)
	rid := int64(18)
	cat, err := repo.Delete(context.Background(), 6, &rid)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if gotPath != "/categories/6?" {
		t.Errorf("path = %q, want /categories/6 with no query", gotPath)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotCT)
	}
	if string(gotBody) != `{"replacement_id":18}` {
		t.Errorf("body = %q, want {\"replacement_id\":18}", string(gotBody))
	}
	if cat == nil || cat.ID != 6 {
		t.Errorf("returned category = %+v", cat)
	}
}

func TestCategoryRepository_Delete_NilReplacement_SendsNoBody(t *testing.T) {
	var gotBody []byte
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	})
	repo := NewCategoryRepository(exec)
	if _, err := repo.Delete(context.Background(), 6, nil); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(gotBody) != 0 {
		t.Errorf("body = %q on nil replacement, want empty", string(gotBody))
	}
}
