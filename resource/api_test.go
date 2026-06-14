package resource

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeTempConfig writes a YAML config to a temporary file and returns its
// absolute path. The file is cleaned up when the test finishes.
func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cask.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

// resetConfigState clears the package-level state mutated by tests so they
// don't bleed into each other.
func resetConfigState(t *testing.T) {
	t.Helper()
	prevOverride := ConfigPathOverride
	prevBase := apiBaseURL
	t.Cleanup(func() {
		ConfigPathOverride = prevOverride
		apiBaseURL = prevBase
	})
	ConfigPathOverride = ""
}

func TestConfigPath_OverrideTakesPrecedence(t *testing.T) {
	resetConfigState(t)
	t.Setenv("CASKCONFIG", "/should/be/ignored.yaml")
	t.Setenv("HOME", "/home/should-also-be-ignored")

	ConfigPathOverride = "/explicit/path.yaml"
	got, err := configPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/explicit/path.yaml" {
		t.Errorf("got %q, want %q", got, "/explicit/path.yaml")
	}
}

func TestConfigPath_EnvAbsolute(t *testing.T) {
	resetConfigState(t)
	t.Setenv("CASKCONFIG", "/etc/cask/conf.yaml")

	got, err := configPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/etc/cask/conf.yaml" {
		t.Errorf("got %q, want %q", got, "/etc/cask/conf.yaml")
	}
}

func TestConfigPath_EnvRelativeRejected(t *testing.T) {
	resetConfigState(t)
	t.Setenv("CASKCONFIG", "relative/path.yaml")

	_, err := configPath()
	if err == nil {
		t.Fatal("expected error for relative CASKCONFIG, got nil")
	}
	if !strings.Contains(err.Error(), "absolute path") {
		t.Errorf("error %q should mention 'absolute path'", err.Error())
	}
}

func TestConfigPath_DefaultsToHomeDotCask(t *testing.T) {
	resetConfigState(t)
	t.Setenv("CASKCONFIG", "")

	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := configPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(home, ".cask.yaml")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLoadConfig_Valid(t *testing.T) {
	resetConfigState(t)
	ConfigPathOverride = writeTempConfig(t, "cloudflare_account_id: acc-1\ncloudflare_api_key: key-1\n")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CloudflareAccountID != "acc-1" {
		t.Errorf("account id = %q, want %q", cfg.CloudflareAccountID, "acc-1")
	}
	if cfg.CloudflareAPIKey != "key-1" {
		t.Errorf("api key = %q, want %q", cfg.CloudflareAPIKey, "key-1")
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	resetConfigState(t)
	ConfigPathOverride = filepath.Join(t.TempDir(), "does-not-exist.yaml")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read config file") {
		t.Errorf("error %q should mention 'failed to read config file'", err.Error())
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	resetConfigState(t)
	ConfigPathOverride = writeTempConfig(t, "this: : not: valid: yaml\n  - broken")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
	if !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("error %q should mention 'failed to unmarshal'", err.Error())
	}
}

func TestLoadConfig_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing api key", "cloudflare_account_id: acc-1\n"},
		{"missing account id", "cloudflare_api_key: key-1\n"},
		{"both empty", "cloudflare_account_id: \"\"\ncloudflare_api_key: \"\"\n"},
		{"empty file", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetConfigState(t)
			ConfigPathOverride = writeTempConfig(t, tc.body)

			_, err := loadConfig()
			if err == nil {
				t.Fatal("expected error for incomplete config, got nil")
			}
			if !strings.Contains(err.Error(), "cloudflare_account_id and cloudflare_api_key") {
				t.Errorf("error %q should mention required fields", err.Error())
			}
		})
	}
}

func TestSystemPrompt(t *testing.T) {
	cases := []struct {
		name     string
		mode     Mode
		contains []string
	}{
		{
			name:     "shell",
			mode:     ModeShell,
			contains: []string{"suggest shell commands", "at most 3 commands", "Do NOT use markdown"},
		},
		{
			name:     "git",
			mode:     ModeGit,
			contains: []string{"suggest git commands", "at most 3 commands", "Do NOT use markdown"},
		},
		{
			name:     "miscellaneous",
			mode:     ModeMiscellaneous,
			contains: []string{"Answer the user's technical question", "no more than 60 words"},
		},
		{
			name:     "unknown mode falls back to shell",
			mode:     Mode(99),
			contains: []string{"suggest shell commands"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := systemPrompt(tc.mode)
			for _, want := range tc.contains {
				if !strings.Contains(got, want) {
					t.Errorf("prompt %q missing substring %q", got, want)
				}
			}
		})
	}
}

func TestSystemPrompt_GitAndShellAreDifferent(t *testing.T) {
	// Regression guard: each mode should produce a distinct template so a
	// future copy-paste edit can't silently collapse them.
	shell := systemPrompt(ModeShell)
	git := systemPrompt(ModeGit)
	misc := systemPrompt(ModeMiscellaneous)
	if shell == git || shell == misc || git == misc {
		t.Errorf("prompts should be distinct per mode: shell=%q git=%q misc=%q", shell, git, misc)
	}
}

func TestApiRequest_EmptyPromptRejected(t *testing.T) {
	resetConfigState(t)
	// Config should never be read because the empty-prompt check happens first.
	ConfigPathOverride = "/definitely/not/a/real/file.yaml"

	cases := [][]string{
		nil,
		{},
		{""},
		{"   "},
		{"  ", "\t", "\n"},
	}
	for _, args := range cases {
		_, err := ApiRequest(ModeShell, args)
		if err == nil {
			t.Errorf("expected error for args %v, got nil", args)
			continue
		}
		if !strings.Contains(err.Error(), "prompt is required") {
			t.Errorf("error %q should mention 'prompt is required'", err.Error())
		}
	}
}

