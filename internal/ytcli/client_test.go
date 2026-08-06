package ytcli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateIssueEstimation(t *testing.T) {
	tests := []struct {
		name     string
		minutes  int
		wantBody string
	}{
		{
			name:     "set estimation",
			minutes:  120,
			wantBody: `{"$type":"Issue","customFields":[{"$type":"PeriodIssueCustomField","name":"Estimation","value":{"$type":"PeriodValue","minutes":120}}]}`,
		},
		{
			name:     "clear estimation",
			minutes:  0,
			wantBody: `{"$type":"Issue","customFields":[{"$type":"PeriodIssueCustomField","name":"Estimation","value":null}]}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath string
			var gotBody string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
					t.Errorf("expected Bearer auth, got %q", auth)
				}
				buf := make([]byte, r.ContentLength)
				_, _ = r.Body.Read(buf)
				gotBody = string(buf)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":"1"}`))
			}))
			defer server.Close()

			c := &Client{
				baseURL:    server.URL,
				token:      "test-token",
				httpClient: server.Client(),
			}

			if err := c.UpdateIssueEstimation("DEMO-1", tc.minutes); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if gotPath != "/api/issues/DEMO-1" {
				t.Errorf("expected path /api/issues/DEMO-1, got %q", gotPath)
			}

			// Compare JSON semantics rather than exact ordering
			var want, got map[string]interface{}
			if err := json.Unmarshal([]byte(tc.wantBody), &want); err != nil {
				t.Fatalf("bad wantBody: %v", err)
			}
			if err := json.Unmarshal([]byte(gotBody), &got); err != nil {
				t.Fatalf("bad response body %q: %v", gotBody, err)
			}
			wantJSON, _ := json.Marshal(want)
			gotJSON, _ := json.Marshal(got)
			if string(wantJSON) != string(gotJSON) {
				t.Errorf("expected body %s, got %s", wantJSON, gotJSON)
			}
		})
	}
}

func TestUpdateIssueEstimationEmptyConfig(t *testing.T) {
	c := &Client{}
	if err := c.UpdateIssueEstimation("DEMO-1", 60); err == nil {
		t.Fatal("expected error when no URL/token configured")
	} else if !strings.Contains(err.Error(), "missing YouTrack connection URL or token") {
		t.Errorf("unexpected error: %v", err)
	}
}
