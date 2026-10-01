package management

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/auth/codex"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/util"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/watcher/synthesizer"
	coreauth "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/auth"
)

// Download single auth file by name
func (h *Handler) DownloadAuthFile(c *gin.Context) {
	name := strings.TrimSpace(c.Query("name"))
	if isUnsafeAuthFileName(name) {
		c.JSON(400, gin.H{"error": "invalid name"})
		return
	}
	if !strings.HasSuffix(strings.ToLower(name), ".json") {
		c.JSON(400, gin.H{"error": "name must end with .json"})
		return
	}
	full := filepath.Join(h.cfg.AuthDir, name)
	data, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(404, gin.H{"error": "file not found"})
		} else {
			c.JSON(500, gin.H{"error": fmt.Sprintf("failed to read file: %v", err)})
		}
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", name))
	c.Data(200, "application/json", data)
}

// Upload auth file: multipart or raw JSON with ?name=
func (h *Handler) UploadAuthFile(c *gin.Context) {
	if h.authManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "core auth manager unavailable"})
		return
	}
	ctx := c.Request.Context()

	fileHeaders, errMultipart := h.multipartAuthFileHeaders(c)
	if errMultipart != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid multipart form: %v", errMultipart)})
		return
	}
	if len(fileHeaders) == 1 {
		uploaded, errUpload := h.storeUploadedAuthFile(ctx, fileHeaders[0])
		if errUpload != nil {
			if errors.Is(errUpload, errAuthFileMustBeJSON) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "file must be .json or .txt"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": errUpload.Error()})
			return
		}
		resp := gin.H{"status": "ok", "uploaded": len(uploaded), "files": uploaded}
		c.JSON(http.StatusOK, resp)
		return
	}
	if len(fileHeaders) > 1 {
		uploaded := make([]string, 0, len(fileHeaders))
		failed := make([]gin.H, 0)
		for _, file := range fileHeaders {
			names, errUpload := h.storeUploadedAuthFile(ctx, file)
			if errUpload != nil {
				failureName := ""
				if file != nil {
					failureName = filepath.Base(file.Filename)
				}
				msg := errUpload.Error()
				if errors.Is(errUpload, errAuthFileMustBeJSON) {
					msg = "file must be .json or .txt"
				}
				failed = append(failed, gin.H{"name": failureName, "error": msg})
				continue
			}
			uploaded = append(uploaded, names...)
		}
		if len(failed) > 0 {
			c.JSON(http.StatusMultiStatus, gin.H{
				"status":   "partial",
				"uploaded": len(uploaded),
				"files":    uploaded,
				"failed":   failed,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "uploaded": len(uploaded), "files": uploaded})
		return
	}
	if c.ContentType() == "multipart/form-data" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no files uploaded"})
		return
	}
	name := strings.TrimSpace(c.Query("name"))
	if isUnsafeAuthFileName(name) {
		c.JSON(400, gin.H{"error": "invalid name"})
		return
	}
	if !isSupportedAuthFileName(name) {
		c.JSON(400, gin.H{"error": "name must end with .json or .txt"})
		return
	}
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": "failed to read body"})
		return
	}
	uploaded, err := h.storeUploadedAuthPayload(ctx, filepath.Base(name), data)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	resp := gin.H{"status": "ok", "uploaded": len(uploaded), "files": uploaded}
	c.JSON(200, resp)
}

