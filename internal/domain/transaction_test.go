package domain_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jorgejr568/organizze-mcp/internal/domain"
)

func TestCreateTransactionParams_InstallmentsAttributes_Marshals(t *testing.T) {
	p := domain.CreateTransactionParams{
		Description: "Computador",
		Date:        "2026-05-14",
		AmountCents: -100000,
		AccountID:   1,
		CategoryID:  10,
		Installments: &domain.InstallmentsAttributes{
			Periodicity: domain.PeriodicityMonthly,
			Total:       12,
		},
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"installments_attributes":{"periodicity":"monthly","total":12}`) {
		t.Errorf("body = %s", string(b))
	}
}
