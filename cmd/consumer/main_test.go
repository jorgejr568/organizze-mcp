package main

import "testing"

// envInt's happy paths are exercised here. The fatal branches
// (non-integer, out-of-range) call log.Fatalf → os.Exit, which can't be
// tested in-process without subprocess plumbing — the validation logic
// is small enough that two happy-path tests plus the manual smoke test
// of `go run` with bad inputs are sufficient.

func TestEnvInt_DefaultsWhenUnset(t *testing.T) {
	const key = "STATS_TEST_ENVINT_DEFAULT"
	t.Setenv(key, "")
	if got := envInt(key, 7, 0, 100); got != 7 {
		t.Fatalf("envInt(unset) = %d, want default 7", got)
	}
}

func TestEnvInt_UsesProvidedValueWhenInRange(t *testing.T) {
	const key = "STATS_TEST_ENVINT_INRANGE"
	t.Setenv(key, "4")
	if got := envInt(key, 1, 1, 32); got != 4 {
		t.Fatalf("envInt(=4) = %d, want 4", got)
	}
}
