package main

import (
	"bufio"
	"bytes"
	"io"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"
)

func TestEncodeQuotedPrintableRoundTrip(t *testing.T) {
	body := "diff --git a/file b/file\n" +
		"+" + strings.Repeat("0123456789", 12) + "\n" +
		"+trailing space \n" +
		"+equals=value\n"
	raw := []byte("From: sender@example.com\n" +
		"To: recipient@example.com\n" +
		"Subject: [PATCH] preserve long lines\n" +
		"Content-Type: text/plain; charset=UTF-8\n" +
		"Content-Transfer-Encoding: 8bit\n\n" + body)

	encoded, err := encodeQuotedPrintable(raw)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("Content-Transfer-Encoding: 8bit")) {
		t.Fatal("old transfer encoding remains")
	}
	if !bytes.Contains(encoded, []byte("Content-Transfer-Encoding: quoted-printable")) {
		t.Fatal("quoted-printable transfer encoding is missing")
	}

	_, encodedBody, ok := splitMessage(encoded)
	if !ok {
		t.Fatal("encoded message has no body")
	}
	scanner := bufio.NewScanner(bytes.NewReader(encodedBody))
	for scanner.Scan() {
		if length := len(scanner.Bytes()); length > 76 {
			t.Fatalf("transport line has %d bytes: %q", length, scanner.Bytes())
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	decoded, err := io.ReadAll(quotedprintable.NewReader(bytes.NewReader(encodedBody)))
	if err != nil {
		t.Fatal(err)
	}
	want := []byte(strings.ReplaceAll(body, "\n", "\r\n"))
	if !bytes.Equal(decoded, want) {
		t.Fatalf("decoded body differs\n got: %q\nwant: %q", decoded, want)
	}
}

func TestEncodeQuotedPrintablePreservesFoldedHeaders(t *testing.T) {
	raw := []byte("From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: a long subject\r\n\tcontinued here\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\nbody\r\n")

	encoded, err := encodeQuotedPrintable(raw)
	if err != nil {
		t.Fatal(err)
	}
	message, err := mail.ReadMessage(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if got := message.Header.Get("Subject"); got != "a long subject continued here" {
		t.Fatalf("unexpected Subject: %q", got)
	}
}

func TestEncodeQuotedPrintableLeavesSafeEncoding(t *testing.T) {
	raw := []byte("Content-Type: text/plain\r\nContent-Transfer-Encoding: base64\r\n\r\nYm9keQ==\r\n")

	encoded, err := encodeQuotedPrintable(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, raw) {
		t.Fatal("base64 message changed")
	}
}

func TestEncodeQuotedPrintableAddsDefaultContentType(t *testing.T) {
	raw := []byte("From: sender@example.com\nSubject: [PATCH] test\n\n+patch body\n")

	encoded, err := encodeQuotedPrintable(raw)
	if err != nil {
		t.Fatal(err)
	}
	message, err := mail.ReadMessage(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if got := message.Header.Get("Content-Type"); got != "text/plain; charset=UTF-8" {
		t.Fatalf("unexpected Content-Type: %q", got)
	}
}
