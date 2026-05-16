package credprovider

import (
	"context"
	"errors"
	"testing"
)

func TestStatic_ReturnsValues(t *testing.T) {
	p := Static("e@x.com", "key", "UA")
	email, apiKey, ua, err := p(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if email != "e@x.com" || apiKey != "key" || ua != "UA" {
		t.Errorf("got %q,%q,%q", email, apiKey, ua)
	}
}

func TestFromContext_MissingErrors(t *testing.T) {
	_, _, _, err := FromContext(context.Background())
	if !errors.Is(err, ErrNoCredentials) {
		t.Errorf("expected ErrNoCredentials, got %v", err)
	}
}

func TestWithCredentialsThenFromContext(t *testing.T) {
	ctx := WithCredentials(context.Background(), Credentials{
		Email: "e@x.com", APIKey: "k", UserAgent: "UA",
	})
	email, apiKey, ua, err := FromContext(ctx)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if email != "e@x.com" || apiKey != "k" || ua != "UA" {
		t.Errorf("got %q,%q,%q", email, apiKey, ua)
	}
}
