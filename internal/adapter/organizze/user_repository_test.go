package organizze

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestUserRepository_Get(t *testing.T) {
	exec, _ := newTestExecutor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/3" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":3,"name":"Jorge","email":"j@x.com","role":"admin"}`)
	})

	repo := NewUserRepository(exec)
	u, err := repo.Get(context.Background(), 3)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if u.ID != 3 || u.Name != "Jorge" {
		t.Errorf("got %+v", u)
	}
}
