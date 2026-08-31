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

func TestUpdateIssueCustomFieldSet(t *testing.T) {
	issueResponse := `{
		"customFields": [
			{
				"id": "92-10",
				"name": "Boards",
				"$type": "MultiVersionIssueCustomField",
				"value": [
					{"name": "Sprint 1", "$type": "VersionBundleElement"}
				]
			}
		]
	}`

	tests := []struct {
		name     string
		values   []string
		wantBody string
	}{
		{
			name:     "set multiple boards",
			values:   []string{"Sprint 1", "Sprint 6"},
			wantBody: `{"$type":"Issue","customFields":[{"$type":"MultiVersionIssueCustomField","name":"Boards","value":[{"$type":"VersionBundleElement","name":"Sprint 1"},{"$type":"VersionBundleElement","name":"Sprint 6"}]}]}`,
		},
		{
			name:     "clear boards",
			values:   []string{},
			wantBody: `{"$type":"Issue","customFields":[{"$type":"MultiVersionIssueCustomField","name":"Boards","value":[]}]}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(issueResponse))
				case http.MethodPost:
					buf := make([]byte, r.ContentLength)
					_, _ = r.Body.Read(buf)
					gotBody = string(buf)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"id":"1"}`))
				default:
					t.Fatalf("unexpected method %s", r.Method)
				}
			}))
			defer server.Close()

			c := &Client{
				baseURL:    server.URL,
				token:      "test-token",
				httpClient: server.Client(),
			}

			if err := c.UpdateIssueCustomFieldSet("SRDS-256", "Boards", tc.values); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

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

func TestGetProjectCustomFieldOptionsFiltersArchived(t *testing.T) {
	fieldsResponse := `[{
		"id": "92-10",
		"field": {"name": "Boards"},
		"bundle": {"id": "71-4"}
	}]`
	valuesResponse := `[
		{"name": "Sprint 2", "archived": false, "ordinal": 2},
		{"name": "Sprint 1", "archived": false, "ordinal": 1},
		{"name": "Sprint Old", "archived": true, "ordinal": 0}
	]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/bundle/values"):
			if !strings.Contains(r.URL.RawQuery, "$top=200") {
				t.Errorf("expected $top=200 in bundle values query, got %q", r.URL.RawQuery)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(valuesResponse))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(fieldsResponse))
		}
	}))
	defer server.Close()

	c := &Client{
		baseURL:    server.URL,
		token:      "test-token",
		httpClient: server.Client(),
	}

	opts, err := c.GetProjectCustomFieldOptions("0-0", "Boards")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"Sprint 1", "Sprint 2"}
	if len(opts) != len(want) {
		t.Fatalf("expected %v, got %v", want, opts)
	}
	for i := range want {
		if opts[i] != want[i] {
			t.Errorf("option[%d]: want %q, got %q", i, want[i], opts[i])
		}
	}
}

func TestResolveBoardsFieldNameForSprints(t *testing.T) {
	fieldsResponse := `[{
		"field": {"name": "Sprints"},
		"bundle": {"id": "71-4"}
	}, {
		"field": {"name": "Priority"},
		"bundle": {"id": "71-5"}
	}]`
	valuesResponse := `[
		{"name": "Sprint 1", "archived": false, "ordinal": 1},
		{"name": "Sprint 2", "archived": false, "ordinal": 2}
	]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/bundle/values") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(valuesResponse))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fieldsResponse))
	}))
	defer server.Close()

	c := &Client{
		baseURL:    server.URL,
		token:      "test-token",
		httpClient: server.Client(),
	}

	name, err := c.ResolveBoardsFieldNameForSprints("0-0", "SRDS", []string{"Sprint 1", "Sprint 2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Sprints" {
		t.Fatalf("expected field name Sprints, got %q", name)
	}
}

func TestResolveBoardsFieldFromIssue(t *testing.T) {
	response := `[
		{
			"name": "Priority",
			"$type": "SingleEnumIssueCustomField",
			"projectCustomField": {
				"field": {"name": "Priority"},
				"bundle": {"values": [{"name": "Major"}]}
			}
		},
		{
			"name": "Sprints",
			"$type": "MultiVersionIssueCustomField",
			"projectCustomField": {
				"field": {"name": "Sprints"},
				"bundle": {"values": [
					{"name": "Sprint 1"},
					{"name": "Sprint 2"}
				]}
			}
		}
	]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/customFields") {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	c := &Client{
		baseURL:    server.URL,
		token:      "test-token",
		httpClient: server.Client(),
	}

	meta, err := c.ResolveBoardsFieldFromIssue("SRDS-218", []string{"Sprint 1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta == nil || meta.Name != "Sprints" {
		t.Fatalf("expected Sprints field, got %+v", meta)
	}
	if meta.IssueFieldType != "MultiVersionIssueCustomField" {
		t.Fatalf("unexpected issue type %q", meta.IssueFieldType)
	}
}

func TestUpdateIssueCustomFieldSetWrongConfigName(t *testing.T) {
	issueResponse := `{
		"project": {"id": "0-0", "shortName": "SRDS"},
		"customFields": []
	}`
	issueFieldsResponse := `[
		{
			"name": "Boards",
			"$type": "MultiVersionIssueCustomField",
			"projectCustomField": {
				"field": {"name": "Boards"},
				"bundle": {"values": [
					{"name": "Sprint 1"},
					{"name": "Sprint 6"}
				]}
			}
		}
	]`

	var postBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if strings.Contains(r.URL.Path, "/issues/SRDS-218/customFields") {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(issueFieldsResponse))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(issueResponse))
		case http.MethodPost:
			buf := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(buf)
			postBody = string(buf)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"1"}`))
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	c := &Client{
		baseURL:    server.URL,
		token:      "test-token",
		httpClient: server.Client(),
	}

	if err := c.UpdateIssueCustomFieldSet("SRDS-218", "Sprints", []string{"Sprint 6"}, "Sprint 1", "Sprint 6"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantBody := `{"$type":"Issue","customFields":[{"$type":"MultiVersionIssueCustomField","name":"Boards","value":[{"$type":"VersionBundleElement","name":"Sprint 6"}]}]}`
	var want, got map[string]interface{}
	if err := json.Unmarshal([]byte(wantBody), &want); err != nil {
		t.Fatalf("bad wantBody: %v", err)
	}
	if err := json.Unmarshal([]byte(postBody), &got); err != nil {
		t.Fatalf("bad post body %q: %v", postBody, err)
	}
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)
	if string(wantJSON) != string(gotJSON) {
		t.Errorf("expected body %s, got %s", wantJSON, gotJSON)
	}
}