// Delete auth files: single by name or all
func (h *Handler) DeleteAuthFile(c *gin.Context) {
	if h.authManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "core auth manager unavailable"})
		return
	}
	ctx := c.Request.Context()
	if all := c.Query("all"); all == "true" || all == "1" || all == "*" {
		entries, err := os.ReadDir(h.cfg.AuthDir)
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("failed to read auth dir: %v", err)})
			return
		}
		deleted := 0
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(strings.ToLower(name), ".json") {
				continue
			}
			full := filepath.Join(h.cfg.AuthDir, name)
			if !filepath.IsAbs(full) {
				if abs, errAbs := filepath.Abs(full); errAbs == nil {
					full = abs
				}
			}
			if err = os.Remove(full); err == nil {
				if errDel := h.deleteTokenRecord(ctx, full); errDel != nil {
					c.JSON(500, gin.H{"error": errDel.Error()})
					return
				}
				deleted++
				h.removeAuth(ctx, full)
			}
		}
		c.JSON(200, gin.H{"status": "ok", "deleted": deleted})
		return
	}

	names, errNames := requestedAuthFileNamesForDelete(c)
	if errNames != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errNames.Error()})
		return
	}
	if len(names) == 0 {
		filter, errFilter := authFileMutationFilterFromRequest(c)
		if errFilter != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": errFilter.Error()})
			return
		}
		names = h.listAuthFileNamesByFilter(filter)
	}
	if len(names) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid name"})
		return
	}
	if len(names) == 1 {
		if _, status, errDelete := h.deleteAuthFileByName(ctx, names[0]); errDelete != nil {
			c.JSON(status, gin.H{"error": errDelete.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}

	deletedFiles := make([]string, 0, len(names))
	failed := make([]gin.H, 0)
	for _, name := range names {
		deletedName, _, errDelete := h.deleteAuthFileByName(ctx, name)
		if errDelete != nil {
			failed = append(failed, gin.H{"name": name, "error": errDelete.Error()})
			continue
		}
		deletedFiles = append(deletedFiles, deletedName)
	}
	if len(failed) > 0 {
		c.JSON(http.StatusMultiStatus, gin.H{
			"status":  "partial",
			"deleted": len(deletedFiles),
			"files":   deletedFiles,
			"failed":  failed,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "deleted": len(deletedFiles), "files": deletedFiles})
}

func (h *Handler) multipartAuthFileHeaders(c *gin.Context) ([]*multipart.FileHeader, error) {
	if h == nil || c == nil || c.ContentType() != "multipart/form-data" {
		return nil, nil
	}
	form, err := c.MultipartForm()
	if err != nil {
		return nil, err
	}
	if form == nil || len(form.File) == 0 {
		return nil, nil
	}

	keys := make([]string, 0, len(form.File))
	for key := range form.File {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	headers := make([]*multipart.FileHeader, 0)
	for _, key := range keys {
		headers = append(headers, form.File[key]...)
	}
	return headers, nil
}

func (h *Handler) storeUploadedAuthFile(ctx context.Context, file *multipart.FileHeader) ([]string, error) {
	if file == nil {
		return nil, fmt.Errorf("no file uploaded")
	}
	name, errName := normalizeUploadedAuthFileName(file.Filename)
	if errName != nil {
		return nil, errAuthFileMustBeJSON
	}
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("failed to read uploaded file: %w", err)
	}
	return h.storeUploadedAuthPayload(ctx, name, data)
}

func (h *Handler) storeUploadedAuthPayload(ctx context.Context, name string, data []byte) ([]string, error) {
	entries, err := expandUploadedAuthPayload(name, data)
	if err != nil {
		return nil, err
	}
	uploaded := make([]string, 0, len(entries))
	for _, entry := range entries {
		if errWrite := h.writeAuthFile(ctx, entry.name, entry.data); errWrite != nil {
			return uploaded, errWrite
		}
		uploaded = append(uploaded, entry.name)
	}
	return uploaded, nil
}

func (h *Handler) writeAuthFile(ctx context.Context, name string, data []byte) error {
	dst := filepath.Join(h.cfg.AuthDir, filepath.Base(name))
	if !filepath.IsAbs(dst) {
		if abs, errAbs := filepath.Abs(dst); errAbs == nil {
			dst = abs
		}
	}
	auth, err := h.buildAuthFromFileData(dst, data)
	if err != nil {
		return err
	}
	if errWrite := os.WriteFile(dst, data, 0o600); errWrite != nil {
		return fmt.Errorf("failed to write file: %w", errWrite)
	}
	if err := h.upsertAuthRecord(ctx, auth); err != nil {
		return err
	}
	if h.postAuthPersistHook != nil {
		if errHook := h.postAuthPersistHook(ctx, auth); errHook != nil {
			return fmt.Errorf("post-auth persist hook failed: %w", errHook)
		}
	}
	return nil
}

type uploadedAuthPayload struct {
	name string
	data []byte
}

func expandUploadedAuthPayload(name string, data []byte) ([]uploadedAuthPayload, error) {
	var err error
	name, err = normalizeUploadedAuthFileName(name)
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty auth file")
	}

	autoName := isGeneratedAuthContentName(name)
	if trimmed[0] == '[' {
		var rawItems []json.RawMessage
		if err := json.Unmarshal(trimmed, &rawItems); err != nil {
			return nil, fmt.Errorf("invalid auth file array: %w", err)
		}
		return normalizeUploadedAuthItems(name, rawItems, autoName)
	}

	var object map[string]any
	if err := json.Unmarshal(trimmed, &object); err != nil {
		return nil, fmt.Errorf("invalid auth file: %w", err)
	}
	if object == nil {
		return nil, fmt.Errorf("auth file must contain a JSON object")
	}
	if rawItems, ok, err := sub2APIAccountItems(object); err != nil {
		return nil, err
	} else if ok {
		return normalizeUploadedAuthItems(name, rawItems, autoName)
	}

	normalized, err := normalizeUploadedAuthObject(trimmed, object)
	if err != nil {
		return nil, err
	}
	if autoName {
		name = generatedAuthFileName(normalized, "")
	}
	return []uploadedAuthPayload{{name: name, data: normalized}}, nil
}

func isSupportedAuthFileName(name string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(name)))
	return ext == ".json" || ext == ".txt"
}

