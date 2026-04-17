package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

func configDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "please"), nil
}

func configFile() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config"), nil
}

// loadAPIKey returns the API key from env first, then config file.
func loadAPIKey() (string, error) {
	if k := os.Getenv("ANTHROPIC_API_KEY"); k != "" {
		return k, nil
	}
	path, err := configFile()
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "ANTHROPIC_API_KEY=") {
			return strings.TrimPrefix(line, "ANTHROPIC_API_KEY="), nil
		}
	}
	return "", scanner.Err()
}

// saveAPIKey writes the key to the config file with 0600 permissions.
func saveAPIKey(key string) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, "config")

	// Preserve any existing non-API-key lines
	var lines []string
	if f, err := os.Open(path); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(strings.TrimSpace(line), "ANTHROPIC_API_KEY=") {
				lines = append(lines, line)
			}
		}
		f.Close()
	}
	lines = append(lines, "ANTHROPIC_API_KEY="+key)

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0600)
}

// promptAndSaveKey interactively asks for the API key, saves it, and returns it.
func promptAndSaveKey(reason string) (string, error) {
	if reason != "" {
		fmt.Fprintln(os.Stderr, reason)
	}
	fmt.Fprint(os.Stderr, "Enter Anthropic API key: ")

	var key string
	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr) // newline after hidden input
		if err != nil {
			return "", err
		}
		key = strings.TrimSpace(string(b))
	} else {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			key = strings.TrimSpace(scanner.Text())
		}
	}

	if key == "" {
		return "", fmt.Errorf("no key entered")
	}
	if err := saveAPIKey(key); err != nil {
		return "", fmt.Errorf("could not save key: %w", err)
	}

	path, _ := configFile()
	fmt.Fprintf(os.Stderr, "API key saved to %s\n", path)
	return key, nil
}

// resolveAPIKey returns the key from env/config, prompting if missing.
func resolveAPIKey() (string, error) {
	key, err := loadAPIKey()
	if err != nil {
		return "", err
	}
	if key != "" {
		return key, nil
	}
	return promptAndSaveKey("No ANTHROPIC_API_KEY found.")
}

// runSetup is the --setup flow: always prompt, even if a key already exists.
func runSetup() {
	existing, _ := loadAPIKey()
	reason := "Set your Anthropic API key."
	if existing != "" {
		reason = "A key is already configured — enter a new one to replace it."
	}
	if _, err := promptAndSaveKey(reason); err != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render("setup failed: "+err.Error()))
		os.Exit(1)
	}
}
