package terraform

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/zclconf/go-cty/cty"
)

var dmp = diffmatchpatch.New()

const sensitivePrefix = "__sensitive__"

var sensitiveRe = regexp.MustCompile(regexp.QuoteMeta(sensitivePrefix) + `[^\s"]*`)

func maskSensitiveFields(data map[string]interface{}, sensitive interface{}) map[string]interface{} {
	if sensitive == nil || sensitive == false {
		return data
	}

	sensitiveMap, ok := sensitive.(map[string]interface{})
	if !ok {
		return data
	}

	if len(sensitiveMap) == 0 {
		return data
	}

	filtered := make(map[string]interface{})
	for key, val := range data {
		sensVal, sensExists := sensitiveMap[key]
		if !sensExists {
			filtered[key] = val
			continue
		}

		switch s := sensVal.(type) {
		case bool:
			if s {
				filtered[key] = sensitivePrefix + fmt.Sprintf("%v", val)
			} else {
				filtered[key] = val
			}
		case map[string]interface{}:
			if nested, ok := val.(map[string]interface{}); ok {
				filtered[key] = maskSensitiveFields(nested, s)
			} else {
				filtered[key] = val
			}
		case []interface{}:
			if nested, ok := val.([]interface{}); ok {
				filtered[key] = maskSensitiveList(nested, s)
			} else {
				filtered[key] = val
			}
		default:
			filtered[key] = val
		}
	}
	return filtered
}

func maskSensitiveList(data []interface{}, sensitive []interface{}) []interface{} {
	if len(sensitive) == 0 {
		return data
	}

	result := make([]interface{}, 0, len(data))
	for i, item := range data {
		if i >= len(sensitive) {
			result = append(result, item)
			continue
		}

		switch s := sensitive[i].(type) {
		case bool:
			if s {
				result = append(result, sensitivePrefix+fmt.Sprintf("%v", item))
			} else {
				result = append(result, item)
			}
		case map[string]interface{}:
			if nested, ok := item.(map[string]interface{}); ok {
				result = append(result, maskSensitiveFields(nested, s))
			} else {
				result = append(result, item)
			}
		default:
			result = append(result, item)
		}
	}
	return result
}

func generateDiff(before, after map[string]interface{}, beforeSensitive, afterSensitive interface{}) (string, bool) {
	maskedBefore := maskSensitiveFields(before, beforeSensitive)
	maskedAfter := maskSensitiveFields(after, afterSensitive)

	beforeHCL := formatHCL(maskedBefore)
	afterHCL := formatHCL(maskedAfter)

	if beforeHCL == afterHCL {
		return "", false
	}

	srcRunes, dstRunes, lineArray := dmp.DiffLinesToRunes(beforeHCL, afterHCL)
	diffs := dmp.DiffMainRunes(srcRunes, dstRunes, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)
	raw := formatDiffLines(diffs)
	return sensitiveRe.ReplaceAllString(raw, "(sensitive value)"), true
}

func formatDiffLines(diffs []diffmatchpatch.Diff) string {
	var sb strings.Builder
	for _, d := range diffs {
		lines := strings.Split(d.Text, "\n")
		for i, line := range lines {
			if line == "" && i == len(lines)-1 {
				continue
			}
			switch d.Type {
			case diffmatchpatch.DiffEqual:
				sb.WriteString(" " + line + "\n")
			case diffmatchpatch.DiffInsert:
				sb.WriteString("+" + line + "\n")
			case diffmatchpatch.DiffDelete:
				sb.WriteString("-" + line + "\n")
			}
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func formatHCL(attrs map[string]interface{}) string {
	if len(attrs) == 0 {
		return ""
	}

	f := hclwrite.NewEmptyFile()
	keys := slices.Collect(maps.Keys(attrs))
	slices.Sort(keys)
	for _, key := range keys {
		f.Body().SetAttributeValue(key, ctyValueFromInterface(attrs[key]))
	}
	return strings.TrimSpace(string(f.Bytes()))
}

func ctyValueFromInterface(v interface{}) cty.Value {
	switch val := v.(type) {
	case string:
		return cty.StringVal(val)
	case bool:
		return cty.BoolVal(val)
	case float64:
		return cty.NumberFloatVal(val)
	case nil:
		return cty.NullVal(cty.DynamicPseudoType)
	case map[string]interface{}:
		m := make(map[string]cty.Value, len(val))
		for k, v := range val {
			m[k] = ctyValueFromInterface(v)
		}
		return cty.ObjectVal(m)
	case []interface{}:
		s := make([]cty.Value, len(val))
		for i, v := range val {
			s[i] = ctyValueFromInterface(v)
		}
		return cty.TupleVal(s)
	default:
		return cty.StringVal(fmt.Sprintf("%v", val))
	}
}
