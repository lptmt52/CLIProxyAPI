package management

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/auth"
)

func TestUploadAuthFile_BatchMultipart(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	files := []struct {
		name    string
		content string
	}{
		{name: "alpha.json", content: `{"type":"codex","email":"alpha@example.com"}`},
		{name: "beta.json", content: `{"type":"claude","email":"beta@example.com"}`},
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, file := range files {
		part, err := writer.CreateFormFile("file", file.name)
		if err != nil {
			t.Fatalf("failed to create multipart file: %v", err)
		}
		if _, err = part.Write([]byte(file.content)); err != nil {
			t.Fatalf("failed to write multipart content: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v0/management/auth-files", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Request = req

	h.UploadAuthFile(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected upload status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got, ok := payload["uploaded"].(float64); !ok || int(got) != len(files) {
		t.Fatalf("expected uploaded=%d, got %#v", len(files), payload["uploaded"])
	}

	for _, file := range files {
		fullPath := filepath.Join(authDir, file.name)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("expected uploaded file %s to exist: %v", file.name, err)
		}
		if string(data) != file.content {
			t.Fatalf("expected file %s content %q, got %q", file.name, file.content, string(data))
		}
	}

	auths := manager.List()
	if len(auths) != len(files) {
		t.Fatalf("expected %d auth entries, got %d", len(files), len(auths))
	}
}

func TestUploadAuthFile_MultipartSingleArraySplitsImport(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	content := `[{"type":"codex","email":"alpha@example.com"},{"type":"claude","email":"beta@example.com"}]`

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "bundle.json")
	if err != nil {
		t.Fatalf("failed to create multipart file: %v", err)
	}
	if _, err = part.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v0/management/auth-files", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Request = req

	h.UploadAuthFile(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected upload status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got, ok := payload["uploaded"].(float64); !ok || int(got) != 2 {
		t.Fatalf("expected uploaded=2, got %#v", payload["uploaded"])
	}

	files, ok := payload["files"].([]any)
	if !ok || len(files) != 2 {
		t.Fatalf("expected two generated file names, got %#v", payload["files"])
	}
	if files[0] != "bundle_001.json" || files[1] != "bundle_002.json" {
		t.Fatalf("unexpected generated file names: %#v", payload["files"])
	}

	expected := map[string]string{
		"bundle_001.json": `{"type":"codex","email":"alpha@example.com"}`,
		"bundle_002.json": `{"type":"claude","email":"beta@example.com"}`,
	}
	for name, want := range expected {
		data, err := os.ReadFile(filepath.Join(authDir, name))
		if err != nil {
			t.Fatalf("expected generated auth file %s to exist: %v", name, err)
		}
		if string(data) != want {
			t.Fatalf("expected file %s content %q, got %q", name, want, string(data))
		}
	}

	auths := manager.List()
	if len(auths) != 2 {
		t.Fatalf("expected 2 auth entries, got %d", len(auths))
	}
}

func TestUploadAuthFile_BatchMultipart_InvalidJSONDoesNotOverwriteExistingFile(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	existingName := "alpha.json"
	existingContent := `{"type":"codex","email":"alpha@example.com"}`
	if err := os.WriteFile(filepath.Join(authDir, existingName), []byte(existingContent), 0o600); err != nil {
		t.Fatalf("failed to seed existing auth file: %v", err)
	}

	files := []struct {
		name    string
		content string
	}{
		{name: existingName, content: `{"type":"codex"`},
		{name: "beta.json", content: `{"type":"claude","email":"beta@example.com"}`},
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, file := range files {
		part, err := writer.CreateFormFile("file", file.name)
		if err != nil {
			t.Fatalf("failed to create multipart file: %v", err)
		}
		if _, err = part.Write([]byte(file.content)); err != nil {
			t.Fatalf("failed to write multipart content: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v0/management/auth-files", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Request = req

	h.UploadAuthFile(ctx)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("expected upload status %d, got %d with body %s", http.StatusMultiStatus, rec.Code, rec.Body.String())
	}

	data, err := os.ReadFile(filepath.Join(authDir, existingName))
	if err != nil {
		t.Fatalf("expected existing auth file to remain readable: %v", err)
	}
	if string(data) != existingContent {
		t.Fatalf("expected existing auth file to remain %q, got %q", existingContent, string(data))
	}

	betaData, err := os.ReadFile(filepath.Join(authDir, "beta.json"))
	if err != nil {
		t.Fatalf("expected valid auth file to be created: %v", err)
	}
	if string(betaData) != files[1].content {
		t.Fatalf("expected beta auth file content %q, got %q", files[1].content, string(betaData))
	}
}

func TestDeleteAuthFile_BatchQuery(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")

	authDir := t.TempDir()
	files := []string{"alpha.json", "beta.json"}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(authDir, name), []byte(`{"type":"codex"}`), 0o600); err != nil {
			t.Fatalf("failed to write auth file %s: %v", name, err)
		}
	}

	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	h.tokenStore = &memoryAuthStore{}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(
		http.MethodDelete,
		"/v0/management/auth-files?name="+url.QueryEscape(files[0])+"&name="+url.QueryEscape(files[1]),
		nil,
	)
	ctx.Request = req

	h.DeleteAuthFile(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected delete status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got, ok := payload["deleted"].(float64); !ok || int(got) != len(files) {
		t.Fatalf("expected deleted=%d, got %#v", len(files), payload["deleted"])
	}

	for _, name := range files {
		if _, err := os.Stat(filepath.Join(authDir, name)); !os.IsNotExist(err) {
			t.Fatalf("expected auth file %s to be removed, stat err: %v", name, err)
		}
	}
}

func TestUploadAuthFile_TxtCPAIsStoredAsJSON(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	content := `{"type":"codex","email":"txt@example.com"}`
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "txt-auth.txt")
	if err != nil {
		t.Fatalf("failed to create multipart file: %v", err)
	}
	if _, err = part.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v0/management/auth-files", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Request = req

	h.UploadAuthFile(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected upload status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	data, err := os.ReadFile(filepath.Join(authDir, "txt-auth.json"))
	if err != nil {
		t.Fatalf("expected converted .json file to exist: %v", err)
	}
	if string(data) != content {
		t.Fatalf("expected CPA content %q, got %q", content, string(data))
	}
	if _, err := os.Stat(filepath.Join(authDir, "txt-auth.txt")); !os.IsNotExist(err) {
		t.Fatalf("did not expect .txt auth file, stat err: %v", err)
	}
}

func TestUploadAuthFile_Sub2APIIsConvertedToCPA(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	content := `{"name":"sub2api@example.com","platform":"openai","type":"oauth","credentials":{"access_token":"at","refresh_token":"rt","id_token":"idt","account_id":"acc"},"proxy_url":"http://127.0.0.1:7890","priority":1}`
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "sub2api.json")
	if err != nil {
		t.Fatalf("failed to create multipart file: %v", err)
	}
	if _, err = part.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v0/management/auth-files", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Request = req

	h.UploadAuthFile(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected upload status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	data, err := os.ReadFile(filepath.Join(authDir, "sub2api.json"))
	if err != nil {
		t.Fatalf("expected converted auth file to exist: %v", err)
	}
	var converted map[string]any
	if err := json.Unmarshal(data, &converted); err != nil {
		t.Fatalf("failed to decode converted auth: %v", err)
	}
	expected := map[string]any{
		"type":          "codex",
		"email":         "sub2api@example.com",
		"access_token":  "at",
		"refresh_token": "rt",
		"id_token":      "idt",
		"account_id":    "acc",
		"proxy_url":     "http://127.0.0.1:7890",
		"priority":      float64(1),
	}
	for key, want := range expected {
		if got := converted[key]; got != want {
			t.Errorf("converted[%q] = %#v, want %#v", key, got, want)
		}
	}
	if got := converted["type"]; got == "oauth" {
		t.Fatalf("sub2api type must not be retained as CPA type")
	}
	if len(manager.List()) != 1 {
		t.Fatalf("expected one registered auth, got %d", len(manager.List()))
	}
}

func TestUploadAuthFile_Sub2APIAccountsArraySplitsImport(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	content := `{"accounts":[{"name":"one@example.com","platform":"openai","type":"oauth","credentials":{"access_token":"at-1"}},{"name":"two@example.com","platform":"claude","type":"oauth","credentials":{"access_token":"at-2"}}]}`
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "sub2api-bundle.txt")
	if err != nil {
		t.Fatalf("failed to create multipart file: %v", err)
	}
	if _, err = part.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v0/management/auth-files", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Request = req

	h.UploadAuthFile(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected upload status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	for name, wantType := range map[string]string{
		"sub2api-bundle_001.json": "codex",
		"sub2api-bundle_002.json": "claude",
	} {
		data, err := os.ReadFile(filepath.Join(authDir, name))
		if err != nil {
			t.Fatalf("expected generated auth file %s to exist: %v", name, err)
		}
		var converted map[string]any
		if err := json.Unmarshal(data, &converted); err != nil {
			t.Fatalf("failed to decode %s: %v", name, err)
		}
		if got := converted["type"]; got != wantType {
			t.Errorf("%s type = %#v, want %q", name, got, wantType)
		}
	}
	if len(manager.List()) != 2 {
		t.Fatalf("expected two registered auths, got %d", len(manager.List()))
	}
}

func TestUploadAuthFile_ChatGPTSessionContentGeneratesCPAName(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	content, err := json.Marshal(map[string]any{
		"type":         "session",
		"accessToken":  "access-token",
		"sessionToken": "session-token",
		"user":         map[string]any{"email": "chatgpt@example.com"},
		"account":      map[string]any{"planType": "plus"},
	})
	if err != nil {
		t.Fatalf("failed to encode session content: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v0/management/auth-files?name=auth-content.json", bytes.NewReader(content))
	ctx.Request = req

	h.UploadAuthFile(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected upload status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	files, ok := payload["files"].([]any)
	if !ok || len(files) != 1 {
		t.Fatalf("expected one generated file, got %#v", payload["files"])
	}
	name, ok := files[0].(string)
	if !ok {
		t.Fatalf("generated file name has unexpected type: %#v", files[0])
	}
	prefix := "chatgpt@example.com-plus-"
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".json") {
		t.Fatalf("unexpected generated filename %q", name)
	}
	timestamp := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".json")
	if len(timestamp) != len("20060102_150405_000") {
		t.Fatalf("unexpected timestamp in generated filename %q", name)
	}

	data, err := os.ReadFile(filepath.Join(authDir, name))
	if err != nil {
		t.Fatalf("expected generated auth file %s to exist: %v", name, err)
	}
	var converted map[string]any
	if err := json.Unmarshal(data, &converted); err != nil {
		t.Fatalf("failed to decode generated auth: %v", err)
	}
	if converted["type"] != "codex" {
		t.Fatalf("generated auth type = %#v, want codex", converted["type"])
	}
	if converted["access_token"] != "access-token" {
		t.Fatalf("generated access token = %#v, want access-token", converted["access_token"])
	}
	if converted["email"] != "chatgpt@example.com" {
		t.Fatalf("generated email = %#v, want chatgpt@example.com", converted["email"])
	}
	if converted["plan_type"] != "plus" {
		t.Fatalf("generated plan type = %#v, want plus", converted["plan_type"])
	}
	if len(manager.List()) != 1 {
		t.Fatalf("expected one registered auth, got %d", len(manager.List()))
	}
}

func TestUploadAuthFile_Sub2APIContentGeneratesCPAName(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	content, err := json.Marshal(map[string]any{
		"name":     "sub2api@example.com",
		"platform": "openai",
		"type":     "oauth",
		"credentials": map[string]any{
			"access_token": "access-token",
			"plan_type":    "free",
		},
	})
	if err != nil {
		t.Fatalf("failed to encode sub2api content: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v0/management/auth-files?name=auth-content.json", bytes.NewReader(content))
	ctx.Request = req

	h.UploadAuthFile(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected upload status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	files, ok := payload["files"].([]any)
	if !ok || len(files) != 1 {
		t.Fatalf("expected one generated file, got %#v", payload["files"])
	}
	name, ok := files[0].(string)
	if !ok {
		t.Fatalf("generated file name has unexpected type: %#v", files[0])
	}
	if !strings.HasPrefix(name, "sub2api@example.com-free-") || !strings.HasSuffix(name, ".json") {
		t.Fatalf("unexpected generated filename %q", name)
	}
}
