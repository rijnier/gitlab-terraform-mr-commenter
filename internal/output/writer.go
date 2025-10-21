package output

import (
	"fmt"
	"os"

	"gitlab-terraform-mr-commenter/internal/constants"
)

func Write(content, filename string) error {
	if content == "" {
		return fmt.Errorf("content cannot be empty")
	}

	if filename == constants.StdoutFileIndicator {
		_, err := fmt.Print(content)
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf(constants.ErrOutputFileCreate, filename, err)
	}
	defer file.Close()

	if _, err = file.WriteString(content); err != nil {
		return fmt.Errorf(constants.ErrOutputFileWrite, filename, err)
	}

	return nil
}

func PrintSuccess(filename string) {
	destination := "stdout"
	if filename != constants.StdoutFileIndicator {
		destination = filename
	}
	fmt.Fprintf(os.Stderr, constants.SuccessMessage+"\n", destination)
}
