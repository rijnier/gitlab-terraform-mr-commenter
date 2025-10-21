package formatter

import (
	"fmt"
	"strings"
	"text/template"

	"gitlab-terraform-mr-commenter/internal/terraform"
	"gitlab-terraform-mr-commenter/templates"
)

func loadTemplate(funcMap template.FuncMap) (*template.Template, error) {
	tmpl := template.New("plan.md.tmpl")
	if funcMap != nil {
		tmpl = tmpl.Funcs(funcMap)
	}

	tmpl, err := tmpl.Parse(templates.PlanTemplateContent)
	if err != nil {
		return nil, fmt.Errorf("error parsing embedded template: %w", err)
	}
	return tmpl, nil
}

func FormatPlan(multiPlanData *terraform.MultiPlanData) (string, error) {
	funcMap := template.FuncMap{
		"sub": func(a, b int) int { return a - b },
	}

	tmpl, err := loadTemplate(funcMap)
	if err != nil {
		return "", fmt.Errorf("error loading template: %w", err)
	}

	var builder strings.Builder
	if err := tmpl.Execute(&builder, multiPlanData); err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	return builder.String(), nil
}
