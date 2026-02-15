package terraform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
)

func TestDetermineChangeType(t *testing.T) {
	tests := []struct {
		name    string
		actions []tfjson.Action
		want    string
		wantErr bool
	}{
		{
			name:    "create",
			actions: []tfjson.Action{tfjson.ActionCreate},
			want:    "create",
		},
		{
			name:    "delete",
			actions: []tfjson.Action{tfjson.ActionDelete},
			want:    "delete",
		},
		{
			name:    "update",
			actions: []tfjson.Action{tfjson.ActionUpdate},
			want:    "update",
		},
		{
			name:    "recreate",
			actions: []tfjson.Action{tfjson.ActionDelete, tfjson.ActionCreate},
			want:    "recreate",
		},
		{
			name:    "create_and_update_invalid",
			actions: []tfjson.Action{tfjson.ActionCreate, tfjson.ActionUpdate},
			wantErr: true,
		},
		{
			name:    "all_three_invalid",
			actions: []tfjson.Action{tfjson.ActionCreate, tfjson.ActionDelete, tfjson.ActionUpdate},
			wantErr: true,
		},
		{
			name:    "empty",
			actions: []tfjson.Action{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := determineChangeType(tt.actions)
			if (err != nil) != tt.wantErr {
				t.Errorf("determineChangeType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("determineChangeType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildResourceData(t *testing.T) {
	tests := []struct {
		name      string
		resources []*tfjson.ResourceChange
		wantAddrs  []string
		wantLens  int
	}{
		{
			name: "update_with_diff",
			resources: []*tfjson.ResourceChange{
				{
					Address: "aws_instance.web",
					Change: &tfjson.Change{
						Actions: []tfjson.Action{tfjson.ActionUpdate},
						Before: map[string]interface{}{
							"instance_type": "t3.micro",
						},
						After: map[string]interface{}{
							"instance_type": "t3.medium",
						},
					},
				},
			},
			wantAddrs: []string{"aws_instance.web"},
			wantLens:  1,
		},
		{
			name: "sensitive_only_produces_masked_diff",
			resources: []*tfjson.ResourceChange{
				{
					Address: "aws_db_instance.main",
					Change: &tfjson.Change{
						Actions: []tfjson.Action{tfjson.ActionUpdate},
						Before: map[string]interface{}{
							"password": "old",
						},
						After: map[string]interface{}{
							"password": "new",
						},
						BeforeSensitive: map[string]interface{}{
							"password": true,
						},
						AfterSensitive: map[string]interface{}{
							"password": true,
						},
					},
				},
			},
			wantAddrs: []string{"aws_db_instance.main"},
			wantLens:  1,
		},
		{
			name: "no_visible_changes_skipped",
			resources: []*tfjson.ResourceChange{
				{
					Address: "aws_instance.web",
					Change: &tfjson.Change{
						Actions: []tfjson.Action{tfjson.ActionUpdate},
						Before: map[string]interface{}{
							"ami": "ami-123",
						},
						After: map[string]interface{}{
							"ami": "ami-123",
						},
					},
				},
			},
			wantAddrs: nil,
			wantLens:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildResourceData(tt.resources)
			if len(got) != tt.wantLens {
				t.Fatalf("len = %d, want %d", len(got), tt.wantLens)
			}
			for i, addr := range tt.wantAddrs {
				if got[i].Address != addr {
					t.Errorf("got[%d].Address = %q, want %q", i, got[i].Address, addr)
				}
			}
		})
	}
}

func TestProcessChanges(t *testing.T) {
	tests := []struct {
		name              string
		planFile          string
		wantHasChanges    bool
		wantCreated       int
		wantUpdated       int
		wantRecreated     int
		wantDeleted       int
	}{
		{
			name:              "updates_and_deletes",
			planFile:          filepath.Join("testdata", "plan-updates-deletes.json"),
			wantHasChanges:    true,
			wantUpdated:       2,
			wantDeleted:       2,
		},
		{
			name:              "creates_and_recreate",
			planFile:          filepath.Join("testdata", "plan-creates-recreate.json"),
			wantHasChanges:    true,
			wantCreated:       3,
			wantRecreated:     1,
		},
		{
			name:              "deep_nested",
			planFile:          filepath.Join("testdata", "plan-deep-nested.json"),
			wantHasChanges:    true,
			wantUpdated:       2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(tt.planFile)
			if err != nil {
				t.Fatalf("failed to read plan file: %v", err)
			}

			var plan tfjson.Plan
			if err := json.Unmarshal(data, &plan); err != nil {
				t.Fatalf("failed to parse plan: %v", err)
			}
			if err := plan.Validate(); err != nil {
				t.Fatalf("failed to validate plan: %v", err)
			}

			got, err := processChanges(plan.ResourceChanges, tt.planFile)
			if err != nil {
				t.Fatalf("processChanges() error = %v", err)
			}

			if got.HasChanges != tt.wantHasChanges {
				t.Errorf("HasChanges = %v, want %v", got.HasChanges, tt.wantHasChanges)
			}
			if len(got.CreatedResources) != tt.wantCreated {
				t.Errorf("CreatedResources len = %d, want %d", len(got.CreatedResources), tt.wantCreated)
			}
			if len(got.UpdatedResources) != tt.wantUpdated {
				t.Errorf("UpdatedResources len = %d, want %d", len(got.UpdatedResources), tt.wantUpdated)
			}
			if len(got.RecreatedResources) != tt.wantRecreated {
				t.Errorf("RecreatedResources len = %d, want %d", len(got.RecreatedResources), tt.wantRecreated)
			}
			if len(got.DeletedResources) != tt.wantDeleted {
				t.Errorf("DeletedResources len = %d, want %d", len(got.DeletedResources), tt.wantDeleted)
			}

			allResources := append(append(append(got.CreatedResources, got.UpdatedResources...), got.RecreatedResources...), got.DeletedResources...)
			for _, rd := range allResources {
				if rd.Address == "" {
					t.Error("expected non-empty Address")
				}
			}
		})
	}
}

func TestProcessChangesNilResourceChanges(t *testing.T) {
	_, err := processChanges(nil, "test.json")
	if err == nil {
		t.Error("expected error for nil resource changes")
	}
}

func TestExtractPlanName(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "normal_path", path: "/some/path/myplan.json", want: "myplan"},
		{name: "empty", path: "", want: "unknown"},
		{name: "filename_only", path: "plan.json", want: "plan"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractPlanName(tt.path); got != tt.want {
				t.Errorf("extractPlanName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