func TestUpdateIssueCustomFieldSetFromProjectMeta(t *testing.T) {
	issueResponse := `{
		"project": {"id": "0-0", "shortName": "SRDS"},
		"customFields": []
	}`
	projectFieldsResponse := `[{
		"field": {
			"name": "Sprints",
			"fieldType": {"id": "version[*]", "isMultiValue": true}
		}
	}]`

	var postBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if strings.Contains(r.URL.Path, "/customFields") {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(projectFieldsResponse))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(issueResponse))
		case http.MethodPost:
			buf := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(buf)
			postBody = string(buf)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"1"}`))
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	c := &Client{
		baseURL:    server.URL,
		token:      "test-token",
		httpClient: server.Client(),
	}

	if err := c.UpdateIssueCustomFieldSet("SRDS-218", "", []string{"Sprint 6"}, "Sprint 1", "Sprint 2", "Sprint 6"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantBody := `{"$type":"Issue","customFields":[{"$type":"MultiVersionIssueCustomField","name":"Sprints","value":[{"$type":"VersionBundleElement","name":"Sprint 6"}]}]}`
	var want, got map[string]interface{}
	if err := json.Unmarshal([]byte(wantBody), &want); err != nil {
		t.Fatalf("bad wantBody: %v", err)
	}
	if err := json.Unmarshal([]byte(postBody), &got); err != nil {
		t.Fatalf("bad post body %q: %v", postBody, err)
	}
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)
	if string(wantJSON) != string(gotJSON) {
		t.Errorf("expected body %s, got %s", wantJSON, gotJSON)
	}
}

func TestGetBoardsFieldInfoFromAgileEmbeddedSprints(t *testing.T) {
	agilesResponse := `[{
		"id": "204-9",
		"name": "SRDS Sprint",
		"projects": [{"id": "0-0", "shortName": "SRDS"}],
		"sprints": [
			{"id": "218-7", "name": "Sprint 1", "archived": false},
			{"id": "218-8", "name": "Sprint 2", "archived": false}
		],
		"sprintsSettings": {"disableSprints": false}
	}]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/sprints") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(agilesResponse))
	}))
	defer server.Close()

	c := &Client{
		baseURL:    server.URL,
		token:      "test-token",
		httpClient: server.Client(),
	}

	info, err := c.GetBoardsFieldInfo("", "SRDS")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil || len(info.Options) != 2 {
		t.Fatalf("expected sprint options, got %+v", info)
	}
}

