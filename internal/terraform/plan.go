package terraform

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	tfjson "github.com/hashicorp/terraform-json"

	"gitlab-terraform-mr-commenter/internal/constants"
)

// ActionType represents the type of resource change in terraform
type ActionType string

const (
	ActionCreate   ActionType = "create"
	ActionUpdate   ActionType = "update"
	ActionRecreate ActionType = "recreate"
	ActionDelete   ActionType = "delete"
)

// DiffAction represents the type of attribute diff
type DiffAction string

const (
	DiffAdded   DiffAction = "added"
	DiffRemoved DiffAction = "removed"
	DiffChanged DiffAction = "changed"
)

type DiffLine struct {
	Key    string
	Before interface{}
	After  interface{}
	Type   DiffAction
}

type DiffGenerator struct{}

func (dg *DiffGenerator) createDiffLine(key string, before, after interface{}, diffType DiffAction) DiffLine {
	return DiffLine{
		Key:    key,
		Before: before,
		After:  after,
		Type:   diffType,
	}
}

func (dg *DiffGenerator) isValueSensitive(resource *tfjson.ResourceChange, fieldName string) bool {
	for _, sensitive := range []interface{}{resource.Change.AfterSensitive, resource.Change.BeforeSensitive} {
		if sensitiveMap, ok := sensitive.(map[string]interface{}); ok {
			if isSensitive, exists := sensitiveMap[fieldName]; exists {
				return dg.isAnySensitive(isSensitive)
			}
		}
	}
	return false
}

func (dg *DiffGenerator) isAnySensitive(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case []interface{}:
		for _, item := range v {
			if dg.isAnySensitive(item) {
				return true
			}
		}
	case map[string]interface{}:
		for _, item := range v {
			if dg.isAnySensitive(item) {
				return true
			}
		}
	}
	return false
}

func (dg *DiffGenerator) GenerateDiff(resource *tfjson.ResourceChange, beforeMap, afterMap map[string]interface{}) []DiffLine {
	keys := dg.collectUniqueKeys(beforeMap, afterMap)
	diffs := make([]DiffLine, 0, len(keys))

	for _, key := range keys {
		beforeVal, beforeExists := beforeMap[key]
		afterVal, afterExists := afterMap[key]

		if resource != nil && dg.isValueSensitive(resource, key) {
			beforeVal = "[SENSITIVE]"
			afterVal = "[SENSITIVE]"
		}

		switch {
		case !beforeExists && afterExists:
			diffs = append(diffs, dg.createDiffLine(key, nil, afterVal, DiffAdded))
		case beforeExists && !afterExists:
			diffs = append(diffs, dg.createDiffLine(key, beforeVal, nil, DiffRemoved))
		case !reflect.DeepEqual(beforeVal, afterVal):
			diffs = append(diffs, dg.createDiffLine(key, beforeVal, afterVal, DiffChanged))
		}
	}

	return diffs
}

