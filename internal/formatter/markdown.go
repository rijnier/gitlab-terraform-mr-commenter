package formatter

import (
	"fmt"
	"strings"
	"text/template"

	"gitlab-terraform-mr-commenter/internal/terraform"
	"gitlab-terraform-mr-commenter/templates"
)

var planTmpl = template.Must(template.New("plan.md.tmpl").Funcs(template.FuncMap{
	"sub": func(a, b int) int { return a - b },
}).Parse(templates.PlanTemplateContent))

func FormatPlan(multiPlanData *terraform.MultiPlanData) (string, error) {
	var builder strings.Builder
	if err := planTmpl.Execute(&builder, multiPlanData); err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	return builder.String(), nil
}