func TestGetBoardsFieldInfoFromAgile(t *testing.T) {
	agilesResponse := `[{
		"id": "204-9",
		"name": "SRDS Sprint",
		"projects": [{"id": "0-0", "shortName": "SRDS"}],
		"sprintsSettings": {
			"disableSprints": false,
			"sprintSyncField": {"name": "Boards"}
		}
	}]`
	sprintsResponse := `[
		{"id": "218-7", "name": "Sprint 1", "archived": false},
		{"id": "218-8", "name": "Sprint 2", "archived": false}
	]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/sprints"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(sprintsResponse))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(agilesResponse))
		}
	}))
	defer server.Close()

	c := &Client{
		baseURL:    server.URL,
		token:      "test-token",
		httpClient: server.Client(),
	}

	info, err := c.GetBoardsFieldInfo("", "SRDS")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected boards field info")
	}
	if info.FieldName != "Boards" {
		t.Errorf("expected field name Boards, got %q", info.FieldName)
	}
	if len(info.Options) != 2 || info.Options[0] != "Sprint 1" {
		t.Errorf("unexpected options: %v", info.Options)
	}
	if info.AgileID != "204-9" {
		t.Errorf("expected agile id 204-9, got %q", info.AgileID)
	}
}

func TestGetBoardsFieldInfoForIssueUsesAgileSprints(t *testing.T) {
	agilesResponse := `[{
		"id": "204-9",
		"name": "SRDS Sprint",
		"projects": [{"id": "0-3", "shortName": "SRDS"}],
		"sprints": [
			{"id": "218-7", "name": "Sprint 1", "archived": false},
			{"id": "218-8", "name": "Sprint 2", "archived": false}
		],
		"sprintsSettings": {"disableSprints": false}
	}]`
	issueFieldsResponse := `[
		{"name": "Priority", "$type": "SingleEnumIssueCustomField", "projectCustomField": {"field": {"name": "Priority"}, "bundle": {"values": [{"name": "Normal"}]}}}
	]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/issues/SRDS-218/customFields"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(issueFieldsResponse))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(agilesResponse))
		}
	}))
	defer server.Close()

	c := &Client{
		baseURL:    server.URL,
		token:      "test-token",
		httpClient: server.Client(),
	}

	info, err := c.GetBoardsFieldInfoForIssue("SRDS-218", "0-3", "SRDS")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil || !info.UsesAgileSprints {
		t.Fatalf("expected agile sprint mode, got %+v", info)
	}
	if info.AgileID != "204-9" {
		t.Errorf("expected agile id 204-9, got %q", info.AgileID)
	}
}