func normalizeUploadedAuthFileName(name string) (string, error) {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" {
		return "", fmt.Errorf("auth file name is empty")
	}
	if !isSupportedAuthFileName(name) {
		return "", errAuthFileMustBeJSON
	}
	if strings.EqualFold(filepath.Ext(name), ".txt") {
		return strings.TrimSuffix(name, filepath.Ext(name)) + ".json", nil
	}
	return name, nil
}

func normalizeUploadedAuthItems(name string, rawItems []json.RawMessage, autoName bool) ([]uploadedAuthPayload, error) {
	if len(rawItems) == 0 {
		return nil, fmt.Errorf("auth file array is empty")
	}
	normalizedItems := make([][]byte, 0, len(rawItems))
	for _, raw := range rawItems {
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) == 0 || trimmed[0] != '{' {
			return nil, fmt.Errorf("auth file array elements must be JSON objects")
		}
		normalized, err := normalizeUploadedAuthObject(trimmed, nil)
		if err != nil {
			return nil, err
		}
		normalizedItems = append(normalizedItems, normalized)
	}
	if autoName {
		payloads := make([]uploadedAuthPayload, 0, len(normalizedItems))
		for i, item := range normalizedItems {
			suffix := ""
			if len(normalizedItems) > 1 {
				suffix = fmt.Sprintf("%03d", i+1)
			}
			payloads = append(payloads, uploadedAuthPayload{
				name: generatedAuthFileName(item, suffix),
				data: item,
			})
		}
		return payloads, nil
	}
	if len(normalizedItems) == 1 {
		return []uploadedAuthPayload{{name: name, data: normalizedItems[0]}}, nil
	}

	base := strings.TrimSuffix(name, filepath.Ext(name))
	if base == "" {
		base = "auth"
	}
	payloads := make([]uploadedAuthPayload, 0, len(normalizedItems))
	for i, item := range normalizedItems {
		payloads = append(payloads, uploadedAuthPayload{
			name: fmt.Sprintf("%s_%03d.json", base, i+1),
			data: item,
		})
	}
	return payloads, nil
}

func normalizeUploadedAuthObject(raw []byte, object map[string]any) ([]byte, error) {
	if object == nil {
		if err := json.Unmarshal(raw, &object); err != nil {
			return nil, fmt.Errorf("invalid auth file: %w", err)
		}
	}
	if errCPA := validateCPAAuthObject(object); errCPA == nil {
		return append([]byte(nil), raw...), nil
	} else if isChatGPTSessionAccount(object) {
		converted, errSession := convertChatGPTSessionAccount(object)
		if errSession != nil {
			return nil, fmt.Errorf("invalid CPA auth and invalid ChatGPT Session auth: %w", errSession)
		}
		convertedData, errMarshal := json.Marshal(converted)
		if errMarshal != nil {
			return nil, fmt.Errorf("failed to encode converted ChatGPT Session auth: %w", errMarshal)
		}
		return convertedData, nil
	} else if isSub2APIAccount(object) {
		converted, errSub2API := convertSub2APIAccount(object)
		if errSub2API != nil {
			return nil, fmt.Errorf("invalid CPA auth and invalid sub2api auth: %w", errSub2API)
		}
		convertedData, errMarshal := json.Marshal(converted)
		if errMarshal != nil {
			return nil, fmt.Errorf("failed to encode converted sub2api auth: %w", errMarshal)
		}
		return convertedData, nil
	} else {
		return nil, fmt.Errorf("invalid auth file: CPA format is invalid: %v", errCPA)
	}
}

