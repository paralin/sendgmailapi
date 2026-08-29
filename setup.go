package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

type configFiles struct {
	directory   string
	credentials string
	token       string
}

func userConfigFiles() (configFiles, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return configFiles{}, fmt.Errorf("find user home directory: %w", err)
	}
	directory := filepath.Join(homeDir, ".config", "sendgmail")
	return configFiles{
		directory:   directory,
		credentials: filepath.Join(directory, "credentials.json"),
		token:       filepath.Join(directory, "token.json"),
	}, nil
}

func runSetup(args []string) error {
	flags := flag.NewFlagSet("setup", flag.ContinueOnError)
	noBrowser := flags.Bool("no-browser", false, "Print URLs instead of opening a browser")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 1 {
		return fmt.Errorf("usage: sendgmailapi setup [--no-browser] [credentials.json]")
	}

	fmt.Println("SendGmailAPI setup")
	fmt.Println("==================")
	fmt.Println("This wizard connects your Gmail account and configures git send-email.")
	reader := bufio.NewReader(os.Stdin)
	files, err := userConfigFiles()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(files.directory, 0700); err != nil {
		return fmt.Errorf("create configuration directory: %w", err)
	}
	if err := os.Chmod(files.directory, 0700); err != nil {
		return fmt.Errorf("secure configuration directory: %w", err)
	}

	credentialsSource := ""
	if flags.NArg() == 1 {
		credentialsSource = flags.Arg(0)
	} else if _, err := os.Stat(files.credentials); errors.Is(err, fs.ErrNotExist) {
		credentialsSource, err = guideCredentials(reader, !*noBrowser)
		if err != nil {
			return err
		}
	}
	if credentialsSource != "" {
		if err := installCredentials(credentialsSource, files.credentials); err != nil {
			return err
		}
		fmt.Printf("OAuth client: imported %s\n", credentialsSource)
	} else {
		fmt.Printf("OAuth client: using %s\n", files.credentials)
	}

	config, err := getConfig(files.credentials)
	if err != nil {
		return fmt.Errorf("load OAuth client: %w", err)
	}
	fmt.Println("\nStep 5 of 6: Sign in to Gmail")
	token, err := getTokenFromWeb(config, !*noBrowser)
	if err != nil {
		return err
	}
	if err := saveToken(files.token, token); err != nil {
		return err
	}
	fmt.Println("Gmail authorization: saved")

	executable, err := os.Executable()
	if err != nil {
		executable = "sendgmailapi"
	}
	fmt.Println("\nStep 6 of 6: Configure git send-email")
	if askYesNo(reader, "Configure git send-email now?", true) {
		command := exec.Command("git", "config", "--global", "sendemail.smtpServer", executable)
		if output, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("configure git send-email: %w: %s", err, strings.TrimSpace(string(output)))
		}
		fmt.Printf("git send-email: configured to use %s\n", executable)
	} else {
		fmt.Println("Run this later:")
		fmt.Printf("  git config --global sendemail.smtpServer %q\n", executable)
	}

	fmt.Println("\nSetup complete. Verify it with:")
	fmt.Println("  sendgmailapi doctor")
	return nil
}

func guideCredentials(reader *bufio.Reader, launchBrowser bool) (string, error) {
	fmt.Println("\nGoogle Cloud configuration")
	fmt.Println("The wizard can reuse a client JSON that you already downloaded.")
	if downloaded, err := findDownloadedCredentials(); err == nil {
		fmt.Printf("Found: %s\n", downloaded)
		if askYesNo(reader, "Use this OAuth client and skip Google Cloud setup?", true) {
			return downloaded, nil
		}
	}

	fmt.Println("\nStep 1 of 6: Select a project and enable the Gmail API")
	fmt.Println("In Google Cloud:")
	fmt.Println("  1. Select an existing project or create a new project.")
	fmt.Println("  2. Open the Gmail API page.")
	fmt.Println("  3. Click Enable if the API is not already enabled.")
	visitPage(reader, launchBrowser, "Open the Gmail API page?", "https://console.cloud.google.com/apis/library/gmail.googleapis.com")
	if _, err := readLine(reader, "Press Enter after the Gmail API is enabled..."); err != nil {
		return "", err
	}

	fmt.Println("\nStep 2 of 6: Configure the OAuth consent screen")
	fmt.Println("Open Google Auth Platform for the same project, then:")
	fmt.Println("  1. Enter an app name, such as SendGmailAPI.")
	fmt.Println("  2. Select your email for user support and developer contact.")
	fmt.Println("  3. Choose Internal only for the intended Google Workspace organization.")
	fmt.Println("     Otherwise choose External.")
	fmt.Println("  4. Finish and save the initial app configuration.")
	fmt.Println("  5. For an External app in Testing, open Audience and add your Gmail")
	fmt.Println("     address under Test users.")
	fmt.Println("Google may expire refresh tokens after 7 days while an External app is in")
	fmt.Println("Testing. Move the app to Production for lasting authorization; Google may")
	fmt.Println("show an unverified-app warning or require verification.")
	visitPage(reader, launchBrowser, "Open Google Auth Platform?", "https://console.cloud.google.com/auth/overview")
	if _, err := readLine(reader, "Press Enter after the initial app configuration is saved..."); err != nil {
		return "", err
	}

	fmt.Println("\nStep 3 of 6: Add Gmail permission under Data Access")
	fmt.Println("After saving the initial app configuration:")
	fmt.Println("  1. Open Data Access in Google Auth Platform.")
	fmt.Println("  2. Click Add or remove scopes.")
	fmt.Println("  3. Find and select this Gmail API scope:")
	fmt.Println("     https://www.googleapis.com/auth/gmail.send")
	fmt.Println("  4. Click Update, then save the Data Access changes.")
	visitPage(reader, launchBrowser, "Open Data Access?", "https://console.cloud.google.com/auth/scopes")
	if _, err := readLine(reader, "Press Enter after the gmail.send scope is saved..."); err != nil {
		return "", err
	}

	fmt.Println("\nStep 4 of 6: Create and download an OAuth client")
	fmt.Println("In Google Auth Platform > Clients:")
	fmt.Println("  1. Click Create client.")
	fmt.Println("  2. Select Web application.")
	fmt.Println("  3. Add this Authorized redirect URI exactly:")
	fmt.Printf("     %s\n", redirectURI)
	fmt.Println("  4. Create the client and download its JSON file.")
	visitPage(reader, launchBrowser, "Open the OAuth clients page?", "https://console.cloud.google.com/auth/clients")
	if _, err := readLine(reader, "Press Enter after the JSON file finishes downloading..."); err != nil {
		return "", err
	}
	if downloaded, err := findDownloadedCredentials(); err == nil {
		fmt.Printf("Found: %s\n", downloaded)
		return downloaded, nil
	}

	path, err := readLine(reader, "The JSON was not found automatically. Enter its path: ")
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", fmt.Errorf("no OAuth client JSON selected")
	}
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(homeDir, path[2:])
	}
	return path, nil
}

