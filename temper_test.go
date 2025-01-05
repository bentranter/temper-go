package temper

import (
	"os"
	"testing"
)

func TestClientCheckEmptyFilter(t *testing.T) {
	// Create a client where the check for the initial filter is guaranteed to fail.
	client := New("FAKE_KEY", "", &Option{BaseURL: "http://localhost:3000"})

	if client.Check("anything") {
		t.Fatal("expected nil client to return false")
	}
}

func TestClientCheck(t *testing.T) {
	if os.Getenv("SMOKE_TEST") == "" {
		t.Skip("skipping smoke tests, set SMOKE_TEST=true to run with smoke tests enabled")
	}

	client := New(
		os.Getenv("TEMPER_PUBLISHABLE_KEY"),
		os.Getenv("TEMPER_SECRET_KEY"),
		&Option{BaseURL: "http://localhost:3000"},
	)

	if v := client.Check("temper_api_e2e:user:1"); !v {
		t.Errorf("expected temper_api_e2e:user:1 to be true but got %v", v)
	}

	if v := client.Check("temper_api_e2e:user:2"); v {
		t.Errorf("expected temper_api_e2e:user:2 to be false but got %v", v)
	}

	if v := client.Check("temper_api_e2e_rollout:user:3"); !v {
		t.Errorf("expected temper_api_e2e_rollout:user:3 to be true but got %v", v)
	}
}
