package terraform

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var update = flag.Bool("update", false, "update golden files")

func TestGenerateDiff(t *testing.T) {
	tests := []struct {
		name            string
		before          map[string]interface{}
		after           map[string]interface{}
		beforeSensitive interface{}
		afterSensitive  interface{}
		wantChanges     bool
	}{
		{
			name: "update",
			before: map[string]interface{}{
				"instance_type": "t3.micro",
				"ami":           "ami-12345678",
				"tags": map[string]interface{}{
					"Environment": "staging",
				},
			},
			after: map[string]interface{}{
				"instance_type": "t3.medium",
				"ami":           "ami-12345678",
				"tags": map[string]interface{}{
					"Environment": "production",
				},
			},
			wantChanges: true,
		},
		{
			name:   "create",
			before: map[string]interface{}{},
			after: map[string]interface{}{
				"instance_type": "t3.medium",
				"ami":           "ami-12345678",
			},
			wantChanges: true,
		},
		{
			name: "delete",
			before: map[string]interface{}{
				"instance_type": "t3.micro",
			},
			after:       map[string]interface{}{},
			wantChanges: true,
		},
		{
			name: "no_changes",
			before: map[string]interface{}{
				"ami": "ami-12345678",
			},
			after: map[string]interface{}{
				"ami": "ami-12345678",
			},
			wantChanges: false,
		},
		{
			name: "array_change",
			before: map[string]interface{}{
				"cidr_blocks": []interface{}{"10.0.0.0/8"},
			},
			after: map[string]interface{}{
				"cidr_blocks": []interface{}{"10.0.0.0/8", "172.16.0.0/12"},
			},
			wantChanges: true,
		},
		{
			name: "sensitive_only",
			before: map[string]interface{}{
				"password":    "secret123",
				"instance_id": "i-12345",
			},
			after: map[string]interface{}{
				"password":    "newsecret456",
				"instance_id": "i-12345",
			},
			beforeSensitive: map[string]interface{}{"password": true},
			afterSensitive:  map[string]interface{}{"password": true},
			wantChanges:     true,
		},
		{
			name: "nested_sensitive",
			before: map[string]interface{}{
				"config": map[string]interface{}{
					"password": "secret123",
					"username": "admin",
				},
			},
			after: map[string]interface{}{
				"config": map[string]interface{}{
					"password": "newsecret456",
					"username": "superadmin",
				},
			},
			beforeSensitive: map[string]interface{}{
				"config": map[string]interface{}{
					"password": true,
				},
			},
			afterSensitive: map[string]interface{}{
				"config": map[string]interface{}{
					"password": true,
				},
			},
			wantChanges: true,
		},
		{
			name: "sensitive_same_values",
			before: map[string]interface{}{
				"password": "secret123",
			},
			after: map[string]interface{}{
				"password": "newsecret456",
			},
			beforeSensitive: map[string]interface{}{"password": true},
			afterSensitive:  map[string]interface{}{"password": true},
			wantChanges:     true,
		},
		{
			name: "same_values_no_sensitivity",
			before: map[string]interface{}{
				"password": "secret123",
			},
			after: map[string]interface{}{
				"password": "secret123",
			},
			wantChanges: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, hasChanges := generateDiff(tt.before, tt.after, tt.beforeSensitive, tt.afterSensitive)

			if hasChanges != tt.wantChanges {
				t.Errorf("hasChanges = %v, want %v", hasChanges, tt.wantChanges)
			}

			if !tt.wantChanges {
				if got != "" {
					t.Errorf("expected empty diff for no changes, got:\n%s", got)
				}
				return
			}

			goldenPath := filepath.Join("testdata", tt.name+".golden")
			if *update {
				if err := os.WriteFile(goldenPath, []byte(got), 0644); err != nil {
					t.Fatalf("failed to write golden file: %v", err)
				}
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("failed to read golden file: %v", err)
			}

			if diff := cmp.Diff(string(want), got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMaskSensitive(t *testing.T) {
	t.Run("fields_nil_sensitive", func(t *testing.T) {
		got := maskSensitiveFields(map[string]interface{}{"key": "value"}, nil)
		want := map[string]interface{}{"key": "value"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("fields_false_sensitive", func(t *testing.T) {
		got := maskSensitiveFields(map[string]interface{}{"key": "value"}, false)
		want := map[string]interface{}{"key": "value"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("fields_empty_sensitive_map", func(t *testing.T) {
		got := maskSensitiveFields(map[string]interface{}{"key": "value"}, map[string]interface{}{})
		want := map[string]interface{}{"key": "value"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("fields_non_map_sensitive_type", func(t *testing.T) {
		got := maskSensitiveFields(map[string]interface{}{"key": "value"}, "not-a-map")
		want := map[string]interface{}{"key": "value"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("fields_sensitive_key_not_in_data", func(t *testing.T) {
		got := maskSensitiveFields(
			map[string]interface{}{"key": "value"},
			map[string]interface{}{"other": true},
		)
		want := map[string]interface{}{"key": "value"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("fields_bool_true_masks_value", func(t *testing.T) {
		got := maskSensitiveFields(
			map[string]interface{}{"password": "secret", "name": "app"},
			map[string]interface{}{"password": true},
		)
		want := map[string]interface{}{"password": "__sensitive__secret", "name": "app"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("fields_bool_false_keeps_value", func(t *testing.T) {
		got := maskSensitiveFields(
			map[string]interface{}{"password": "secret"},
			map[string]interface{}{"password": false},
		)
		want := map[string]interface{}{"password": "secret"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("fields_nested_map_recursive", func(t *testing.T) {
		got := maskSensitiveFields(
			map[string]interface{}{
				"config": map[string]interface{}{
					"password": "secret",
					"username": "admin",
				},
			},
			map[string]interface{}{
				"config": map[string]interface{}{
					"password": true,
				},
			},
		)
		want := map[string]interface{}{
			"config": map[string]interface{}{
				"password": "__sensitive__secret",
				"username": "admin",
			},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("list_empty_sensitive", func(t *testing.T) {
		got := maskSensitiveList(
			[]interface{}{map[string]interface{}{"key": "a"}},
			[]interface{}{},
		)
		want := []interface{}{map[string]interface{}{"key": "a"}}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("list_shorter_sensitive_trailing_pass_through", func(t *testing.T) {
		got := maskSensitiveList(
			[]interface{}{"a", "b", "c"},
			[]interface{}{true},
		)
		want := []interface{}{"__sensitive__a", "b", "c"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("list_unknown_sensitive_type_passes_through", func(t *testing.T) {
		got := maskSensitiveList(
			[]interface{}{"item"},
			[]interface{}{"not-bool-or-map"},
		)
		want := []interface{}{"item"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("list_bool_true_masks_element", func(t *testing.T) {
		got := maskSensitiveList(
			[]interface{}{"item1", "item2"},
			[]interface{}{true, false},
		)
		want := []interface{}{"__sensitive__item1", "item2"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("list_map_element_recursive", func(t *testing.T) {
		got := maskSensitiveList(
			[]interface{}{
				map[string]interface{}{
					"password": "secret",
					"username": "admin",
				},
			},
			[]interface{}{
				map[string]interface{}{
					"password": true,
				},
			},
		)
		want := []interface{}{
			map[string]interface{}{
				"password": "__sensitive__secret",
				"username": "admin",
			},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})
}
