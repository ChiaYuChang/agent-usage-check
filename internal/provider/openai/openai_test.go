package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"agent-usage-check/internal/provider"
)

func TestOpenAIFetchUsage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/usage", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"rate_limit": {
				"primary_window": { "remaining_percent": 85, "reset_after_seconds": 3600 },
				"secondary_window": { "used_percent": 40, "reset_after_seconds": 72000 }
			}
		}`))
	})
	mux.HandleFunc("/credits", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"credits": [
				{ "status": "available", "expires_at": "2026-08-30T00:00:00Z" }
			]
		}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	tempDir := t.TempDir()
	authFile := filepath.Join(tempDir, "auth.json")
	if err := os.WriteFile(authFile, []byte(`{"tokens":{"access_token":"test-token"}}`), 0600); err != nil {
		t.Fatalf("failed to write auth file: %v", err)
	}

	p := New(authFile, server.Client())
	p.UsageEndpoint = server.URL + "/usage"
	p.CreditsEndpoint = server.URL + "/credits"

	st, err := p.FetchUsage(context.Background())
	if err != nil {
		t.Fatalf("FetchUsage failed: %v", err)
	}

	if st.Provider != "openai" {
		t.Errorf("Provider = %q, want openai", st.Provider)
	}

	rem5h, ok := provider.Percentage(st.Primary)
	if !ok || rem5h != 85 {
		t.Errorf("Primary = (%d, %t), want (85, true)", rem5h, ok)
	}

	rem1w, ok := provider.Percentage(st.Secondary)
	if !ok || rem1w != 60 { // used_percent 40 -> remaining 60
		t.Errorf("Secondary = (%d, %t), want (60, true)", rem1w, ok)
	}

	if len(st.Credits) != 1 || st.Credits[0].Status != "available" {
		t.Errorf("unexpected credits: %+v", st.Credits)
	}
}
