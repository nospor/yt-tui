package ytcli

import "testing"

func TestExtractStringFieldValues(t *testing.T) {
	issue := Issue{
		CustomFields: []CustomField{
			{
				Name: "Boards",
				Value: []interface{}{
					map[string]interface{}{"name": "Sprint 1", "$type": "VersionBundleElement"},
					map[string]interface{}{"name": "Sprint 2", "$type": "VersionBundleElement"},
				},
			},
			{
				Name:  "Repo",
				Value: map[string]interface{}{"name": "repo-a", "$type": "EnumBundleElement"},
			},
		},
	}

	got := issue.ExtractStringFieldValues("Boards")
	want := []string{"Sprint 1", "Sprint 2"}
	if len(got) != len(want) {
		t.Fatalf("expected %d values, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("value[%d]: want %q, got %q", i, want[i], got[i])
		}
	}

	gotSingle := issue.ExtractStringFieldValues("Repo")
	if len(gotSingle) != 1 || gotSingle[0] != "repo-a" {
		t.Errorf("expected single repo value, got %v", gotSingle)
	}

	if gotMissing := issue.ExtractStringFieldValues("Missing"); gotMissing != nil {
		t.Errorf("expected nil for missing field, got %v", gotMissing)
	}
}
