# SendGmailAPI

[![GoDoc Widget]][GoDoc] [![Go Report Card Widget]][Go Report Card]

> Send emails with git send-email over the Gmail API.

[GoDoc]: https://godoc.org/github.com/paralin/sendgmailapi
[GoDoc Widget]: https://godoc.org/github.com/paralin/sendgmailapi?status.svg
[Go Report Card Widget]: https://goreportcard.com/badge/github.com/paralin/sendgmailapi
[Go Report Card]: https://goreportcard.com/report/github.com/paralin/sendgmailapi

## Introduction

SendGmailAPI is a Go application that allows you to send emails using the Gmail
API. It's particularly useful for developers who want to use `git send-email`
with their Gmail account, bypassing the need for SMTP configuration.

> **Warning**: Gmail [automatically wraps emails to 72 characters], which breaks patches sent with `git send-email`.

[automatically wraps emails to 72 characters]: https://github.com/google/gmail-oauth2-tools/issues/32#issuecomment-2401237305

## Setup

### Enable the API

1. Go to the [Google Cloud console](https://console.cloud.google.com/marketplace/product/google/gmail.googleapis.com) and enable the Gmail API.

### Configure the OAuth consent screen

1. In the Google Cloud console, go to [OAuth consent screen](https://console.cloud.google.com/apis/credentials/consent).
2. For User type, select Internal, then click Create.
3. Complete the app registration form, then click Save and Continue.
4. Skip adding scopes and click Save and Continue.
5. Review your app registration summary. To make changes, click Edit. If the app registration looks OK, click Back to Dashboard.

### Authorize credentials for a web application

1. In the Google Cloud console, go to [Credentials](https://console.cloud.google.com/apis/credentials).
2. Click Create Credentials > OAuth client ID.
3. Click Application type > Web application.
4. In the Name field, type a name for the credential like "sendgmailapi".
5. Add http://localhost:8090 as an authorized redirect URI.
6. Click Create. The OAuth client created screen appears, showing your new Client ID and Client secret.
7. Download the JSON file with the credentials.

Note: This application now uses a local server to handle the OAuth2 flow, which is more secure and doesn't rely on external services.

### Set up credentials

1. Create a directory for configuration:
   ```
   mkdir -p ~/.config/sendgmail
   chmod 0700 ~/.config/sendgmail
   ```
2. Move the downloaded JSON file to this directory:
   ```
   mv ~/Downloads/client_secret*.json ~/.config/sendgmail/credentials.json
   chmod 0600 ~/.config/sendgmail/credentials.json
   ```

### Add test user

1. Go back to APIs & Services > OAuth consent screen in the Google Cloud console.
2. Add your Gmail address (e.g., USERNAME@gmail.com) as a test user.

## Usage

Install sendgmailapi:

```
go install github.com/paralin/sendgmailapi@latest
```

Run the setup to get the token:

```
$(go env GOPATH)/bin/sendgmailapi -setup
```

This will open a browser window for you to authorize the application and generate the token.

Once set up, you can use SendGmailAPI to send emails. The application reads the email content from standard input.

Add to your .gitconfig at ~/.gitconfig:

```
git config --global sendemail.smtpServer $(go env GOPATH)/bin/sendgmailapi
```

Or to send a simple email:

```
echo "Subject: Test Email
To: recipient@example.com

This is a test email." | sendgmailapi
```

## License

MIT