func TestUpdateIssueSprints(t *testing.T) {
	var addCalls, deleteCalls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/issues/SRDS-218/sprints"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"218-7","name":"Sprint 1","archived":false}]`))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/issues/SRDS-218") && strings.Contains(r.URL.RawQuery, "fields=id"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"3-551"}`))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/agiles/204-9/sprints"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"id":"218-7","name":"Sprint 1","archived":false},
				{"id":"218-9","name":"Sprint 3","archived":false}
			]`))
		case r.Method == http.MethodDelete:
			deleteCalls++
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/sprints/218-9/issues"):
			addCalls++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"3-551"}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := &Client{
		baseURL:    server.URL,
		token:      "test-token",
		httpClient: server.Client(),
	}

	if err := c.UpdateIssueSprints("SRDS-218", "204-9", []string{"Sprint 3"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleteCalls != 1 {
		t.Errorf("expected 1 delete call, got %d", deleteCalls)
	}
	if addCalls != 1 {
		t.Errorf("expected 1 add call, got %d", addCalls)
	}
}

func TestUpdateIssueBoardsAgile(t *testing.T) {
	var addCalls, deleteCalls int

	getIssueResponse := `{
		"id": "3-551",
		"idReadable": "SRDS-218",
		"project": {"id": "0-3", "shortName": "SRDS"}
	}`
	agilesResponse := `[{
		"id": "204-9",
		"name": "SRDS Sprint",
		"projects": [{"id": "0-3", "shortName": "SRDS"}],
		"sprints": [
			{"id": "218-7", "name": "Sprint 1", "archived": false},
			{"id": "218-9", "name": "Sprint 3", "archived": false}
		],
		"sprintsSettings": {
			"disableSprints": false,
			"sprintSyncField": {"name": "Boards", "field": {"name": "Boards"}}
		}
	}]`
	issueFieldsResponse := `[
		{"name": "Priority", "$type": "SingleEnumIssueCustomField", "projectCustomField": {"field": {"name": "Priority"}, "bundle": {"values": [{"name": "Normal"}]}}}
	]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/issues/SRDS-218") && strings.Contains(r.URL.RawQuery, "idReadable"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(getIssueResponse))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/issues/SRDS-218/customFields"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(issueFieldsResponse))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/issues/SRDS-218/sprints"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"218-7","name":"Sprint 1","archived":false}]`))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/issues/SRDS-218") && strings.Contains(r.URL.RawQuery, "fields=id"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"3-551"}`))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/agiles/204-9/sprints"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"id":"218-7","name":"Sprint 1","archived":false},
				{"id":"218-9","name":"Sprint 3","archived":false}
			]`))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/agiles"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(agilesResponse))
		case r.Method == http.MethodDelete:
			deleteCalls++
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/sprints/218-9/issues"):
			addCalls++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"3-551"}`))
		default:
			t.Fatalf("unexpected request %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
	}))
	defer server.Close()

	c := &Client{
		baseURL:    server.URL,
		token:      "test-token",
		httpClient: server.Client(),
	}

	if err := c.UpdateIssueBoards("SRDS-218", []string{"Sprint 3"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleteCalls != 1 {
		t.Errorf("expected 1 delete call, got %d", deleteCalls)
	}
	if addCalls != 1 {
		t.Errorf("expected 1 add call, got %d", addCalls)
	}
}

func TestUpdateIssueBoardsCustomField(t *testing.T) {
	getIssueResponse := `{
		"id": "3-551",
		"idReadable": "SRDS-218",
		"project": {"id": "0-0", "shortName": "SRDS"},
		"customFields": []
	}`
	issueFieldsResponse := `[
		{
			"name": "Boards",
			"$type": "MultiVersionIssueCustomField",
			"projectCustomField": {
				"field": {"name": "Boards"},
				"bundle": {"values": [
					{"name": "Sprint 1"},
					{"name": "Sprint 6"}
				]}
			}
		}
	]`

	var postBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if strings.Contains(r.URL.Path, "/issues/SRDS-218/customFields") {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(issueFieldsResponse))
				return
			}
			if strings.Contains(r.URL.Path, "/agiles") {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`[]`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(getIssueResponse))
		case http.MethodPost:
			buf := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(buf)
			postBody = string(buf)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"1"}`))
		default:
			t.Fatalf("unexpected method %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := &Client{
		baseURL:    server.URL,
		token:      "test-token",
		httpClient: server.Client(),
	}

	if err := c.UpdateIssueBoards("SRDS-218", []string{"Sprint 6"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantBody := `{"$type":"Issue","customFields":[{"$type":"MultiVersionIssueCustomField","name":"Boards","value":[{"$type":"VersionBundleElement","name":"Sprint 6"}]}]}`
	var want, got map[string]interface{}
	if err := json.Unmarshal([]byte(wantBody), &want); err != nil {
		t.Fatalf("bad wantBody: %v", err)
	}
	if err := json.Unmarshal([]byte(postBody), &got); err != nil {
		t.Fatalf("bad post body %q: %v", postBody, err)
	}
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)
	if string(wantJSON) != string(gotJSON) {
		t.Errorf("expected body %s, got %s", wantJSON, gotJSON)
	}
}
