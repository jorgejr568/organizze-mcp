package organizze

import (
	"context"
	"io"
	"net/http"
	"testing"
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