func validateCPAAuthObject(object map[string]any) error {
	typeName, ok := object["type"].(string)
	if !ok || strings.TrimSpace(typeName) == "" {
		return fmt.Errorf("missing type")
	}
	switch strings.ToLower(strings.TrimSpace(typeName)) {
	case "oauth", "oauth2", "api-key", "api_key", "apikey", "bearer", "token", "session", "chatgpt-session", "chatgpt_session", "chatgpt":
		return fmt.Errorf("unsupported auth type %q", typeName)
	}
	if credentials, ok := object["credentials"].(map[string]any); ok && len(credentials) > 0 {
		return fmt.Errorf("credentials must be flattened")
	}
	return nil
}

func isGeneratedAuthContentName(name string) bool {
	return strings.EqualFold(filepath.Base(strings.TrimSpace(name)), "auth-content.json")
}

func isChatGPTSessionAccount(object map[string]any) bool {
	if object == nil {
		return false
	}
	typeName, _ := firstSub2APIString(object, "type", "provider", "platform")
	normalizedType := strings.ToLower(strings.TrimSpace(typeName))
	if normalizedType == "session" || normalizedType == "chatgpt-session" || normalizedType == "chatgpt_session" || normalizedType == "chatgpt" {
		return true
	}
	for _, key := range []string{
		"session_token", "sessionToken", "chatgpt_account_id", "chatgpt_plan_type",
	} {
		if _, ok := object[key]; ok {
			return true
		}
	}
	if _, ok := object["session"]; ok {
		return true
	}
	if _, ok := object["tokens"]; ok {
		if _, hasAccessToken := object["accessToken"]; hasAccessToken {
			return true
		}
	}
	return false
}

func convertChatGPTSessionAccount(account map[string]any) (map[string]any, error) {
	sources := []map[string]any{account}
	for _, key := range []string{"session", "tokens", "auth", "credentials", "user", "account", "profile"} {
		if nested := authNestedObject(account, key); nested != nil {
			sources = append(sources, nested)
		}
	}

	converted := make(map[string]any, len(account)+8)
	for key, value := range account {
		switch key {
		case "type", "provider", "platform", "session", "tokens", "auth", "credentials", "user", "account", "profile":
			continue
		default:
			converted[key] = value
		}
	}
	converted["type"] = "codex"
	copyAuthValueFromSources(converted, sources, "access_token", "access_token", "accessToken", "token", "session_token", "sessionToken")
	copyAuthValueFromSources(converted, sources, "refresh_token", "refresh_token", "refreshToken")
	copyAuthValueFromSources(converted, sources, "id_token", "id_token", "idToken")
	copyAuthValueFromSources(converted, sources, "account_id", "account_id", "accountId", "chatgpt_account_id", "chatgptAccountId", "user_id", "userId")
	copyAuthValueFromSources(converted, sources, "email", "email", "account_email", "userEmail", "username")
	copyAuthValueFromSources(converted, sources, "plan_type", "plan_type", "planType", "chatgpt_plan_type", "chatgptPlanType", "subscription_type", "subscription", "plan")

	if idToken, _ := converted["id_token"].(string); strings.TrimSpace(idToken) != "" {
		if claims, errParse := codex.ParseJWTToken(idToken); errParse == nil && claims != nil {
			if _, exists := converted["email"]; !exists && claims.GetUserEmail() != "" {
				converted["email"] = claims.GetUserEmail()
			}
			if _, exists := converted["account_id"]; !exists && claims.GetAccountID() != "" {
				converted["account_id"] = claims.GetAccountID()
			}
			if _, exists := converted["plan_type"]; !exists && claims.CodexAuthInfo.ChatgptPlanType != "" {
				converted["plan_type"] = claims.CodexAuthInfo.ChatgptPlanType
			}
		}
	}
	if accessToken, _ := converted["access_token"].(string); strings.TrimSpace(accessToken) == "" {
		return nil, fmt.Errorf("missing access token or session token")
	}
	return converted, nil
}

func authNestedObject(object map[string]any, key string) map[string]any {
	value, ok := object[key]
	if !ok {
		return nil
	}
	if nested, ok := value.(map[string]any); ok {
		return nested
	}
	if encoded, ok := value.(string); ok && strings.TrimSpace(encoded) != "" {
		var nested map[string]any
		if json.Unmarshal([]byte(encoded), &nested) == nil {
			return nested
		}
	}
	return nil
}

