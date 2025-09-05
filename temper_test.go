package temper_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/bentranter/temper-go"
)

// mockTemperBackend
func mockTemperBackend() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/public", func(w http.ResponseWriter, r *http.Request) {
		// TODO
	})
	mux.HandleFunc("/api/public/filter", func(w http.ResponseWriter, r *http.Request) {
		rawFilterResp := []byte(`{"filter":"AAAAAAAAAAChyQAAAAAAAKHJAAAAAAAAONKlyQAAAAAIhwAAAAAAAAAAAAAAAAAAAAAAAAAAAABAnQAAAAAAAAAAAAAAAAAAAAAAAAAAAADLPwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAcdx5tgAAAACNEQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAPaPvckAAAAAAAAAAAAAAACSYQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==","rollout":"ZPPzHfbwt2xk7lAWLwPCQgE+Qryr1ydL"}`)
		w.Header().Set("Content-Type", "application/json")
		w.Write(rawFilterResp)
	})

	return mux
}

func TestMain(m *testing.M) {
	srv := httptest.NewServer(mockTemperBackend())
	defer srv.Close()

	temper.Init("FAKE_KEY", "FAKE_SECRET", &temper.Option{
		BaseURL: srv.URL,
	})

	os.Exit(m.Run())
}

func TestTemperCheck(t *testing.T) {
	if temper.Check("anything") {
		t.Fatal("expected nil client to return false")
	}

	if v := temper.Check("temper_api_e2e:user:1"); !v {
		t.Errorf("expected temper_api_e2e:user:1 to be true but got %v", v)
	}
	if v := temper.Check("temper_api_e2e:user:2"); v {
		t.Errorf("expected temper_api_e2e:user:2 to be false but got %v", v)
	}
	if v := temper.Check("temper_api_e2e_rollout:user:3"); !v {
		t.Errorf("expected temper_api_e2e_rollout:user:3 to be true but got %v", v)
	}
}

func TestTemperRefactor(t *testing.T) {
	type fnArgs struct {
		V string
	}

	type retVal struct {
		V string
	}

	result := temper.Refactor(&temper.RefactorArgs[fnArgs, retVal]{
		Name: "test",
		Old: func(args fnArgs) retVal {
			return retVal(args)
		},
		New: func(args fnArgs) retVal {
			return retVal(args)
		},
	}, fnArgs{
		V: "test",
	})

	if result.V != "test" {
		t.Fatalf("expected test, got %s", result.V)
	}
}
