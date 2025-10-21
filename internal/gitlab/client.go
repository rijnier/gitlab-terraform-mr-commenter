package gitlab

import (
	"fmt"
	"strconv"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab-terraform-mr-commenter/internal/config"
	"gitlab-terraform-mr-commenter/internal/constants"
)

type MRNote struct {
	ID     string
	Body   string
	Exists bool
}

type Client struct {
	client *gitlab.Client
	config *config.Config
	mrID   int
}

func New(cfg *config.Config) (*Client, error) {
	client, err := gitlab.NewClient(cfg.GitlabToken, gitlab.WithBaseURL(cfg.GitlabURL))
	if err != nil {
		return nil, fmt.Errorf("error creating GitLab client: %w", err)
	}

	mrID, err := strconv.Atoi(cfg.MergeRequestID)
	if err != nil {
		return nil, fmt.Errorf("error converting MergeRequestID: %w", err)
	}

	return &Client{
		client: client,
		config: cfg,
		mrID:   mrID,
	}, nil
}

func (c *Client) ValidateAccess() error {
	user, _, err := c.client.Users.CurrentUser()
	if err != nil {
		return c.wrapGitLabError(constants.ErrGitlabAuth, err)
	}

	project, _, err := c.client.Projects.GetProject(c.config.ProjectID, nil)
	if err != nil {
		return c.wrapGitLabError(constants.ErrGitlabProject, c.config.ProjectID, err)
	}

	mr, _, err := c.client.MergeRequests.GetMergeRequest(c.config.ProjectID, c.mrID, nil)
	if err != nil {
		return c.wrapGitLabError(constants.ErrGitlabMR, c.config.MergeRequestID, err)
	}

	fmt.Printf("Authenticated as user: %s\n", user.Username)
	fmt.Printf("Project: %s\n", project.NameWithNamespace)
	fmt.Printf("Merge Request: %s\n", mr.Title)

	return nil
}

func (c *Client) CreateNote(title, body string) error {
	note := &gitlab.CreateMergeRequestNoteOptions{
		Body:     &body,
		Internal: gitlab.Ptr(true),
	}

	_, _, err := c.client.Notes.CreateMergeRequestNote(c.config.ProjectID, c.mrID, note)
	if err != nil {
		return c.wrapGitLabError(constants.ErrGitlabCreateNote, err)
	}

	return nil
}

func (c *Client) FindExistingPlanNote() (*MRNote, error) {
	notes, _, err := c.client.Notes.ListMergeRequestNotes(c.config.ProjectID, c.mrID, nil)
	if err != nil {
		return nil, c.wrapGitLabError(constants.ErrGitlabListNotes, err)
	}

	for _, note := range notes {
		if !note.Internal {
			continue
		}
		
		hasPlanSummary := len(note.Body) >= len(constants.TerraformPlansSummary) && 
			note.Body[:len(constants.TerraformPlansSummary)] == constants.TerraformPlansSummary
		
		if hasPlanSummary {
			return &MRNote{
				ID:     fmt.Sprintf("%d", note.ID),
				Body:   note.Body,
				Exists: true,
			}, nil
		}
	}

	return &MRNote{Exists: false}, nil
}

func (c *Client) UpdateNote(noteID, body string) error {
	note := &gitlab.UpdateMergeRequestNoteOptions{
		Body: &body,
	}

	noteIDInt, _ := strconv.Atoi(noteID)
	_, _, err := c.client.Notes.UpdateMergeRequestNote(c.config.ProjectID, c.mrID, noteIDInt, note)
	if err != nil {
		return c.wrapGitLabError(constants.ErrGitlabUpdateNote, noteID, err)
	}

	return nil
}



func (c *Client) ShouldUpdateNote(existingBody, newBody string) bool {
	normalizeContent := func(s string) string {
		return strings.Join(strings.Fields(s), " ")
	}
	return !strings.EqualFold(normalizeContent(existingBody), normalizeContent(newBody))
}

func (c *Client) wrapGitLabError(template string, args ...interface{}) error {
	return fmt.Errorf(template, args...)
}