func copyAuthValueFromSources(dst map[string]any, sources []map[string]any, target string, aliases ...string) {
	if _, exists := dst[target]; exists {
		return
	}
	for _, source := range sources {
		if value, ok := firstAuthString(source, aliases...); ok {
			dst[target] = value
			return
		}
	}
}

func firstAuthString(object map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if value, ok := object[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), true
		}
	}
	return "", false
}

func generatedAuthFileName(data []byte, suffix string) string {
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		object = map[string]any{}
	}
	provider, _ := firstAuthString(object, "type", "provider")
	account, _ := firstAuthString(object, "email", "account_email", "username", "name", "account_id")
	planType, _ := firstAuthString(object, "plan_type", "planType", "chatgpt_plan_type", "subscription_type", "plan")
	if provider == "codex" {
		if idToken, _ := firstAuthString(object, "id_token", "idToken"); idToken != "" {
			if claims, errParse := codex.ParseJWTToken(idToken); errParse == nil && claims != nil {
				if account == "" {
					account = claims.GetUserEmail()
					if account == "" {
						account = claims.GetAccountID()
					}
				}
				if planType == "" {
					planType = claims.CodexAuthInfo.ChatgptPlanType
				}
			}
		}
	}
	if account == "" {
		account = "auth"
	}
	if planType == "" {
		planType = provider
	}
	account = sanitizeAuthFileNamePart(account)
	planType = sanitizeAuthFileNamePart(planType)
	if account == "" {
		account = "auth"
	}
	if planType == "" {
		planType = "unknown"
	}
	suffix = sanitizeAuthFileNamePart(suffix)
	if suffix != "" {
		suffix = "-" + suffix
	}
	return fmt.Sprintf("%s-%s-%s%s.json", account, planType, time.Now().Format("20060102_150405_000"), suffix)
}

func sanitizeAuthFileNamePart(value string) string {
	var builder strings.Builder
	for _, r := range strings.TrimSpace(value) {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '@' || r == '.' || r == '_' || r == '-' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('_')
		}
	}
	return strings.Trim(builder.String(), "._-")
}

func isSub2APIAccount(object map[string]any) bool {
	if _, ok := firstSub2APIString(object, "platform", "provider"); ok {
		return true
	}
	typeName, _ := firstSub2APIString(object, "type")
	if isSub2APIAuthType(typeName) {
		_, hasCredentials := object["credentials"]
		return hasCredentials
	}
	_, hasCredentials := object["credentials"]
	if hasCredentials {
		_, isPlatform := mapSub2APIPlatform(typeName)
		return isPlatform
	}
	_, isPlatform := mapSub2APIPlatform(typeName)
	return isPlatform
}

func isSub2APIAuthType(typeName string) bool {
	switch strings.ToLower(strings.TrimSpace(typeName)) {
	case "oauth", "oauth2", "api-key", "api_key", "apikey", "bearer", "token":
		return true
	default:
		return false
	}
}

func sub2APIAccountItems(object map[string]any) ([]json.RawMessage, bool, error) {
	raw, ok := object["accounts"]
	if !ok {
		return nil, false, nil
	}
	data, errMarshal := json.Marshal(raw)
	if errMarshal != nil {
		return nil, true, fmt.Errorf("invalid sub2api accounts: %w", errMarshal)
	}
	var items []json.RawMessage
	if errUnmarshal := json.Unmarshal(data, &items); errUnmarshal != nil {
		return nil, true, fmt.Errorf("invalid sub2api accounts: %w", errUnmarshal)
	}
	return items, true, nil
}

