package output

import (
	"errors"
	"fmt"
	"os"
)

const stdoutFileIndicator = "-"

func Write(content, filename string) (err error) {
	if filename == stdoutFileIndicator {
		_, err := fmt.Print(content)
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", filename, err)
	}
	defer func() { err = errors.Join(err, file.Close()) }()

	if _, err = file.WriteString(content); err != nil {
		return fmt.Errorf("failed to write to output file %s: %w", filename, err)
	}

	return nil
}

func PrintSuccess(filename string) {
	destination := "stdout"
	if filename != stdoutFileIndicator {
		destination = filename
	}
	fmt.Fprintf(os.Stderr, "Markdown output written to %s\n", destination)
}
