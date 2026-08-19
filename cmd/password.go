package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	if len(password) == 0 {
		return "", errors.New("password cannot be empty")
	}
	return string(password), nil
}

func createPassword() (string, error) {
	fmt.Println("No Argus password is configured yet.")
	fmt.Println("Create a password to encrypt your API keys and settings.")
	password, err := readPassword("Create password: ")
	if err != nil {
		return "", err
	}
	if len(password) < 8 {
		return "", errors.New("password must be at least 8 characters")
	}
	confirmation, err := readPassword("Confirm password: ")
	if err != nil {
		return "", err
	}
	if password != confirmation {
		return "", errors.New("passwords do not match")
	}
	return password, nil
}

var resetCmd = &cobra.Command{
	Use:     "reset",
	Aliases: []string{"wipe"},
	Short:   "Delete the password, API keys, priorities, pauses, and statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := DefaultPath()
		if err != nil {
			return err
		}

		fmt.Printf("This permanently deletes %s and all stored API keys and settings.\n", path)
		fmt.Print("Type DELETE to continue: ")
		input, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && len(input) == 0 {
			return fmt.Errorf("read confirmation: %w", err)
		}
		if strings.TrimSpace(input) != "DELETE" {
			return errors.New("reset cancelled")
		}

		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete encrypted store: %w", err)
		}
		if err := os.Remove(path + ".tmp"); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete temporary store: %w", err)
		}
		fmt.Println("Argus password and stored settings were deleted.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
}
