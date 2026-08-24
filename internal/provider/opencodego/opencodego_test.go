package opencodego

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"agent-usage-check/internal/provider"
)

const sampleUsageJSON = `{
  "usage": {
    "rolling": { "status": "ok", "percent": 12, "resetsAt": "2026-09-10T12:00:00Z" },
    "weekly": { "status": "ok", "percent": 78, "resetsAt": "2026-09-14T00:00:00Z" },
    "monthly": { "status": "ok", "percent": 94, "resetsAt": "2026-10-01T00:00:00Z" }
  }
}`

func testServer(t *testing.T, body string, status int, wantKey string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+wantKey {
			t.Errorf("Authorization = %q, want %q", got, "Bearer "+wantKey)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
}

func TestFetchUsage(t *testing.T) {
	server := testServer(t, sampleUsageJSON, http.StatusOK, "test-key")
	defer server.Close()

	p := New("test-key", "", server.URL, server.Client())

	st, err := p.FetchUsage(context.Background())
	if err != nil {
		t.Fatalf("FetchUsage failed: %v", err)
	}

	if st.Provider != "opencode-go" {
		t.Errorf("Provider = %q, want opencode-go", st.Provider)
	}

	// percent is *used*, so remaining = 100 - percent
	rem5h, ok := provider.Percentage(st.Primary)
	if !ok || rem5h != 88 {
		t.Errorf("Primary 5h remaining = (%d, %t), want (88, true)", rem5h, ok)
	}
	rem1w, ok := provider.Percentage(st.Secondary)
	if !ok || rem1w != 22 {
		t.Errorf("Secondary 1w remaining = (%d, %t), want (22, true)", rem1w, ok)
	}

	if len(st.Groups) != 1 {
		t.Fatalf("len(Groups) = %d, want 1", len(st.Groups))
	}
	monthly, ok := st.Groups[0].Windows["monthly"]
	if !ok {
		t.Fatalf("Groups[0].Windows missing monthly: %v", st.Groups[0].Windows)
	}
	remMonthly, ok := provider.Percentage(monthly)
	if !ok || remMonthly != 6 {
		t.Errorf("monthly remaining = (%d, %t), want (6, true)", remMonthly, ok)
	}

	if st.Primary.ResetTime == nil || st.Primary.ResetAfter == nil {
		t.Errorf("Primary missing reset info: %+v", st.Primary)
	}
}

func TestFetchUsageUnauthorized(t *testing.T) {
	server := testServer(t, `{"type":"error"}`, http.StatusUnauthorized, "bad-key")
	defer server.Close()

	p := New("bad-key", "", server.URL, server.Client())
	if _, err := p.FetchUsage(context.Background()); err == nil {
		t.Fatal("expected error for 401, got nil")
	}
}

func TestFetchUsageNoSubscription(t *testing.T) {
	server := testServer(t, `{"type":"error"}`, http.StatusForbidden, "test-key")
	defer server.Close()

	p := New("test-key", "", server.URL, server.Client())
	if _, err := p.FetchUsage(context.Background()); err == nil {
		t.Fatal("expected error for 403, got nil")
	}
}

func TestResolveAPIKeyFromAuthFile(t *testing.T) {
	authFile := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(authFile, []byte(`{"opencode-go":{"key":"file-key"}}`), 0600); err != nil {
		t.Fatal(err)
	}

	p := New("", authFile, "http://127.0.0.1/", nil)
	t.Setenv("OPENCODE_API_KEY", "")
	t.Setenv("ZEN_API_KEY", "")

	key, err := p.resolveAPIKey()
	if err != nil {
		t.Fatalf("resolveAPIKey failed: %v", err)
	}
	if key != "file-key" {
		t.Errorf("key = %q, want file-key", key)
	}
}

func TestResolveAPIKeyPrefersEnvOverAuthFile(t *testing.T) {
	authFile := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(authFile, []byte(`{"opencode-go":{"key":"file-key"}}`), 0600); err != nil {
		t.Fatal(err)
	}

	p := New("", authFile, "http://127.0.0.1/", nil)
	t.Setenv("OPENCODE_API_KEY", "env-key")

	key, err := p.resolveAPIKey()
	if err != nil {
		t.Fatalf("resolveAPIKey failed: %v", err)
	}
	if key != "env-key" {
		t.Errorf("key = %q, want env-key", key)
	}
}

func TestResolveAPIKeyMissing(t *testing.T) {
	p := New("", filepath.Join(t.TempDir(), "nonexistent.json"), "http://127.0.0.1/", nil)
	t.Setenv("OPENCODE_API_KEY", "")
	t.Setenv("ZEN_API_KEY", "")

	if _, err := p.resolveAPIKey(); err == nil {
		t.Fatal("expected error when no key is available, got nil")
	}
}