func convertSub2APIAccount(account map[string]any) (map[string]any, error) {
	platform, _ := firstSub2APIString(account, "platform", "provider")
	if platform == "" {
		return nil, fmt.Errorf("missing platform")
	}
	cpaType, ok := mapSub2APIPlatform(platform)
	if !ok {
		return nil, fmt.Errorf("unsupported platform %q", platform)
	}

	converted := make(map[string]any, len(account)+8)
	for key, value := range account {
		if key == "platform" || key == "provider" || key == "credentials" {
			continue
		}
		converted[key] = value
	}
	converted["type"] = cpaType

	if credentials := sub2APICredentials(account); credentials != nil {
		for key, value := range credentials {
			if _, exists := converted[key]; !exists {
				converted[key] = value
			}
		}
		copySub2APIAlias(converted, credentials, "access_token", "accessToken", "token")
		copySub2APIAlias(converted, credentials, "refresh_token", "refreshToken")
		copySub2APIAlias(converted, credentials, "id_token", "idToken")
		copySub2APIAlias(converted, credentials, "account_id", "accountId")
		copySub2APIAlias(converted, credentials, "plan_type", "plan_type", "planType", "chatgpt_plan_type", "chatgptPlanType", "plan")
		copySub2APIAlias(converted, credentials, "api_key", "apiKey", "key")
	}
	if config, ok := account["config"].(map[string]any); ok {
		copySub2APIAlias(converted, config, "proxy_url", "proxyUrl")
		copySub2APIAlias(converted, config, "priority")
		copySub2APIAlias(converted, config, "disabled")
		copySub2APIAlias(converted, config, "note")
	}
	if email, ok := firstSub2APIString(account, "email", "account_email", "username", "name"); ok {
		if _, exists := converted["email"]; !exists {
			converted["email"] = email
		}
	}
	if _, exists := converted["email"]; !exists {
		if credentials := sub2APICredentials(account); credentials != nil {
			if email, ok := firstSub2APIString(credentials, "email", "account_email", "username"); ok {
				converted["email"] = email
			}
		}
	}
	if _, exists := converted["proxy_url"]; !exists {
		copySub2APIAlias(converted, account, "proxy_url", "proxyUrl")
	}
	return converted, nil
}

func sub2APICredentials(account map[string]any) map[string]any {
	raw, ok := account["credentials"]
	if !ok {
		return nil
	}
	if credentials, ok := raw.(map[string]any); ok {
		return credentials
	}
	encoded, ok := raw.(string)
	if !ok || strings.TrimSpace(encoded) == "" {
		return nil
	}
	var credentials map[string]any
	if err := json.Unmarshal([]byte(encoded), &credentials); err != nil {
		return nil
	}
	return credentials
}

func copySub2APIAlias(dst, src map[string]any, target string, aliases ...string) {
	if _, exists := dst[target]; exists {
		return
	}
	for _, alias := range aliases {
		if value, ok := src[alias]; ok {
			dst[target] = value
			return
		}
	}
}

func firstSub2APIString(object map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if value, ok := object[key].(string); ok {
			if value = strings.TrimSpace(value); value != "" {
				return value, true
			}
		}
	}
	return "", false
}

func mapSub2APIPlatform(platform string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(platform))
	normalized = strings.NewReplacer("_", "-", " ", "-").Replace(normalized)
	switch normalized {
	case "openai", "chatgpt", "codex", "gpt", "openai-codex", "openai-responses":
		return "codex", true
	case "claude", "anthropic":
		return "claude", true
	case "gemini", "google", "google-gemini":
		return "gemini", true
	case "gemini-cli", "google-gemini-cli":
		return "gemini-cli", true
	case "antigravity":
		return "antigravity", true
	case "kimi":
		return "kimi", true
	case "xai", "grok":
		return "xai", true
	case "vertex", "vertex-ai", "google-vertex":
		return "vertex", true
	case "openai-compatible", "openai-compat", "openai-compatibility":
		return "openai-compatibility", true
	default:
		return "", false
	}
}

func requestedAuthFileNamesForDelete(c *gin.Context) ([]string, error) {
	if c == nil {
		return nil, nil
	}
	names := uniqueAuthFileNames(c.QueryArray("name"))
	if len(names) > 0 {
		return names, nil
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body")
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil, nil
	}

	var objectBody struct {
		Name         string   `json:"name"`
		Names        []string `json:"names"`
		Provider     string   `json:"provider"`
		StatusCode   *int     `json:"status_code"`
		Unauthorized *bool    `json:"unauthorized"`
	}
	if body[0] == '[' {
		var arrayBody []string
		if err := json.Unmarshal(body, &arrayBody); err != nil {
			return nil, fmt.Errorf("invalid request body")
		}
		return uniqueAuthFileNames(arrayBody), nil
	}
	if err := json.Unmarshal(body, &objectBody); err != nil {
		return nil, fmt.Errorf("invalid request body")
	}

	out := make([]string, 0, len(objectBody.Names)+1)
	if strings.TrimSpace(objectBody.Name) != "" {
		out = append(out, objectBody.Name)
	}
	out = append(out, objectBody.Names...)
	return uniqueAuthFileNames(out), nil
}

