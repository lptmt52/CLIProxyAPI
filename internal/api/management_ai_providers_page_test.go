package api

import (
	"strings"
	"testing"
)

func TestInjectManagementAIProvidersEntry(t *testing.T) {
	t.Run("injects wrapper and entry once", func(t *testing.T) {
		page := `<html><head></head><body><script>void window.__cliproxyManagementClient</script><script type="module">var ap=new class{get(){return null}patch(){return null}},op={checkLatest:()=>ap.get('/latest-version')}</script></body></html>`
		first := string(injectManagementAIProvidersEntry([]byte(page)))
		second := string(injectManagementAIProvidersEntry([]byte(first)))

		if got := strings.Count(first, `id="cliproxy-ai-providers-entry"`); got != 1 {
			t.Fatalf("entry script count = %d, want 1", got)
		}
		if got := strings.Count(second, `id="cliproxy-ai-providers-entry"`); got != 1 {
			t.Fatalf("entry script count after reinjection = %d, want 1", got)
		}
		if !strings.Contains(first, `window.__cliproxyManagementClient=ap`) {
			t.Fatalf("management client wrapper is missing: %s", first)
		}
		if !strings.Contains(first, `target = "_blank"`) || !strings.Contains(first, `data-cliproxy-provider-copy`) {
			t.Fatalf("provider interactions are missing: %s", first)
		}
		if !strings.Contains(first, `var priority = Number(item.priority);`) || !strings.Contains(first, `data-cliproxy-provider-priority-header`) || !strings.Contains(first, `headerReference = nativeHeaders[3]`) || !strings.Contains(first, `decoratePriority(ensurePriorityCell(row), record)`) {
			t.Fatalf("provider priority column is missing: %s", first)
		}
		if strings.Contains(first, `priority.textContent = "优先级 " + record.priority`) {
			t.Fatalf("provider priority is still rendered beside the label: %s", first)
		}
		if !strings.Contains(first, `if (document.activeElement !== priority && priority.value !== value) priority.value = value;`) {
			t.Fatalf("provider priority rendering is not idempotent: %s", first)
		}
		if !strings.Contains(first, `if (!isActive() || enhanceTimer !== null) return;`) {
			t.Fatalf("provider enhancement scheduling is not bounded: %s", first)
		}
		for _, marker := range []string{
			`window.addEventListener("popstate", handleRouteChange);`,
			`window.addEventListener("cliproxy-route-change", handleRouteChange);`,
			`["pushState", "replaceState"].forEach(function (method)`,
			`scheduleRouteReload();`,
			`value: { priority: next }`,
			`showToast(`,
			`api.__cliproxyProviderPutWrapped`,
			`loadRecords(true);`,
		} {
			if !strings.Contains(first, marker) {
				t.Fatalf("provider route refresh handler is missing %q", marker)
			}
		}
	})

	t.Run("detects current minified client names", func(t *testing.T) {
		page := `<html><body><script type="module">var sp=new class{get(){return null}patch(){return null}},cp={checkLatest:()=>sp.get('/latest-version')}</script></body></html>`
		got := string(injectManagementAIProvidersEntry([]byte(page)))
		if !strings.Contains(got, `window.__cliproxyManagementClient=sp;var cp={checkLatest:()=>sp.get(`) {
			t.Fatalf("current management client was not detected: %s", got)
		}
	})
	t.Run("appends when body is absent", func(t *testing.T) {
		page := `<html><script type="module">var ap=new class{get(){return null}patch(){return null}},op={checkLatest:()=>ap.get('/latest-version')}</script>`
		got := string(injectManagementAIProvidersEntry([]byte(page)))
		if !strings.HasSuffix(got, managementAIProvidersEntryScript) {
			t.Fatalf("entry script was not appended without body: %s", got)
		}
	})

	t.Run("handles empty input", func(t *testing.T) {
		got := string(injectManagementAIProvidersEntry(nil))
		if got != managementAIProvidersEntryScript {
			t.Fatalf("empty input result does not equal entry script")
		}
	})
}
