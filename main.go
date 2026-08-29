package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const (
	callbackAddr = "localhost:8090"
	redirectURI  = "http://" + callbackAddr
)

func getConfig(file string) (*oauth2.Config, error) {
	contents, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read client secret file: %w", err)
	}
	config, err := google.ConfigFromJSON(contents, gmail.GmailSendScope)
	if err != nil {
		return nil, fmt.Errorf("parse client secret file: %w", err)
	}
	return config, nil
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	input, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer input.Close()

	token := &oauth2.Token{}
	if err := json.NewDecoder(input).Decode(token); err != nil {
		return nil, err
	}
	return token, nil
}

func oauthCallbackHandler(state string, codeCh chan<- string, errCh chan<- error) http.Handler {
	sendError := func(err error) {
		select {
		case errCh <- err:
		default:
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/" {
			http.NotFound(response, request)
			return
		}
		if oauthError := request.FormValue("error"); oauthError != "" {
			sendError(fmt.Errorf("Google authorization failed: %s", oauthError))
			http.Error(response, "Authorization failed. Return to the terminal.", http.StatusBadRequest)
			return
		}
		if request.FormValue("state") != state {
			sendError(fmt.Errorf("invalid OAuth state"))
			http.Error(response, "Invalid authorization state.", http.StatusBadRequest)
			return
		}
		code := request.FormValue("code")
		if code == "" {
			sendError(fmt.Errorf("Google returned no authorization code"))
			http.Error(response, "Authorization code is missing.", http.StatusBadRequest)
			return
		}

		select {
		case codeCh <- code:
		default:
		}
		_, _ = fmt.Fprintln(response, "Authorization successful. You can close this window.")
	})
	return mux
}

func getTokenFromWeb(config *oauth2.Config, launchBrowser bool) (*oauth2.Token, error) {
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("create OAuth state: %w", err)
	}
	state := hex.EncodeToString(stateBytes)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	server := &http.Server{
		Addr:              callbackAddr,
		Handler:           oauthCallbackHandler(state, codeCh, errCh),
		ReadHeaderTimeout: 10 * time.Second,
	}
	defer server.Shutdown(context.Background())

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return nil, fmt.Errorf("listen for OAuth callback on %s: %w", server.Addr, err)
	}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			select {
			case errCh <- fmt.Errorf("serve OAuth callback: %w", err):
			default:
			}
		}
	}()

	config.RedirectURL = redirectURI
	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
	if launchBrowser {
		if err := openBrowser(authURL); err != nil {
			fmt.Printf("Could not open a browser: %v\n", err)
		} else {
			fmt.Println("Opened Google sign-in in your browser.")
		}
	}
	fmt.Printf("If needed, open this URL:\n%s\n", authURL)

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return nil, err
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("authorization timed out; run 'sendgmailapi setup' to try again")
	}

	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("exchange OAuth code: %w", err)
	}
	return token, nil
}

func runEncode() error {
	message, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read message: %w", err)
	}
	message, err = encodeQuotedPrintable(message)
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}
	if _, err := os.Stdout.Write(message); err != nil {
		return fmt.Errorf("write encoded message: %w", err)
	}
	return nil
}

func runSend() error {
	files, err := userConfigFiles()
	if err != nil {
		return err
	}
	config, err := getConfig(files.credentials)
	if err != nil {
		return fmt.Errorf("Gmail is not configured; run 'sendgmailapi setup': %w", err)
	}
	token, err := tokenFromFile(files.token)
	if err != nil {
		return fmt.Errorf("Gmail is not authorized; run 'sendgmailapi setup': %w", err)
	}
	client := config.Client(context.Background(), token)
	gmailService, err := gmail.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("create Gmail service: %w", err)
	}

	message, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read message: %w", err)
	}
	message, err = encodeQuotedPrintable(message)
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}

	gmailMessage := &gmail.Message{Raw: base64.RawURLEncoding.EncodeToString(message)}
	if _, err := gmailService.Users.Messages.Send("me", gmailMessage).Do(); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	fmt.Fprintln(os.Stderr, "Message sent successfully.")
	return nil
}

func printUsage() {
	fmt.Println(`Send email through the Gmail API without breaking patch lines.

Usage:
  sendgmailapi setup [credentials.json]  Guided Gmail sign-in and git setup
  sendgmailapi doctor                    Check configuration and authorization
  sendgmailapi encode                    Write Gmail-safe MIME without sending
  sendgmailapi help                      Show this help

With no command, sendgmailapi reads an RFC 5322 message from standard input.
This is the sendmail-compatible mode used by git send-email.`)
}

func runArgs(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "setup":
			return runSetup(args[1:])
		case "doctor":
			return runDoctor()
		case "encode":
			return runEncode()
		case "help", "-h", "--help":
			printUsage()
			return nil
		}
	}

	legacy := flag.NewFlagSet("sendgmailapi", flag.ContinueOnError)
	setup := legacy.Bool("setup", false, "Run the setup wizard")
	encodeOnly := legacy.Bool("encode-only", false, "Write Gmail-safe MIME to standard output")
	legacy.String("f", "", "Sendmail compatibility")
	legacy.Bool("i", true, "Sendmail compatibility")
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("unknown command %q; run 'sendgmailapi help'", args[0])
	}
	if err := legacy.Parse(args); err != nil {
		return err
	}
	if *setup {
		return runSetup(nil)
	}
	if *encodeOnly {
		return runEncode()
	}
	return runSend()
}

func run() error {
	return runArgs(os.Args[1:])
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "sendgmailapi: %v\n", err)
		os.Exit(1)
	}
}
