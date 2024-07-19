package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func getConfig(file string) (*oauth2.Config, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, gmail.GmailSendScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
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
		saveToken(tokenFile, tok)
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
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %v", err)
	}
	return tok, nil
}

func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func setupMode(config *oauth2.Config, tokenFile string) {
	tok, err := getTokenFromWeb(config)
	if err != nil {
		log.Fatalf("Unable to get token from web: %v", err)
	}
	saveToken(tokenFile, tok)
	fmt.Println("Setup completed successfully!")
}

func main() {
	setupFlag := flag.Bool("setup", false, "Run in setup mode")
	flag.Parse()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Unable to get user config directory: %v", err)
	}

	credentialsFile := filepath.Join(homeDir, ".config", "sendgmail", "credentials.json")
	tokenFile := filepath.Join(homeDir, ".config", "sendgmail", "token.json")

	config, err := getConfig(credentialsFile)
	if err != nil {
		log.Fatalf("Unable to get OAuth2 config: %v", err)
	}

	if *setupFlag {
		setupMode(config, tokenFile)
		return
	}

	client, err := getClient(config, tokenFile)
	if err != nil {
		log.Fatalf("Unable to get OAuth2 client: %v", err)
	}

	ctx := context.Background()
	gmailService, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to create Gmail service: %v", err)
	}

	message, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("Failed to read message: %v", err)
	}

	gmsg := &gmail.Message{
		Raw: base64.URLEncoding.EncodeToString(message),
	}

	_, err = gmailService.Users.Messages.Send("me", gmsg).Do()
	if err != nil {
		log.Fatalf("Unable to send email: %v", err)
	}

	fmt.Println("Message sent successfully!")
}
