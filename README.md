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

Gmail rewrites long lines in unencoded plain-text messages. SendGmailAPI avoids
that rewrite by converting plain-text message bodies to quoted-printable MIME.
The transport lines remain within the MIME limit, and the recipient's mail
client reconstructs the original patch lines.

## Quick start

Install the command:

```sh
go install github.com/paralin/sendgmailapi@latest
```

Start the interactive setup wizard:

```sh
sendgmailapi setup
```

The wizard opens each Google Cloud page and pauses while you complete these
steps:

1. Select or create a Google Cloud project and enable the Gmail API.
2. Configure the OAuth consent screen in Google Auth Platform.
3. Choose the audience. Use **Internal** only for the intended Google Workspace
   organization. Otherwise use **External**.
4. Under **Data Access**, add the Gmail API scope
   `https://www.googleapis.com/auth/gmail.send`.
5. For an External app in Testing, add your Gmail address under **Audience >
   Test users**. Google may expire refresh tokens after seven days while the
   app remains in Testing. Move it to Production for lasting authorization;
   Google may show an unverified-app warning or require verification.
6. Create a **Web application** OAuth client. Add
   `http://localhost:8090` under **Authorized redirect URIs** exactly as shown.
7. Download the client JSON. The wizard finds it in `~/Downloads`, or asks for
   its path.
8. Sign in to Google and approve the `gmail.send` permission.
9. Let the wizard configure `git send-email`.

Use the same Google Cloud project for the Gmail API, consent screen, audience,
and OAuth client. Enter an app name such as `SendGmailAPI`, and select your
email address for the user-support and developer-contact fields.

You can also give the downloaded file directly:

```sh
sendgmailapi setup ~/Downloads/client_secret_....json
```

If the Google consent screen is in testing mode, add your Gmail account as a
test user before signing in. The wizard stores the OAuth client and token with
private file permissions under `~/.config/sendgmail/`.

Check the setup without sending mail:

```sh
sendgmailapi doctor
```

## Send patches

After setup, use `git send-email` normally:

```sh
git send-email --to recipient@example.com outgoing/*.patch
```

SendGmailAPI runs as the configured sendmail command. It converts plain-text
messages to quoted-printable MIME before calling the Gmail API, so Gmail does
not wrap long patch lines.

## Other commands

Send an RFC 5322 message directly:

```sh
cat message.eml | sendgmailapi
```

Inspect the Gmail-safe MIME without sending it:

```sh
sendgmailapi encode < message.eml
```

Show command help:

```sh
sendgmailapi help
```

The legacy `-setup` and `-encode-only` flags remain available.

## License

MIT

