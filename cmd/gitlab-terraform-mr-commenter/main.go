package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"gitlab-terraform-mr-commenter/internal/config"
	"gitlab-terraform-mr-commenter/internal/constants"
	"gitlab-terraform-mr-commenter/internal/formatter"
	"gitlab-terraform-mr-commenter/internal/gitlab"
	"gitlab-terraform-mr-commenter/internal/output"
	"gitlab-terraform-mr-commenter/internal/terraform"
	"gitlab-terraform-mr-commenter/internal/types"
)

const noChangesMessage = "No changes detected."

type GitLabCommenter interface {
	ValidateAccess(ctx context.Context) error
	FindExistingPlanNote(ctx context.Context) (*types.MRNote, error)
	ShouldUpdateNote(existingBody, newBody string) bool
	UpdateNote(ctx context.Context, noteID int64, body string) error
	CreateNote(ctx context.Context, body string) error
}

func withMarker(body string) string {
	return constants.NoteMarker + "\n" + body
}

func main() {
	var outputFile string

	flag.StringVar(&outputFile, "output", "", "Write output to file (use '-' for stdout)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <terraform-plan.json> [<terraform-plan2.json> ...]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment Variables:\n")
		fmt.Fprintf(os.Stderr, "  GITLAB_TOKEN      GitLab personal access token (required)\n")
		fmt.Fprintf(os.Stderr, "  GITLAB_URL        GitLab instance URL (default: https://gitlab.com)\n")
		fmt.Fprintf(os.Stderr, "  GITLAB_PROJECT_ID GitLab project ID (required)\n")
		fmt.Fprintf(os.Stderr, "  GITLAB_MR_ID      GitLab merge request ID (required)\n")
	}

	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}
	planFiles := args

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, planFiles, outputFile); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, planFiles []string, outputFile string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error loading configuration: %w", err)
	}

	gitlabClient, err := gitlab.New(cfg)
	if err != nil {
		return fmt.Errorf("error creating GitLab client: %w", err)
	}

	return runWithClients(ctx, planFiles, outputFile, gitlabClient)
}

func runWithClients(ctx context.Context, planFiles []string, outputFile string, gitlabClient GitLabCommenter) error {
	commentBody, err := loadAndProcessPlans(planFiles)
	if err != nil {
		return err
	}

	if outputFile != "" {
		if err := output.Write(commentBody, outputFile); err != nil {
			return fmt.Errorf("error writing output: %w", err)
		}
		output.PrintSuccess(outputFile)
		return nil
	}

	return handleGitLabComment(ctx, commentBody, gitlabClient)
}

func loadAndProcessPlans(planFiles []string) (string, error) {
	multiPlanData, err := terraform.ProcessMultiplePlans(planFiles)
	if err != nil {
		return "", fmt.Errorf("error processing terraform plans: %w", err)
	}

	var commentBody string
	if multiPlanData.HasChanges {
		commentBody, err = formatter.FormatPlan(multiPlanData)
		if err != nil {
			return "", fmt.Errorf("error formatting plans: %w", err)
		}
	} else {
		commentBody = noChangesMessage
	}

	return commentBody, nil
}

func handleGitLabComment(ctx context.Context, commentBody string, gitlabClient GitLabCommenter) error {
	if err := gitlabClient.ValidateAccess(ctx); err != nil {
		return fmt.Errorf("error validating GitLab access: %w", err)
	}

	slog.Info("comment body ready", "length", len(commentBody))
	existingNote, err := gitlabClient.FindExistingPlanNote(ctx)
	if err != nil {
		return fmt.Errorf("error finding existing plan note: %w", err)
	}
	return updateOrCreateNote(ctx, gitlabClient, existingNote, commentBody)
}

func updateOrCreateNote(ctx context.Context, gitlabClient GitLabCommenter, existingNote *types.MRNote, commentBody string) error {
	markedBody := withMarker(commentBody)
	if existingNote.Exists {
		slog.Info("found existing note", "note_id", existingNote.ID)
		strippedExisting := strings.TrimPrefix(existingNote.Body, constants.NoteMarker+"\n")
		if !gitlabClient.ShouldUpdateNote(strippedExisting, commentBody) {
			slog.Info("note up to date, skipping update")
			return nil
		}
		if err := gitlabClient.UpdateNote(ctx, existingNote.ID, markedBody); err != nil {
			return fmt.Errorf("error updating note: %w", err)
		}
		slog.Info("updated existing note", "note_id", existingNote.ID)
		return nil
	}
	slog.Info("creating new note")
	if err := gitlabClient.CreateNote(ctx, markedBody); err != nil {
		return fmt.Errorf("error creating note: %w", err)
	}
	slog.Info("created new note")
	return nil
}