func authFileMutationFilterFromRequest(c *gin.Context) (authFileMutationFilter, error) {
	filter := authFileMutationFilter{
		Provider: strings.ToLower(strings.TrimSpace(c.Query("provider"))),
	}
	if raw := strings.TrimSpace(c.Query("status_code")); raw != "" {
		statusCode, errAtoi := strconv.Atoi(raw)
		if errAtoi != nil || statusCode < 0 {
			return filter, fmt.Errorf("invalid status_code")
		}
		filter.StatusCode = statusCode
	}
	if raw := strings.TrimSpace(c.Query("unauthorized")); raw != "" {
		unauthorized, errParse := strconv.ParseBool(raw)
		if errParse != nil {
			return filter, fmt.Errorf("invalid unauthorized")
		}
		filter.Unauthorized = &unauthorized
	}

	if c.Request == nil || c.Request.Body == nil {
		return filter, nil
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return filter, fmt.Errorf("failed to read body")
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	body = bytes.TrimSpace(body)
	if len(body) == 0 || body[0] == '[' {
		return filter, nil
	}

	var objectBody struct {
		Provider     string `json:"provider"`
		StatusCode   *int   `json:"status_code"`
		Unauthorized *bool  `json:"unauthorized"`
	}
	if err := json.Unmarshal(body, &objectBody); err != nil {
		return filter, nil
	}
	if provider := strings.ToLower(strings.TrimSpace(objectBody.Provider)); provider != "" {
		filter.Provider = provider
	}
	if objectBody.StatusCode != nil {
		filter.StatusCode = *objectBody.StatusCode
	}
	if objectBody.Unauthorized != nil {
		filter.Unauthorized = objectBody.Unauthorized
	}
	return filter, nil
}

func uniqueAuthFileNames(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(names))
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func (h *Handler) deleteAuthFileByName(ctx context.Context, name string) (string, int, error) {
	name = strings.TrimSpace(name)
	if isUnsafeAuthFileName(name) {
		return "", http.StatusBadRequest, fmt.Errorf("invalid name")
	}

	targetPath := filepath.Join(h.cfg.AuthDir, filepath.Base(name))
	targetID := ""
	if targetAuth := h.findAuthForDelete(name); targetAuth != nil {
		if !isPluginVirtualSourceDelete(name, targetAuth) {
			return filepath.Base(name), http.StatusConflict, errPluginVirtualAuth
		}
		targetID = strings.TrimSpace(targetAuth.ID)
		if path := strings.TrimSpace(authAttribute(targetAuth, "path")); path != "" {
			targetPath = path
		}
	}
	if !filepath.IsAbs(targetPath) {
		if abs, errAbs := filepath.Abs(targetPath); errAbs == nil {
			targetPath = abs
		}
	}
	if errRemove := os.Remove(targetPath); errRemove != nil {
		if os.IsNotExist(errRemove) {
			return filepath.Base(name), http.StatusNotFound, errAuthFileNotFound
		}
		return filepath.Base(name), http.StatusInternalServerError, fmt.Errorf("failed to remove file: %w", errRemove)
	}
	if errDeleteRecord := h.deleteTokenRecord(ctx, targetPath); errDeleteRecord != nil {
		return filepath.Base(name), http.StatusInternalServerError, errDeleteRecord
	}
	h.removeAuthsForPath(ctx, targetPath, targetID)
	return filepath.Base(name), http.StatusOK, nil
}

func isPluginVirtualSourceDelete(name string, auth *coreauth.Auth) bool {
	if !coreauth.IsPluginVirtualAuth(auth) {
		return true
	}
	sourcePath := strings.TrimSpace(authAttribute(auth, coreauth.AttributeVirtualSource))
	if sourcePath == "" {
		sourcePath = strings.TrimSpace(authAttribute(auth, "path"))
	}
	if sourcePath == "" {
		return false
	}
	return strings.EqualFold(filepath.Base(strings.TrimSpace(name)), filepath.Base(sourcePath))
}

func (h *Handler) findAuthForDelete(name string) *coreauth.Auth {
	if h == nil || h.authManager == nil {
		return nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if auth, ok := h.authManager.GetByID(name); ok {
		return auth
	}
	auths := h.authManager.List()
	for _, auth := range auths {
		if auth == nil {
			continue
		}
		if strings.TrimSpace(auth.FileName) == name {
			return auth
		}
		if filepath.Base(strings.TrimSpace(authAttribute(auth, "path"))) == name {
			return auth
		}
	}
	return nil
}

func (h *Handler) authIDForPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		if abs, errAbs := filepath.Abs(path); errAbs == nil {
			path = abs
		}
	}
	id := path
	if h != nil && h.cfg != nil {
		authDir := strings.TrimSpace(h.cfg.AuthDir)
		if resolvedAuthDir, errResolve := util.ResolveAuthDir(authDir); errResolve == nil && resolvedAuthDir != "" {
			authDir = resolvedAuthDir
		}
		if authDir != "" {
			authDir = filepath.Clean(authDir)
			if !filepath.IsAbs(authDir) {
				if abs, errAbs := filepath.Abs(authDir); errAbs == nil {
					authDir = abs
				}
			}
			if rel, errRel := filepath.Rel(authDir, path); errRel == nil && rel != "" {
				id = rel
			}
		}
	}
	// On Windows, normalize ID casing to avoid duplicate auth entries caused by case-insensitive paths.
	if runtime.GOOS == "windows" {
		id = strings.ToLower(id)
	}
	return id
}

