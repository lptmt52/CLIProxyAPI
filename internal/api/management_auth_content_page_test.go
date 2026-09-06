package api

import (
	"strings"
	"testing"
)

func TestInjectManagementAuthContentEntryUsesBoundedVisibilityUpdates(t *testing.T) {
	page := `<html><head></head><body><div id="root"></div></body></html>`
	first := string(injectManagementAuthContentEntry([]byte(page)))
	second := string(injectManagementAuthContentEntry([]byte(first)))

	if got := strings.Count(first, `id="cliproxy-auth-content-entry"`); got != 1 {
		t.Fatalf("entry script count = %d, want 1", got)
	}
	if got := strings.Count(second, `id="cliproxy-auth-content-entry"`); got != 1 {
		t.Fatalf("entry script count after reinjection = %d, want 1", got)
	}
	if !strings.Contains(first, `if (!active) {`) {
		t.Fatalf("inactive routes still create the auth content panel: %s", first)
	}
	if !strings.Contains(first, `if (updateTimer !== null) return;`) {
		t.Fatalf("auth content visibility updates are not bounded: %s", first)
	}
	if strings.Contains(first, `new MutationObserver(updateVisibility)`) {
		t.Fatalf("auth content observer still invokes DOM updates directly: %s", first)
	}
}