func (dg *DiffGenerator) collectUniqueKeys(beforeMap, afterMap map[string]interface{}) []string {
	keySet := make(map[string]struct{}, len(beforeMap)+len(afterMap))
	for key := range beforeMap {
		keySet[key] = struct{}{}
	}
	for key := range afterMap {
		keySet[key] = struct{}{}
	}

	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

type ResourceData struct {
	Address    string
	ChangeType ActionType
	Diffs      []DiffLine
}

func (rd *ResourceData) FormattedDiffs() []string {
	formatted := make([]string, 0, len(rd.Diffs))
	for _, diff := range rd.Diffs {
		switch diff.Type {
		case DiffAdded:
			formatted = append(formatted, fmt.Sprintf("+%s: %s", diff.Key, formatValue(diff.After)))
		case DiffRemoved:
			formatted = append(formatted, fmt.Sprintf("-%s: %s", diff.Key, formatValue(diff.Before)))
		case DiffChanged:
			formatted = append(formatted,
				fmt.Sprintf("-%s: %s", diff.Key, formatValue(diff.Before)),
				fmt.Sprintf("+%s: %s", diff.Key, formatValue(diff.After)),
			)
		}
	}
	return formatted
}

type PlanIdentifier struct {
	Name     string
	FilePath string
	Index    int
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
	Identifier *PlanIdentifier
	Data       *PlanData
}

type Processor struct{}

func (p *Processor) LoadPlan(filename string) (*tfjson.Plan, error) {
	return loadAndValidatePlan(filename)
}

func (p *Processor) ProcessPlan(plan *tfjson.Plan) (*PlanData, error) {
	if plan.ResourceChanges == nil {
		return nil, fmt.Errorf(constants.ErrTerraformPlanEmpty, "unknown")
	}
	return buildPlanData(categorizeChanges(plan.ResourceChanges)), nil
}

func (p *Processor) ProcessMultiplePlans(planFiles []string) (*MultiPlanData, error) {
	if len(planFiles) == 0 {
		return nil, fmt.Errorf("no plan files provided")
	}

	multiPlanData := &MultiPlanData{
		Plans: make([]*PlanWithIdentifier, len(planFiles)),
	}

	for i, planFile := range planFiles {
		plan, err := p.LoadPlan(planFile)
		if err != nil {
			return nil, fmt.Errorf("error loading plan file %s: %w", planFile, err)
		}

		planData, err := p.ProcessPlan(plan)
		if err != nil {
			return nil, fmt.Errorf("error processing plan file %s: %w", planFile, err)
		}

		identifier := &PlanIdentifier{
			Name:     extractPlanName(planFile),
			FilePath: planFile,
			Index:    i,
		}

		multiPlanData.Plans[i] = &PlanWithIdentifier{
			Identifier: identifier,
			Data:       planData,
		}

		if planData.HasChanges {
			multiPlanData.HasChanges = true
		}
	}

	return multiPlanData, nil
}

type ChangeCategories struct {
	Created   []*tfjson.ResourceChange
	Updated   []*tfjson.ResourceChange
	Recreated []*tfjson.ResourceChange
	Deleted   []*tfjson.ResourceChange
}

func categorizeChanges(resourceChanges []*tfjson.ResourceChange) *ChangeCategories {
	categories := &ChangeCategories{}

	for _, change := range resourceChanges {
		if len(change.Change.Actions) == 0 {
			continue
		}

		changeType := determineChangeType(change.Change.Actions)
		switch changeType {
		case ActionCreate:
			categories.Created = append(categories.Created, change)
		case ActionUpdate:
			categories.Updated = append(categories.Updated, change)
		case ActionRecreate:
			categories.Recreated = append(categories.Recreated, change)
		case ActionDelete:
			categories.Deleted = append(categories.Deleted, change)
		}
	}

	return categories
}

func determineChangeType(actions []tfjson.Action) ActionType {
	var create, delete, update bool

	for _, action := range actions {
		switch action {
		case tfjson.ActionCreate:
			create = true
		case tfjson.ActionDelete:
			delete = true
		case tfjson.ActionUpdate:
			update = true
		}
	}

	switch {
	case create && !delete && !update:
		return ActionCreate
	case !create && delete && !update:
		return ActionDelete
	case create && delete && !update:
		return ActionRecreate
	case !create && !delete && update:
		return ActionUpdate
	default:
		return ActionUpdate
	}
}

func buildPlanData(changes *ChangeCategories) *PlanData {
	sortResourceChanges := func(changes []*tfjson.ResourceChange) {
		sort.Slice(changes, func(i, j int) bool {
			return changes[i].Address < changes[j].Address
		})
	}

	sortResourceChanges(changes.Created)
	sortResourceChanges(changes.Updated)
	sortResourceChanges(changes.Recreated)
	sortResourceChanges(changes.Deleted)

	planData := &PlanData{
		CreatedResources:   buildResourceData(changes.Created, ActionCreate),
		UpdatedResources:   buildResourceData(changes.Updated, ActionUpdate),
		RecreatedResources: buildResourceData(changes.Recreated, ActionRecreate),
		DeletedResources:   buildResourceData(changes.Deleted, ActionDelete),
		HasChanges:         len(changes.Created) > 0 || len(changes.Updated) > 0 || len(changes.Recreated) > 0 || len(changes.Deleted) > 0,
	}

	return planData
}

func buildResourceData(resources []*tfjson.ResourceChange, changeType ActionType) []*ResourceData {
	data := make([]*ResourceData, len(resources))
	diffGen := DiffGenerator{}

	for index, resource := range resources {
		beforeMap, _ := resource.Change.Before.(map[string]interface{})
		afterMap, _ := resource.Change.After.(map[string]interface{})
		if beforeMap == nil {
			beforeMap = make(map[string]interface{})
		}
		if afterMap == nil {
			afterMap = make(map[string]interface{})
		}
		data[index] = &ResourceData{
			Address:    resource.Address,
			ChangeType: changeType,
			Diffs:      diffGen.GenerateDiff(resource, beforeMap, afterMap),
		}
	}

	return data
}

func loadAndValidatePlan(filename string) (*tfjson.Plan, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf(constants.ErrTerraformPlanFile, filename, err)
	}

	var plan tfjson.Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf(constants.ErrTerraformPlanParse, filename, err)
	}

	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf(constants.ErrTerraformPlanInvalid, filename, err)
	}

	return &plan, nil
}

func formatValue(value interface{}) string {
	if value == nil {
		return "null"
	}

	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(jsonBytes)
}

func extractPlanName(filePath string) string {
	if filePath == "" {
		return "unknown"
	}

	parts := strings.Split(filePath, "/")
	if len(parts) == 0 {
		return filePath
	}

	fileName := parts[len(parts)-1]

	if strings.HasSuffix(fileName, ".json") {
		return fileName[:len(fileName)-5]
	}

	return fileName
}