func (h *Handler) registerAuthFromFile(ctx context.Context, path string, data []byte) error {
	if h.authManager == nil {
		return nil
	}
	auth, err := h.buildAuthFromFileData(path, data)
	if err != nil {
		return err
	}
	return h.upsertAuthRecord(ctx, auth)
}

func (h *Handler) buildAuthFromFileData(path string, data []byte) (*coreauth.Auth, error) {
	if path == "" {
		return nil, fmt.Errorf("auth path is empty")
	}
	if data == nil {
		var err error
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read auth file: %w", err)
		}
	}
	metadata := make(map[string]any)
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("invalid auth file: %w", err)
	}
	coreauth.NormalizeCredentialMetadata(metadata)
	provider, _ := metadata["type"].(string)
	if provider == "" {
		provider = "unknown"
	}
	label := provider
	if email, ok := metadata["email"].(string); ok && email != "" {
		label = email
	}
	lastRefresh, hasLastRefresh := extractLastRefreshTimestamp(metadata)

	authID := h.authIDForPath(path)
	if authID == "" {
		authID = path
	}
	auth := (*coreauth.Auth)(nil)
	if h != nil && h.cfg != nil {
		sctx := &synthesizer.SynthesisContext{
			Config:      h.cfg,
			AuthDir:     h.cfg.AuthDir,
			Now:         time.Now(),
			IDGenerator: synthesizer.NewStableIDGenerator(),
		}
		generated, errSynthesize := synthesizer.SynthesizeAuthFile(sctx, path, data)
		if errSynthesize != nil {
			return nil, fmt.Errorf("invalid auth file: %w", errSynthesize)
		}
		if len(generated) > 0 && generated[0] != nil {
			auth = generated[0].Clone()
		}
	}
	if auth == nil {
		auth = &coreauth.Auth{
			ID:       authID,
			Provider: provider,
			Label:    label,
			Status:   coreauth.StatusActive,
			Attributes: map[string]string{
				"path":   path,
				"source": path,
			},
			Metadata:  metadata,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}
	auth.ID = authID
	auth.FileName = filepath.Base(path)
	if hasLastRefresh {
		auth.LastRefreshedAt = lastRefresh
	}
	if h != nil && h.authManager != nil {
		if existing, ok := h.authManager.GetByID(authID); ok {
			auth.CreatedAt = existing.CreatedAt
			if !hasLastRefresh {
				auth.LastRefreshedAt = existing.LastRefreshedAt
			}
			auth.NextRefreshAfter = existing.NextRefreshAfter
			auth.Runtime = existing.Runtime
		}
	}
	coreauth.ApplyCustomHeadersFromMetadata(auth)
	return auth, nil
}

func (h *Handler) upsertAuthRecord(ctx context.Context, auth *coreauth.Auth) error {
	if h == nil || h.authManager == nil || auth == nil {
		return nil
	}
	if existing, ok := h.authManager.GetByID(auth.ID); ok {
		auth.CreatedAt = existing.CreatedAt
		_, err := h.authManager.Update(ctx, auth)
		return err
	}
	_, err := h.authManager.Register(ctx, auth)
	return err
}