func visitPage(reader *bufio.Reader, launchBrowser bool, prompt, url string) {
	if launchBrowser && askYesNo(reader, prompt, true) {
		if err := openBrowser(url); err != nil {
			fmt.Printf("Could not open the browser: %v\n", err)
			fmt.Printf("Open this URL manually: %s\n", url)
		}
		return
	}
	fmt.Printf("Open: %s\n", url)
}

func askYesNo(reader *bufio.Reader, prompt string, defaultYes bool) bool {
	suffix := " [Y/n] "
	if !defaultYes {
		suffix = " [y/N] "
	}
	for {
		answer, err := readLine(reader, prompt+suffix)
		if err != nil {
			return false
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "":
			return defaultYes
		case "y", "yes":
			return true
		case "n", "no":
			return false
		default:
			fmt.Println("Please answer yes or no.")
		}
	}
}

func readLine(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)
	line, err := reader.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) && line != "" {
			return strings.TrimSpace(line), nil
		}
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func runDoctor() error {
	files, err := userConfigFiles()
	if err != nil {
		return err
	}
	config, err := getConfig(files.credentials)
	if err != nil {
		return fmt.Errorf("OAuth client: not ready (%w)\nRun: sendgmailapi setup credentials.json", err)
	}
	fmt.Printf("OAuth client: %s\n", files.credentials)

	token, err := tokenFromFile(files.token)
	if err != nil {
		return fmt.Errorf("Gmail authorization: not ready (%w)\nRun: sendgmailapi setup", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := config.TokenSource(ctx, token).Token(); err != nil {
		return fmt.Errorf("Gmail authorization: invalid (%w)\nRun: sendgmailapi setup", err)
	}
	fmt.Printf("Gmail authorization: %s\n", files.token)
	fmt.Println("Status: ready to send")
	return nil
}

func findDownloadedCredentials() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find user home directory: %w", err)
	}
	patterns := []string{
		filepath.Join(homeDir, "Downloads", "client_secret*.json"),
		filepath.Join(homeDir, "Downloads", "credentials*.json"),
	}
	type candidate struct {
		path    string
		modTime time.Time
	}
	var candidates []candidate
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return "", fmt.Errorf("search downloaded OAuth clients: %w", err)
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err == nil && !info.IsDir() {
				candidates = append(candidates, candidate{path: match, modTime: info.ModTime()})
			}
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no OAuth client JSON found in Downloads\nDownload it from Google Cloud, then run: sendgmailapi setup credentials.json")
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime.After(candidates[j].modTime)
	})
	return candidates[0].path, nil
}

func installCredentials(source, destination string) error {
	if _, err := getConfig(source); err != nil {
		return fmt.Errorf("validate OAuth client %s: %w", source, err)
	}
	contents, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read OAuth client %s: %w", source, err)
	}
	if err := writePrivateFile(destination, contents); err != nil {
		return fmt.Errorf("install OAuth client: %w", err)
	}
	return nil
}

func writePrivateFile(path string, contents []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".sendgmailapi-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func openBrowser(url string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", url)
	case "linux":
		command = exec.Command("xdg-open", url)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("automatic browser opening is unsupported on %s", runtime.GOOS)
	}
	if err := command.Run(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return nil
}

func saveToken(path string, token *oauth2.Token) error {
	contents, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("encode OAuth token: %w", err)
	}
	if err := writePrivateFile(path, contents); err != nil {
		return fmt.Errorf("save OAuth token: %w", err)
	}
	return nil
}