func TestApiRequest_ConfigLoadError(t *testing.T) {
	resetConfigState(t)
	ConfigPathOverride = filepath.Join(t.TempDir(), "missing.yaml")

	_, err := ApiRequest(ModeShell, []string{"hello"})
	if err == nil {
		t.Fatal("expected error from missing config, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read config file") {
		t.Errorf("error %q should bubble up the config read error", err.Error())
	}
}

// newTestServer spins up an httptest server, points apiBaseURL at it, and
// captures the last request observed for assertions.
type captured struct {
	method string
	path   string
	auth   string
	ctype  string
	body   []byte
}

func newTestServer(t *testing.T, status int, respBody string) (*httptest.Server, *captured) {
	t.Helper()
	cap := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.method = r.Method
		cap.path = r.URL.Path
		cap.auth = r.Header.Get("Authorization")
		cap.ctype = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		cap.body = b
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)
	return srv, cap
}

func TestApiRequest_SuccessShellMode(t *testing.T) {
	resetConfigState(t)
	srv, cap := newTestServer(t, http.StatusOK,
		`{"result":{"response":"1. find . -size +100M\n2. du -ah . | sort -rh\n3. ls -lhS"},"success":true}`)
	apiBaseURL = srv.URL

	ConfigPathOverride = writeTempConfig(t,
		"cloudflare_account_id: my-acct\ncloudflare_api_key: secret-token\n")

	items, err := ApiRequest(ModeShell, []string{"find", "big", "files"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cap.method != http.MethodPost {
		t.Errorf("method = %q, want POST", cap.method)
	}
	wantPath := "/accounts/my-acct/ai/run/@cf/meta/llama-3.1-8b-instruct-fast"
	if cap.path != wantPath {
		t.Errorf("path = %q, want %q", cap.path, wantPath)
	}
	if cap.auth != "Bearer secret-token" {
		t.Errorf("auth = %q, want %q", cap.auth, "Bearer secret-token")
	}
	if cap.ctype != "application/json" {
		t.Errorf("content-type = %q, want application/json", cap.ctype)
	}

	sys, user := decodeMessages(t, cap.body)
	if !strings.Contains(user, "find big files") {
		t.Errorf("user message %q should contain joined args", user)
	}
	if !strings.Contains(sys, "shell commands") {
		t.Errorf("system message %q should use the shell-mode template", sys)
	}

	want := []string{"find . -size +100M", "du -ah . | sort -rh", "ls -lhS"}
	if !reflect.DeepEqual(items, want) {
		t.Errorf("items = %#v, want %#v", items, want)
	}
}

func TestApiRequest_UsesGitTemplateForGitMode(t *testing.T) {
	resetConfigState(t)
	srv, cap := newTestServer(t, http.StatusOK,
		`{"result":{"response":"git revert HEAD"},"success":true}`)
	apiBaseURL = srv.URL

	ConfigPathOverride = writeTempConfig(t,
		"cloudflare_account_id: a\ncloudflare_api_key: b\n")

	if _, err := ApiRequest(ModeGit, []string{"undo", "last", "commit"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sys, _ := decodeMessages(t, cap.body)
	if !strings.Contains(sys, "git commands") {
		t.Errorf("git-mode system message %q should mention 'git commands'", sys)
	}
}

// decodeMessages unmarshals a captured request body and returns the system and
// user message contents.
func decodeMessages(t *testing.T, body []byte) (system, user string) {
	t.Helper()
	var payload struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("body is not valid JSON: %v (body=%s)", err, body)
	}
	for _, m := range payload.Messages {
		switch m.Role {
		case "system":
			system = m.Content
		case "user":
			user = m.Content
		}
	}
	return system, user
}

func TestApiRequest_HTTPErrorStatus(t *testing.T) {
	resetConfigState(t)
	srv, _ := newTestServer(t, http.StatusUnauthorized, `{"errors":["nope"]}`)
	apiBaseURL = srv.URL
	ConfigPathOverride = writeTempConfig(t,
		"cloudflare_account_id: a\ncloudflare_api_key: b\n")

	_, err := ApiRequest(ModeShell, []string{"do", "thing"})
	if err == nil {
		t.Fatal("expected error for 401 status, got nil")
	}
	if !strings.Contains(err.Error(), "API request failed") {
		t.Errorf("error %q should mention 'API request failed'", err.Error())
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error %q should include the HTTP status", err.Error())
	}
}

func TestApiRequest_APIReportsFailure(t *testing.T) {
	resetConfigState(t)
	srv, _ := newTestServer(t, http.StatusOK,
		`{"success":false,"errors":["model overloaded"]}`)
	apiBaseURL = srv.URL
	ConfigPathOverride = writeTempConfig(t,
		"cloudflare_account_id: a\ncloudflare_api_key: b\n")

	_, err := ApiRequest(ModeShell, []string{"hi"})
	if err == nil {
		t.Fatal("expected error when success=false, got nil")
	}
	if !strings.Contains(err.Error(), "API error") {
		t.Errorf("error %q should mention 'API error'", err.Error())
	}
}

func TestApiRequest_InvalidJSONResponse(t *testing.T) {
	resetConfigState(t)
	srv, _ := newTestServer(t, http.StatusOK, `not json at all`)
	apiBaseURL = srv.URL
	ConfigPathOverride = writeTempConfig(t,
		"cloudflare_account_id: a\ncloudflare_api_key: b\n")

	_, err := ApiRequest(ModeShell, []string{"hi"})
	if err == nil {
		t.Fatal("expected error for non-JSON response, got nil")
	}
}
