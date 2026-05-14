package organizze

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

func TestAccountRepository_List(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[{"id":1,"name":"Checking","type":"checking","default":true}]`)
	})

	repo := NewAccountRepository(exec)
	accounts, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(accounts) != 1 || accounts[0].Name != "Checking" || !accounts[0].Default {
		t.Errorf("got %+v", accounts)
	}
}

func TestAccountRepository_Get(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts/42" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":42,"name":"Itau"}`)
	})
	repo := NewAccountRepository(exec)
	acc, err := repo.Get(context.Background(), 42)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if acc.ID != 42 || acc.Name != "Itau" {
		t.Errorf("got %+v", acc)
	}
}

func TestAccountRepository_Create(t *testing.T) {
	var gotBody domain.CreateAccountParams
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/accounts" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":18,"name":"Itaú CC","type":"checking","default":true}`)
	})
	repo := NewAccountRepository(exec)
	a, err := repo.Create(context.Background(), domain.CreateAccountParams{
		Name: "Itaú CC", Type: "checking", Default: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.ID != 18 || a.Name != "Itaú CC" {
		t.Errorf("got %+v", a)
	}
	if gotBody.Name != "Itaú CC" || gotBody.Type != "checking" {
		t.Errorf("server received %+v", gotBody)
	}
}

func TestAccountRepository_Update_SendsOnlySetFields(t *testing.T) {
	var raw map[string]any
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/accounts/18" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&raw)
		_, _ = io.WriteString(w, `{"id":18,"name":"Renamed","type":"checking"}`)
	})
	repo := NewAccountRepository(exec)
	name := "Renamed"
	a, err := repo.Update(context.Background(), 18, domain.UpdateAccountParams{Name: &name})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if a.Name != "Renamed" {
		t.Errorf("got %+v", a)
	}
	if _, has := raw["type"]; has {
		t.Errorf("absent fields must be omitted; body=%v", raw)
	}
	if raw["name"] != "Renamed" {
		t.Errorf("body=%v", raw)
	}
}

func TestAccountRepository_Update_SendsArchivedWhenSet(t *testing.T) {
	var gotBody []byte
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":1,"name":"Itaú","type":"checking","archived":true}`)
	})
	repo := NewAccountRepository(exec)
	archived := true
	a, err := repo.Update(context.Background(), 1, domain.UpdateAccountParams{Archived: &archived})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if string(gotBody) != `{"archived":true}` {
		t.Errorf("body = %q, want {\"archived\":true}", string(gotBody))
	}
	if a == nil || !a.Archived {
		t.Errorf("returned = %+v", a)
	}
}

func TestAccountRepository_Delete(t *testing.T) {
	called := false
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodDelete || r.URL.Path != "/accounts/18" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	repo := NewAccountRepository(exec)
	if err := repo.Delete(context.Background(), 18); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !called {
		t.Error("handler not invoked")
	}
}
