package terraform

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"

	tfjson "github.com/hashicorp/terraform-json"
)

type ResourceData struct {
	Address string
	Diff    string
}

type PlanData struct {
	HasChanges         bool
	CreatedResources   []*ResourceData
	UpdatedResources   []*ResourceData
	RecreatedResources []*ResourceData
	DeletedResources   []*ResourceData
}

type MultiPlanData struct {
	HasChanges bool
	Plans      []*PlanWithIdentifier
}

type PlanWithIdentifier struct {
	Name string
	Data *PlanData
}

func ProcessMultiplePlans(planFiles []string) (*MultiPlanData, error) {
	if len(planFiles) == 0 {
		return nil, fmt.Errorf("no plan files provided")
	}

	multiPlanData := &MultiPlanData{
		Plans: make([]*PlanWithIdentifier, len(planFiles)),
	}

	for i, planFile := range planFiles {
		plan, err := loadAndValidatePlan(planFile)
		if err != nil {
			return nil, err
		}

		planData, err := processChanges(plan.ResourceChanges, planFile)
		if err != nil {
			return nil, err
		}

		multiPlanData.Plans[i] = &PlanWithIdentifier{
			Name: extractPlanName(planFile),
			Data: planData,
		}

		multiPlanData.HasChanges = multiPlanData.HasChanges || planData.HasChanges
	}

	return multiPlanData, nil
}

func processChanges(resourceChanges []*tfjson.ResourceChange, planFile string) (*PlanData, error) {
	if resourceChanges == nil {
		return nil, fmt.Errorf("terraform plan %s appears to be incomplete or in wrong format", planFile)
	}

	var created, updated, recreated, deleted []*tfjson.ResourceChange

	for _, change := range resourceChanges {
		if len(change.Change.Actions) == 0 {
			continue
		}

		changeType, err := determineChangeType(change.Change.Actions)
		if err != nil {
			return nil, fmt.Errorf("resource %s: %w", change.Address, err)
		}

		switch changeType {
		case "create":
			created = append(created, change)
		case "update":
			updated = append(updated, change)
		case "recreate":
			recreated = append(recreated, change)
		case "delete":
			deleted = append(deleted, change)
		}
	}

	sortByAddress(created)
	sortByAddress(updated)
	sortByAddress(recreated)
	sortByAddress(deleted)

	return &PlanData{
		CreatedResources:   buildResourceData(created),
		UpdatedResources:   buildResourceData(updated),
		RecreatedResources: buildResourceData(recreated),
		DeletedResources:   buildResourceData(deleted),
		HasChanges:         len(created) > 0 || len(updated) > 0 || len(recreated) > 0 || len(deleted) > 0,
	}, nil
}

func determineChangeType(actions []tfjson.Action) (string, error) {
	seen := make(map[tfjson.Action]bool, len(actions))
	for _, a := range actions {
		seen[a] = true
	}

	switch {
	case seen[tfjson.ActionCreate] && !seen[tfjson.ActionDelete] && !seen[tfjson.ActionUpdate]:
		return "create", nil
	case !seen[tfjson.ActionCreate] && seen[tfjson.ActionDelete] && !seen[tfjson.ActionUpdate]:
		return "delete", nil
	case seen[tfjson.ActionCreate] && seen[tfjson.ActionDelete] && !seen[tfjson.ActionUpdate]:
		return "recreate", nil
	case !seen[tfjson.ActionCreate] && !seen[tfjson.ActionDelete] && seen[tfjson.ActionUpdate]:
		return "update", nil
	default:
		return "", fmt.Errorf("unexpected action combination: %v", actions)
	}
}

func sortByAddress(changes []*tfjson.ResourceChange) {
	slices.SortFunc(changes, func(a, b *tfjson.ResourceChange) int {
		return strings.Compare(a.Address, b.Address)
	})
}

func buildResourceData(resources []*tfjson.ResourceChange) []*ResourceData {
	result := make([]*ResourceData, 0, len(resources))

	for _, resource := range resources {
		beforeMap, _ := resource.Change.Before.(map[string]interface{})
		afterMap, _ := resource.Change.After.(map[string]interface{})
		if beforeMap == nil {
			beforeMap = make(map[string]interface{})
		}
		if afterMap == nil {
			afterMap = make(map[string]interface{})
		}

		diff, hasChanges := generateDiff(beforeMap, afterMap, resource.Change.BeforeSensitive, resource.Change.AfterSensitive)
		if !hasChanges {
			continue
		}

		result = append(result, &ResourceData{
			Address: resource.Address,
			Diff:    diff,
		})
	}

	return result
}

func loadAndValidatePlan(filename string) (*tfjson.Plan, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open terraform plan file %s: %w", filename, err)
	}

	var plan tfjson.Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse terraform plan JSON from %s: %w", filename, err)
	}

	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf("invalid terraform plan format in %s: %w", filename, err)
	}

	return &plan, nil
}

func extractPlanName(filePath string) string {
	if filePath == "" {
		return "unknown"
	}
	return strings.TrimSuffix(path.Base(filePath), ".json")
}
