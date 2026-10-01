package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestManagementV8LocalUsageRoutesRequireAuthentication(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "local-extension-test-key")
	server := newTestServer(t)
	for _, path := range []string{"/v8/management/observability/usage/token-usage", "/v8/management/observability/usage/statistics"} {
		for _, authorized := range []bool{false, true} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.RemoteAddr = "127.0.0.1:12345"
			want := http.StatusUnauthorized
			if authorized {
				req.Header.Set("Authorization", "Bearer local-extension-test-key")
				want = http.StatusOK
			}
			rr := httptest.NewRecorder()
			server.engine.ServeHTTP(rr, req)
			if rr.Code != want {
				t.Fatalf("%s authorized=%v: status=%d body=%s", path, authorized, rr.Code, rr.Body.String())
			}
		}
	}
}

func TestManagementV8CurrentBundleJavaScriptSyntax(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is optional for bundle syntax verification")
	}
	page, err := os.ReadFile(filepath.Join("..", "..", "static", "management.html"))
	if err != nil {
		t.Fatal(err)
	}
	page = injectManagementTokenUsageEntry(page)
	page = injectManagementAuthContentEntry(page)
	page = injectManagementAIProvidersEntry(page)
	page = injectManagementQuotaActionsEntry(page)
	if !strings.Contains(string(page), "window.__cliproxyManagementClient=") {
		t.Fatal("current bundle client was not detected")
	}
	scripts := regexp.MustCompile(`(?s)<script[^>]*>(.*?)</script>`).FindAllSubmatch(page, -1)
	if len(scripts) < 5 {
		t.Fatalf("expected native bundle and extension scripts, got %d", len(scripts))
	}
	path := filepath.Join(t.TempDir(), "script.mjs")
	for _, script := range scripts {
		if err := os.WriteFile(path, script[1], 0600); err != nil {
			t.Fatal(err)
		}
		if output, err := exec.Command(node, "--check", path).CombinedOutput(); err != nil {
			t.Fatalf("invalid injected JavaScript: %v\n%s", err, output)
		}
	}
}

func TestQuotaV8PatchIsAtomicAndIdempotent(t *testing.T) {
	page := "function quota(){let [p,m]=(0,R.useState)(1);let data=(0,R.useMemo)(()=>slice(items,p,20),[items,p]);return (0,J.jsx)(Select,{value:sort,options:choices,onChange:change,ariaLabel:t(`quota_management.sort_label`),size:`sm`})}"
	patched := patchManagementQuotaV8Page(page)
	if patched == page || !strings.Contains(patched, "[items,p,cliproxyQuotaPageSize]") {
		t.Fatal("v8 quota pagination not patched")
	}
	if patchManagementQuotaV8Page(patched) != patched {
		t.Fatal("patch is not idempotent")
	}
	missing := strings.Replace(page, "slice(items,p,20)", "slice(items,p,30)", 1)
	if patchManagementQuotaV8Page(missing) != missing {
		t.Fatal("unrecognized bundle was partially patched")
	}
}
