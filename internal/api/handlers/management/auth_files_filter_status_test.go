package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

func TestListAuthFiles_FiltersUnauthorizedAndPaginates(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	for _, tc := range []struct {
		name       string
		provider   string
		statusCode int
	}{
		{name: "codex-a.json", provider: "codex", statusCode: http.StatusUnauthorized},
		{name: "codex-b.json", provider: "codex", statusCode: http.StatusUnauthorized},
		{name: "codex-c.json", provider: "codex", statusCode: http.StatusTooManyRequests},
		{name: "claude-a.json", provider: "claude", statusCode: http.StatusUnauthorized},
	} {
		fullPath := filepath.Join(authDir, tc.name)
		if errWrite := os.WriteFile(fullPath, []byte(`{"type":"`+tc.provider+`"}`), 0o600); errWrite != nil {
			t.Fatalf("failed to seed auth file %s: %v", tc.name, errWrite)
		}
		record := &coreauth.Auth{
			ID:       tc.name,
			FileName: tc.name,
			Provider: tc.provider,
			Attributes: map[string]string{
				"path": fullPath,
			},
			Metadata: map[string]any{
				"type": tc.provider,
			},
			LastError: &coreauth.Error{
				Code:       "request_failed",
				Message:    "upstream failed",
				HTTPStatus: tc.statusCode,
			},
		}
		if _, errRegister := manager.Register(context.Background(), record); errRegister != nil {
			t.Fatalf("failed to register auth record %s: %v", tc.name, errRegister)
		}
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	rec := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(
		http.MethodGet,
		"/v0/management/auth-files?provider=codex&status_code=401&page=1&page_size=1",
		nil,
	)
	ginCtx.Request = req

	h.ListAuthFiles(ginCtx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if errUnmarshal := json.Unmarshal(rec.Body.Bytes(), &payload); errUnmarshal != nil {
		t.Fatalf("failed to decode list payload: %v", errUnmarshal)
	}
	filesRaw, ok := payload["files"].([]any)
	if !ok {
		t.Fatalf("expected files array, payload: %#v", payload)
	}
	if len(filesRaw) != 1 {
		t.Fatalf("expected paged files length 1, got %d", len(filesRaw))
	}
	fileEntry, ok := filesRaw[0].(map[string]any)
	if !ok {
		t.Fatalf("expected file entry object, got %#v", filesRaw[0])
	}
	if got := int(fileEntry["last_error_status_code"].(float64)); got != http.StatusUnauthorized {
		t.Fatalf("expected last_error_status_code=%d, got %d", http.StatusUnauthorized, got)
	}

	pagination, ok := payload["pagination"].(map[string]any)
	if !ok {
		t.Fatalf("expected pagination object, payload: %#v", payload)
	}
	if got := int(pagination["total"].(float64)); got != 2 {
		t.Fatalf("expected pagination total=2, got %d", got)
	}
	if got := int(pagination["page_size"].(float64)); got != 1 {
		t.Fatalf("expected pagination page_size=1, got %d", got)
	}
}

func TestPatchAuthFileStatus_FilterBatch(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	names := []string{"codex-a.json", "codex-b.json", "codex-ok.json"}
	for idx, name := range names {
		fullPath := filepath.Join(authDir, name)
		if errWrite := os.WriteFile(fullPath, []byte(`{"type":"codex"}`), 0o600); errWrite != nil {
			t.Fatalf("failed to seed auth file %s: %v", name, errWrite)
		}
		statusCode := 0
		if idx < 2 {
			statusCode = http.StatusUnauthorized
		}
		record := &coreauth.Auth{
			ID:       name,
			FileName: name,
			Provider: "codex",
			Attributes: map[string]string{
				"path": fullPath,
			},
			Metadata: map[string]any{
				"type": "codex",
			},
		}
		if statusCode > 0 {
			record.LastError = &coreauth.Error{HTTPStatus: statusCode, Message: "unauthorized"}
		}
		if _, errRegister := manager.Register(context.Background(), record); errRegister != nil {
			t.Fatalf("failed to register auth record %s: %v", name, errRegister)
		}
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPatch, "/v0/management/auth-files/status", strings.NewReader(`{"provider":"codex","status_code":401,"disabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req

	h.PatchAuthFileStatus(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected patch status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	for idx, name := range names {
		auth, ok := manager.GetByID(name)
		if !ok {
			t.Fatalf("expected auth %s to exist", name)
		}
		wantDisabled := idx < 2
		if auth.Disabled != wantDisabled {
			t.Fatalf("auth %s disabled=%v, want %v", name, auth.Disabled, wantDisabled)
		}
	}
}

func TestDeleteAuthFile_FilterBatchQuery(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)
	h.tokenStore = &memoryAuthStore{}

	for idx, name := range []string{"codex-a.json", "codex-b.json", "codex-ok.json"} {
		fullPath := filepath.Join(authDir, name)
		if errWrite := os.WriteFile(fullPath, []byte(`{"type":"codex"}`), 0o600); errWrite != nil {
			t.Fatalf("failed to seed auth file %s: %v", name, errWrite)
		}
		record := &coreauth.Auth{
			ID:       name,
			FileName: name,
			Provider: "codex",
			Attributes: map[string]string{
				"path": fullPath,
			},
			Metadata: map[string]any{
				"type": "codex",
			},
		}
		if idx < 2 {
			record.LastError = &coreauth.Error{HTTPStatus: http.StatusUnauthorized, Message: "unauthorized"}
		}
		if _, errRegister := manager.Register(context.Background(), record); errRegister != nil {
			t.Fatalf("failed to register auth record %s: %v", name, errRegister)
		}
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(
		http.MethodDelete,
		"/v0/management/auth-files?provider=codex&status_code=401&unauthorized=true",
		nil,
	)
	ctx.Request = req

	h.DeleteAuthFile(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected delete status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	for _, name := range []struct {
		fileName string
		exists   bool
	}{
		{fileName: "codex-a.json", exists: false},
		{fileName: "codex-b.json", exists: false},
		{fileName: "codex-ok.json", exists: true},
	} {
		_, errStat := os.Stat(filepath.Join(authDir, name.fileName))
		if name.exists && errStat != nil {
			t.Fatalf("expected auth file %s to remain, stat err: %v", name.fileName, errStat)
		}
		if !name.exists && !os.IsNotExist(errStat) {
			t.Fatalf("expected auth file %s to be removed, stat err: %v", name.fileName, errStat)
		}
	}
}
