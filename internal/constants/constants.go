package constants

const (
	TerraformPlansSummary = "## Terraform Plan Summary"

	NoChangesMessage               = "No changes detected."
	NoChangesAcrossAllPlansMessage = "No changes detected across all plans."

	ErrTerraformPlanFile    = "failed to open terraform plan file %s: %w"
	ErrTerraformPlanParse   = "failed to parse terraform plan JSON from %s: %w"
	ErrTerraformPlanInvalid = "invalid terraform plan format in %s: %w"
	ErrTerraformPlanEmpty   = "terraform plan %s appears to be incomplete or in wrong format"

	ErrGitlabAuth                 = "failed to authenticate with GitLab: %w"
	ErrGitlabProject              = "failed to access project %s: %w"
	ErrGitlabMR                   = "failed to access merge request %s: %w"
	ErrGitlabListNotes            = "failed to list MR notes: %w"
	ErrGitlabCreateNote           = "failed to create MR note: %w"
	ErrGitlabUpdateNote           = "failed to update MR note %s: %w"
	ErrGitlabListDiscussions      = "failed to list MR discussions: %w"
	ErrGitlabCreateDiscussion     = "failed to create MR discussion: %w"
	ErrGitlabGetDiscussion        = "failed to get MR discussion %s: %w"
	ErrGitlabUpdateDiscussionNote = "failed to update discussion note %s/%s: %w"

	ErrOutputFileCreate = "failed to create output file %s: %w"
	ErrOutputFileWrite  = "failed to write to output file %s: %w"

	ErrConfigLoad = "failed to load configuration: %w"

	StdoutFileIndicator = "-"
	SuccessMessage      = "Markdown output written to %s"

	EnvGitlabToken     = "GITLAB_TOKEN"
	EnvGitlabURL       = "GITLAB_URL"
	EnvGitlabProjectID = "GITLAB_PROJECT_ID"
	EnvGitlabMRID      = "GITLAB_MR_ID"

	LogCommentBodyLength   = "Comment body length: %d characters\n"
	LogFoundExistingNote   = "Found existing internal note %s\n"
	LogNoteUpToDate        = "Internal note already exists and content is up to date. No action needed.\n"
	LogUpdatedExistingNote = "Updated existing internal note %s\n"
	LogCreatingNewNote     = "No existing internal note found, creating new one\n"
	LogCreatedNewNote      = "Created new internal note with full content\n"
)
