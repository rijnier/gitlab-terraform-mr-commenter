package gitlab

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab-terraform-mr-commenter/internal/config"
	"gitlab-terraform-mr-commenter/internal/constants"
	"gitlab-terraform-mr-commenter/internal/types"
)

type Client struct {
	client    *gitlab.Client
	projectID string
	mrID      int64
}

func New(cfg *config.Config) (*Client, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	client, err := gitlab.NewClient(cfg.GitlabToken, gitlab.WithBaseURL(cfg.GitlabURL), gitlab.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("error creating GitLab client: %w", err)
	}

	return &Client{
		client:    client,
		projectID: cfg.ProjectID,
		mrID:      cfg.MergeRequestID,
	}, nil
}

func (c *Client) ValidateAccess(ctx context.Context) error {
	user, _, err := c.client.Users.CurrentUser(gitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to authenticate with GitLab: %w", err)
	}

	project, _, err := c.client.Projects.GetProject(c.projectID, nil, gitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to access project %s: %w", c.projectID, err)
	}

	mr, _, err := c.client.MergeRequests.GetMergeRequest(c.projectID, c.mrID, nil, gitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to access merge request %d: %w", c.mrID, err)
	}

	slog.Info("authenticated", "user", user.Username, "project", project.NameWithNamespace, "merge_request", mr.Title)

	return nil
}

func (c *Client) CreateNote(ctx context.Context, body string) error {
	note := &gitlab.CreateMergeRequestNoteOptions{
		Body:     &body,
		Internal: gitlab.Ptr(true),
	}

	_, _, err := c.client.Notes.CreateMergeRequestNote(c.projectID, c.mrID, note, gitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to create MR note: %w", err)
	}

	return nil
}

func (c *Client) FindExistingPlanNote(ctx context.Context) (*types.MRNote, error) {
	notes, _, err := c.client.Notes.ListMergeRequestNotes(c.projectID, c.mrID, nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to list MR notes: %w", err)
	}

	for _, note := range notes {
		if !note.Internal {
			continue
		}

		if strings.Contains(note.Body, constants.NoteMarker) {
			return &types.MRNote{
				ID:     note.ID,
				Body:   note.Body,
				Exists: true,
			}, nil
		}
	}

	return &types.MRNote{Exists: false}, nil
}

func (c *Client) UpdateNote(ctx context.Context, noteID int64, body string) error {
	note := &gitlab.UpdateMergeRequestNoteOptions{
		Body: &body,
	}

	_, _, err := c.client.Notes.UpdateMergeRequestNote(c.projectID, c.mrID, noteID, note, gitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to update MR note %d: %w", noteID, err)
	}

	return nil
}

func (c *Client) ShouldUpdateNote(existingBody, newBody string) bool {
	normalizeContent := func(s string) string {
		return strings.Join(strings.Fields(s), " ")
	}
	return !strings.EqualFold(normalizeContent(existingBody), normalizeContent(newBody))
}
