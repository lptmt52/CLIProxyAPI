package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectManagementQuotaActionsEntry(t *testing.T) {
	page := `<html><body><script>var xO=e=>Math.min(30,Math.max(3,Math.round(e))),tM=9,nM=12;function RN(){let{t:e}=V(),[p,m]=(0,y.useState)(1),h=Yb();return $M(M,p,20),(0,H.jsx)(hc,{value:d,options:R,onChange:L,ariaLabel:e(` + "`quota_management.sort_label`" + `),size:` + "`sm`" + `})}var zN=</script></body></html>`
	first := string(injectManagementQuotaActionsEntry([]byte(page)))
	second := string(injectManagementQuotaActionsEntry([]byte(first)))
	if strings.Count(first, `id="cliproxy-quota-actions-entry"`) != 1 || strings.Count(second, `id="cliproxy-quota-actions-entry"`) != 1 {
		t.Fatal("quota actions entry was not injected exactly once")
	}
	for _, marker := range []string{`pageSizeOptions = [20, 30, 50, 100]`, `client.patch("/auth-files/status"`, `client.delete("/auth-files"`, `data-cliproxy-quota-actions`, `QuotaCard-module__actionRow___mzLCF`, `QuotaCard-module__actionPill___GQddc`, `\u505c\u7528`, `card.remove()`, `card.classList.add`, `actions.querySelectorAll("button")`, `[q,qq]=(0,y.useState)(50)`, `$M(M,p,q)`, `xO=e=>[20,30,50,100].includes`, `tM=50,nM=50`, "`\\u6bcf\\u9875`", "[20,30,50,100].map"} {
		if !strings.Contains(first, marker) {
			t.Fatalf("missing marker %q", marker)
		}
	}
	if strings.Contains(first, `window.location.reload()`) {
		t.Fatal("quota actions should update cards in place")
	}
}

func TestPatchManagementQuotaPageBundleCurrentAsset(t *testing.T) {
	assetPath := filepath.Join("..", "..", "static", "management.html")
	bundle, errRead := os.ReadFile(assetPath)
	if errRead != nil {
		t.Fatalf("read management asset: %v", errRead)
	}
	patched := patchManagementQuotaPageBundle(string(bundle))
	for _, marker := range []string{`[q,qq]=(0,y.useState)(50)`, `$M(M,p,q)`, "`\\u6bcf\\u9875`", "[20,30,50,100].map"} {
		if !strings.Contains(patched, marker) {
			t.Fatalf("current management asset was not patched with %q", marker)
		}
	}
}
