package management

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// PatchAuthFileStatus retains native single-credential updates and applies batch
// operations through the same persistence, locking and plugin-hook path.
func (h *Handler) PatchAuthFileStatus(c *gin.Context) {
	raw, errRead := c.GetRawData()
	if errRead != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	var req struct {
		Name         string   `json:"name"`
		Names        []string `json:"names"`
		Provider     string   `json:"provider"`
		StatusCode   *int     `json:"status_code"`
		Unauthorized *bool    `json:"unauthorized"`
		Disabled     *bool    `json:"disabled"`
	}
	if errJSON := json.Unmarshal(raw, &req); errJSON != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	hasFilter := strings.TrimSpace(req.Provider) != "" || req.StatusCode != nil || req.Unauthorized != nil
	if req.Disabled == nil || (len(req.Names) == 0 && (strings.TrimSpace(req.Name) != "" || !hasFilter)) {
		h.patchAuthFileStatusSingle(c)
		return
	}
	if h.authManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "core auth manager unavailable"})
		return
	}
	names := uniqueAuthFileNames(append([]string{req.Name}, req.Names...))
	if len(names) == 0 {
		filter := authFileMutationFilter{Provider: strings.ToLower(strings.TrimSpace(req.Provider)), Unauthorized: req.Unauthorized}
		if req.StatusCode != nil {
			if *req.StatusCode < 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status_code"})
				return
			}
			filter.StatusCode = *req.StatusCode
		}
		names = h.listAuthFileNamesByFilter(filter)
	}
	if len(names) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no matching auth files"})
		return
	}
	updated := make([]string, 0, len(names))
	failed := make([]gin.H, 0)
	for _, name := range names {
		body, errJSON := json.Marshal(gin.H{"name": name, "disabled": *req.Disabled})
		if errJSON != nil {
			failed = append(failed, gin.H{"name": name, "error": errJSON.Error()})
			continue
		}
		child := c.Copy()
		child.Request = c.Request.Clone(c.Request.Context())
		child.Request.Body = io.NopCloser(bytes.NewReader(body))
		child.Request.ContentLength = int64(len(body))
		writer := &authStatusBatchWriter{ResponseWriter: c.Writer, header: make(http.Header), status: http.StatusOK}
		child.Writer = writer
		h.patchAuthFileStatusSingle(child)
		if writer.status >= http.StatusBadRequest {
			var detail gin.H
			_ = json.Unmarshal(writer.body.Bytes(), &detail)
			failed = append(failed, gin.H{"name": name, "error": detail["error"], "status_code": writer.status})
			continue
		}
		updated = append(updated, name)
	}
	status, result := http.StatusOK, "ok"
	if len(failed) > 0 {
		status, result = http.StatusMultiStatus, "partial"
		if len(updated) == 0 {
			status, result = http.StatusNotFound, "error"
		}
	}
	c.JSON(status, gin.H{"status": result, "disabled": *req.Disabled, "updated": len(updated), "files": updated, "failed": failed})
}

// authStatusBatchWriter captures a child JSON response without writing to the client.
type authStatusBatchWriter struct {
	gin.ResponseWriter
	header  http.Header
	body    bytes.Buffer
	status  int
	written bool
}

func (w *authStatusBatchWriter) Header() http.Header    { return w.header }
func (w *authStatusBatchWriter) WriteHeader(status int) { w.status = status }
func (w *authStatusBatchWriter) WriteHeaderNow()        { w.written = true }
func (w *authStatusBatchWriter) Write(p []byte) (int, error) {
	w.written = true
	return w.body.Write(p)
}
func (w *authStatusBatchWriter) WriteString(s string) (int, error) {
	w.written = true
	return w.body.WriteString(s)
}
func (w *authStatusBatchWriter) Status() int   { return w.status }
func (w *authStatusBatchWriter) Size() int     { return w.body.Len() }
func (w *authStatusBatchWriter) Written() bool { return w.written }
