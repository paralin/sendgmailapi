package main

import (
	"bytes"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
)

func encodeQuotedPrintable(raw []byte) ([]byte, error) {
	header, body, ok := splitMessage(raw)
	if !ok {
		return nil, fmt.Errorf("message has no header separator")
	}

	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse message headers: %w", err)
	}
	contentType := message.Header.Get("Content-Type")
	mediaType := "text/plain"
	if contentType != "" {
		mediaType, _, err = mime.ParseMediaType(contentType)
		if err != nil {
			return nil, fmt.Errorf("parse Content-Type: %w", err)
		}
	}
	if mediaType != "text/plain" {
		return nil, fmt.Errorf("unsupported Content-Type %q", mediaType)
	}

	transferEncoding := strings.ToLower(strings.TrimSpace(message.Header.Get("Content-Transfer-Encoding")))
	switch transferEncoding {
	case "quoted-printable", "base64":
		return raw, nil
	case "", "7bit", "8bit", "binary":
	default:
		return nil, fmt.Errorf("unsupported Content-Transfer-Encoding %q", transferEncoding)
	}

	header = removeHeader(header, "Content-Transfer-Encoding")
	if message.Header.Get("MIME-Version") == "" {
		header = append(header, []byte("\nMIME-Version: 1.0")...)
	}
	if contentType == "" {
		header = append(header, []byte("\nContent-Type: text/plain; charset=UTF-8")...)
	}

	var encoded bytes.Buffer
	writer := quotedprintable.NewWriter(&encoded)
	if _, err := writer.Write(body); err != nil {
		return nil, fmt.Errorf("write quoted-printable body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("finish quoted-printable body: %w", err)
	}

	var result bytes.Buffer
	result.Write(bytes.Join(splitLines(header), []byte("\r\n")))
	result.WriteString("\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n")
	result.Write(encoded.Bytes())
	return result.Bytes(), nil
}

func splitMessage(raw []byte) (header, body []byte, ok bool) {
	if index := bytes.Index(raw, []byte("\r\n\r\n")); index >= 0 {
		return raw[:index], raw[index+4:], true
	}
	if index := bytes.Index(raw, []byte("\n\n")); index >= 0 {
		return raw[:index], raw[index+2:], true
	}
	return nil, nil, false
}

func removeHeader(header []byte, name string) []byte {
	lines := splitLines(header)
	kept := lines[:0]
	removeContinuation := false
	prefix := strings.ToLower(name) + ":"
	for _, line := range lines {
		continuation := len(line) > 0 && (line[0] == ' ' || line[0] == '\t')
		if continuation && removeContinuation {
			continue
		}
		removeContinuation = strings.HasPrefix(strings.ToLower(string(line)), prefix)
		if !removeContinuation {
			kept = append(kept, line)
		}
	}
	return bytes.Join(kept, []byte("\n"))
}

func splitLines(data []byte) [][]byte {
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	return bytes.Split(data, []byte("\n"))
}
