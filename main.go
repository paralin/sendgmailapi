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
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const redirectURI = "http://localhost:8090"

var (
	dummyF string
	dummyI bool
)

func getConfig(file string) (*oauth2.Config, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read client secret file: %w", err)
	}

	config, err := google.ConfigFromJSON(b, gmail.GmailSendScope)
	if err != nil {
		return nil, fmt.Errorf("parse client secret file: %w", err)
	}

	return config, nil
}

func getClient(config *oauth2.Config, tokenFile string) (*http.Client, error) {
	tok, err := tokenFromFile(tokenFile)
	if err != nil {
		tok, err = getTokenFromWeb(config)
		if err != nil {
			return nil, err
		}
		if err := saveToken(tokenFile, tok); err != nil {
			return nil, err
		}
	}
	return config.Client(context.Background(), tok), nil
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tok := &oauth2.Token{}
	if err := json.NewDecoder(f).Decode(tok); err != nil {
		return nil, err
	}
	return tok, nil
}

func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("create OAuth state: %w", err)
	}
	state := hex.EncodeToString(stateBytes)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:              redirectURI[7:],
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.FormValue("state") != state {
			errCh <- fmt.Errorf("invalid OAuth state")
			http.Error(w, "Invalid state", http.StatusBadRequest)
			return
		}

		codeCh <- r.FormValue("code")
		_, _ = fmt.Fprintln(w, "Authorization successful. You can close this window.")
		go func() {
			if err := server.Shutdown(context.Background()); err != nil {
				log.Printf("shut down OAuth server: %v", err)
			}
		}()
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("serve OAuth callback: %w", err)
		}
	}()

	config.RedirectURL = redirectURI
	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline)
	fmt.Printf("Listening on %s\n", redirectURI)
	fmt.Printf("Visit this URL to authorize the application:\n%s\n", authURL)

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return nil, err
	case <-time.After(2 * time.Minute):
		return nil, fmt.Errorf("authorization timed out")
	}

	tok, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("exchange OAuth code: %w", err)
	}
	return tok, nil
}

func saveToken(path string, token *oauth2.Token) error {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("open OAuth token file: %w", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(token); err != nil {
		return fmt.Errorf("write OAuth token file: %w", err)
	}
	return nil
}

func setupMode(config *oauth2.Config, tokenFile string) error {
	tok, err := getTokenFromWeb(config)
	if err != nil {
		return err
	}
	if err := saveToken(tokenFile, tok); err != nil {
		return err
	}
	fmt.Println("Setup completed successfully!")
	return nil
}

func run() error {
	setupFlag := flag.Bool("setup", false, "Run in setup mode")
	encodeOnlyFlag := flag.Bool("encode-only", false, "Write the Gmail-safe MIME message to standard output")
	flag.StringVar(&dummyF, "f", "", "Dummy flag for sendmail compatibility")
	flag.BoolVar(&dummyI, "i", true, "Dummy flag for sendmail compatibility")
	flag.Parse()

	if *encodeOnlyFlag {
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

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find user home directory: %w", err)
	}

	credentialsFile := filepath.Join(homeDir, ".config", "sendgmail", "credentials.json")
	tokenFile := filepath.Join(homeDir, ".config", "sendgmail", "token.json")
	config, err := getConfig(credentialsFile)
	if err != nil {
		return fmt.Errorf("load OAuth config: %w", err)
	}

	if *setupFlag {
		return setupMode(config, tokenFile)
	}

	client, err := getClient(config, tokenFile)
	if err != nil {
		return fmt.Errorf("create OAuth client: %w", err)
	}
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

	gmsg := &gmail.Message{Raw: base64.RawURLEncoding.EncodeToString(message)}
	if _, err := gmailService.Users.Messages.Send("me", gmsg).Do(); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	fmt.Println("Message sent successfully!")
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
