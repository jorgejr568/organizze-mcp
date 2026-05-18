package domain_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

// TestTags_UnmarshalJSON_AcceptsBothShapes locks in the flexible decode for
// Transaction.Tags / Transfer.Tags. The documented Organizze shape is an array
// of {name: string} objects, but GET /credit_cards/{id}/invoices/{invoice_id}
// returns the same field as a comma-separated string. Both must round-trip
// into the same domain.Tags slice.
func TestTags_UnmarshalJSON_AcceptsBothShapes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want domain.Tags
	}{
		{"null", `null`, nil},
		{"empty array", `[]`, domain.Tags{}},
		{"object array", `[{"name":"coffee"},{"name":"weekday"}]`, domain.Tags{{Name: "coffee"}, {Name: "weekday"}}},
		{"empty string", `""`, nil},
		{"single string", `"coffee"`, domain.Tags{{Name: "coffee"}}},
		{"comma-separated string", `"coffee,weekday"`, domain.Tags{{Name: "coffee"}, {Name: "weekday"}}},
		{"trims whitespace", `" coffee , weekday "`, domain.Tags{{Name: "coffee"}, {Name: "weekday"}}},
		{"drops empty parts", `"coffee,,weekday,"`, domain.Tags{{Name: "coffee"}, {Name: "weekday"}}},
		{"whitespace-only string", `"   "`, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got domain.Tags
			if err := json.Unmarshal([]byte(tt.in), &got); err != nil {
				t.Fatalf("unmarshal %q: %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

// TestTags_UnmarshalJSON_RejectsOtherShapes guards against silent acceptance
// of object / number / bool — only array and string are valid wire shapes.
func TestTags_UnmarshalJSON_RejectsOtherShapes(t *testing.T) {
	bad := []string{
		`42`,
		`true`,
		`{"name":"coffee"}`,
	}
	for _, in := range bad {
		t.Run(in, func(t *testing.T) {
			var got domain.Tags
			if err := json.Unmarshal([]byte(in), &got); err == nil {
				t.Errorf("expected error for %q, got %#v", in, got)
			}
		})
	}
}

// TestTags_Marshal_PreservesArrayShape ensures the documented outbound
// shape ([{name: ...}, ...]) is unchanged — request bodies must keep working.
func TestTags_Marshal_PreservesArrayShape(t *testing.T) {
	in := domain.Tags{{Name: "coffee"}, {Name: "weekday"}}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `[{"name":"coffee"},{"name":"weekday"}]`
	if string(b) != want {
		t.Errorf("got %s, want %s", string(b), want)
	}
}

// TestInvoice_DecodesTransactionsWithStringTags is the integration-shaped
// regression for the production error this fix targets. An Invoice payload
// whose transactions[].tags is a string (the credit-card-invoice endpoint's
// undocumented shape) must decode without error.
func TestInvoice_DecodesTransactionsWithStringTags(t *testing.T) {
	raw := `{
		"id": 318,
		"date": "2026-05-17",
		"starting_date": "2026-04-21",
		"closing_date": "2026-05-20",
		"amount_cents": 12345,
		"credit_card_id": 386176,
		"transactions": [
			{
				"id": 1,
				"description": "Coffee",
				"date": "2026-05-12",
				"amount_cents": -1500,
				"account_id": 3,
				"category_id": 21,
				"tags": "coffee,weekday"
			}
		]
	}`
	var inv domain.Invoice
	if err := json.Unmarshal([]byte(raw), &inv); err != nil {
		t.Fatalf("unmarshal invoice: %v", err)
	}
	if len(inv.Transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(inv.Transactions))
	}
	got := inv.Transactions[0].Tags
	want := domain.Tags{{Name: "coffee"}, {Name: "weekday"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tags = %#v, want %#v", got, want)
	}
}
