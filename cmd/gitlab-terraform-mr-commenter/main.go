package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"gitlab-terraform-mr-commenter/internal/config"
	"gitlab-terraform-mr-commenter/internal/constants"
	"gitlab-terraform-mr-commenter/internal/formatter"
	"gitlab-terraform-mr-commenter/internal/gitlab"
	"gitlab-terraform-mr-commenter/internal/output"
	"gitlab-terraform-mr-commenter/internal/terraform"
)

func updateExistingNote(gitlabClient *gitlab.Client, noteID, commentBody string) error {
	if err := gitlabClient.UpdateNote(noteID, commentBody); err != nil {
		return fmt.Errorf("error updating internal note: %w", err)
	}
	fmt.Printf(constants.LogUpdatedExistingNote, noteID)
	return nil
}

func createNewNote(gitlabClient *gitlab.Client, commentBody string) error {
	if err := gitlabClient.CreateNote("", commentBody); err != nil {
		return fmt.Errorf("error creating internal note: %w", err)
	}
	fmt.Printf(constants.LogCreatedNewNote)
	return nil
}

func main() {
	var outputFile string

	flag.StringVar(&outputFile, "output", "", "Write output to file (use '-' for stdout)")
	flag.StringVar(&outputFile, "o", "", "Write output to file (use '-' for stdout) (shorthand)")

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

	if err := run(planFiles, outputFile); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run(planFiles []string, outputFile string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("error loading configuration: %w", err)
	}

	gitlabClient, err := gitlab.New(cfg)
	if err != nil {
		return fmt.Errorf("error creating GitLab client: %w", err)
	}

	terraformProcessor := &terraform.Processor{}

	return runWithClients(planFiles, outputFile, gitlabClient, terraformProcessor)
}

func runWithClients(planFiles []string, outputFile string, gitlabClient *gitlab.Client, terraformProcessor *terraform.Processor) error {
	commentBody, err := loadAndProcessPlans(planFiles, terraformProcessor)
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

	return handleGitLabComment(commentBody, gitlabClient)
}

func loadAndProcessPlans(planFiles []string, terraformProcessor *terraform.Processor) (string, error) {
	multiPlanData, err := terraformProcessor.ProcessMultiplePlans(planFiles)
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
		commentBody = constants.NoChangesMessage
	}

	return commentBody, nil
}

func handleGitLabComment(commentBody string, gitlabClient *gitlab.Client) error {
	if err := gitlabClient.ValidateAccess(); err != nil {
		return fmt.Errorf("error validating GitLab access: %w", err)
	}

	fmt.Printf(constants.LogCommentBodyLength, len(commentBody))
	existingNote, err := gitlabClient.FindExistingPlanNote()
	if err != nil {
		return fmt.Errorf("error finding existing plan note: %w", err)
	}
	return updateOrCreateNote(gitlabClient, existingNote, commentBody)
}

func updateOrCreateNote(gitlabClient *gitlab.Client, existingNote *gitlab.MRNote, commentBody string) error {
	if existingNote.Exists {
		fmt.Printf(constants.LogFoundExistingNote, existingNote.ID)
		if !gitlabClient.ShouldUpdateNote(existingNote.Body, commentBody) {
			fmt.Printf(constants.LogNoteUpToDate)
			return nil
		}
		return updateExistingNote(gitlabClient, existingNote.ID, commentBody)
	}
	fmt.Printf(constants.LogCreatingNewNote)
	return createNewNote(gitlabClient, commentBody)
}
