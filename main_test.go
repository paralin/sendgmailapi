package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOAuthCallbackHandler(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantCode   string
		wantError  string
	}{
		{name: "success", target: "/?state=expected&code=code", wantStatus: http.StatusOK, wantCode: "code"},
		{name: "wrong state", target: "/?state=wrong&code=code", wantStatus: http.StatusBadRequest, wantError: "invalid OAuth state"},
		{name: "Google error", target: "/?error=access_denied", wantStatus: http.StatusBadRequest, wantError: "access_denied"},
		{name: "missing code", target: "/?state=expected", wantStatus: http.StatusBadRequest, wantError: "no authorization code"},
		{name: "not found", target: "/other", wantStatus: http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			codeCh := make(chan string, 1)
			errCh := make(chan error, 1)
			request := httptest.NewRequest(http.MethodGet, test.target, nil)
			response := httptest.NewRecorder()
			oauthCallbackHandler("expected", codeCh, errCh).ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.wantCode != "" {
				select {
				case code := <-codeCh:
					if code != test.wantCode {
						t.Fatalf("code = %q, want %q", code, test.wantCode)
					}
				default:
					t.Fatal("callback produced no code")
				}
			}
			if test.wantError != "" {
				select {
				case err := <-errCh:
					if !strings.Contains(err.Error(), test.wantError) {
						t.Fatalf("error = %q, want text %q", err, test.wantError)
					}
				default:
					t.Fatal("callback produced no error")
				}
			}
		})
	}
}

func TestRunArgsRejectsUnknownCommand(t *testing.T) {
	err := runArgs([]string{"staus"})
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("runArgs() error = %v, want unknown command", err)
	}
}
