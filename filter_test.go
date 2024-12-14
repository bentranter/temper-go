package temper

import (
	"encoding/json"
	"testing"
)

func Test_filter(t *testing.T) {
	rawFilterResp := []byte(`{"filter":"AAAAAAAAAAChyQAAAAAAAKHJAAAAAAAAONKlyQAAAAAIhwAAAAAAAAAAAAAAAAAAAAAAAAAAAABAnQAAAAAAAAAAAAAAAAAAAAAAAAAAAADLPwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAcdx5tgAAAACNEQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAPaPvckAAAAAAAAAAAAAAACSYQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==","rollout":"ZPPzHfbwt2xk7lAWLwPCQgE+Qryr1ydL"}`)
	fr := &filterResponse{}
	if err := json.Unmarshal(rawFilterResp, fr); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	f, err := from(fr)
	if err != nil {
		t.Fatalf("failed to create filter from response: %v", err)
	}

	if v := f.lookup([]byte("temper_api_e2e:user:1")); !v {
		t.Errorf("expected temper_api_e2e:user:1 to be true but got %v", v)
	}
	if v := f.lookup([]byte("temper_api_e2e:user:2")); v {
		t.Errorf("expected temper_api_e2e:user:2 to be false but got %v", v)
	}
	if v := f.lookup([]byte("temper_api_e2e_rollout:user:3")); !v {
		t.Errorf("expected temper_api_e2e_rollout:user:3 to be true but got %v", v)
	}
}

func Test_filter_zero(t *testing.T) {
	rawFilterResp := []byte(`{}`)
	fr := &filterResponse{}
	if err := json.Unmarshal(rawFilterResp, fr); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	f, err := from(fr)
	if err != nil {
		t.Fatalf("failed to create filter from empty response: %v", err)
	}

	if v := f.lookup([]byte("test:user:1")); v {
		t.Errorf("expected value to be false but got %v", v)
	}
	// Malformed key (missing the `:`) shouldn't break and should return false.
	if v := f.lookup([]byte("test")); v {
		t.Errorf("expected value to be false but got %v", v)
	}
}
