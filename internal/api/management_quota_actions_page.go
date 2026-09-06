package api

import "strings"

func injectManagementQuotaActionsEntry(data []byte) []byte {
	if len(data) == 0 {
		return []byte(managementQuotaActionsEntryScript)
	}
	html := patchManagementQuotaPageBundle(string(data))
	if strings.Contains(html, "cliproxy-quota-actions-entry") {
		return data
	}
	lower := strings.ToLower(html)
	index := strings.LastIndex(lower, "</body>")
	if index < 0 {
		return []byte(html + managementQuotaActionsEntryScript)
	}
	return []byte(html[:index] + managementQuotaActionsEntryScript + html[index:])
}

func patchManagementQuotaPageBundle(html string) string {
	// The upstream panel is a minified bundle. Keep this scoped to its stable quota page markers.
	html = strings.Replace(html, ",[p,m]=(0,y.useState)(1),h=Yb()", ",[p,m]=(0,y.useState)(1),[q,qq]=(0,y.useState)(50),h=Yb()", 1)
	html = strings.Replace(html, "$M(M,p,20)", "$M(M,p,q)", 1)
	html = strings.Replace(html, "xO=e=>Math.min(30,Math.max(3,Math.round(e)))", "xO=e=>[20,30,50,100].includes(Math.round(e))?Math.round(e):50", 1)
	html = strings.Replace(html, "tM=9,nM=12", "tM=50,nM=50", 1)
	old := "(0,H.jsx)(hc,{value:d,options:R,onChange:L,ariaLabel:e(`quota_management.sort_label`),size:`sm`})"
	newValue := "(0,H.jsxs)(`div`,{style:{display:`flex`,gap:8,alignItems:`center`},children:[(0,H.jsx)(hc,{value:d,options:R,onChange:L,ariaLabel:e(`quota_management.sort_label`),size:`sm`}),(0,H.jsxs)(`label`,{style:{display:`flex`,gap:5,alignItems:`center`,fontSize:12},children:[`\\u6bcf\\u9875`,(0,H.jsx)(`select`,{value:q,onChange:e=>{let t=Number(e.target.value);[20,30,50,100].includes(t)&&(qq(t),m(1))},children:[20,30,50,100].map(e=>(0,H.jsx)(`option`,{value:e,children:e},e))})]})]})"
	return strings.Replace(html, old, newValue, 1)
}

const managementQuotaActionsEntryScript = `<script id="cliproxy-quota-actions-entry">
(function () {
  if (window.__cliproxyQuotaActionsEntryMounted) return;
  window.__cliproxyQuotaActionsEntryMounted = true;

  var pageSizeOptions = [20, 30, 50, 100];
  var busy = false;

  function route() {
    return (window.location.hash || "").replace(/^#/, "").split("?")[0].replace(/\/$/, "");
  }

  function api() { return window.__cliproxyManagementClient; }
  function text(element) { return element ? String(element.textContent || "").trim() : ""; }

  function authPageSize() {
    if (route() !== "/auth-files") return;
    var input = document.getElementById("auth-files-page-size");
    if (!input || input.getAttribute("data-cliproxy-page-size") === "1") return;
    var select = document.createElement("select");
    select.id = input.id + "-options";
    select.className = input.className;
    select.setAttribute("data-cliproxy-page-size", "1");
    pageSizeOptions.forEach(function (size) {
      var option = document.createElement("option");
      option.value = String(size);
      option.textContent = String(size);
      select.appendChild(option);
    });
    var current = Number(input.value);
    select.value = pageSizeOptions.indexOf(current) >= 0 ? String(current) : "50";
    select.addEventListener("change", function () {
      var value = Number(select.value);
      try {
        var state = JSON.parse(localStorage.getItem("authFiles.uiState") || "{}");
        state.pageSize = value;
        state.regularPageSize = value;
        state.compactPageSize = value;
        localStorage.setItem("authFiles.uiState", JSON.stringify(state));
      } catch (_) {}
      input.value = String(value);
      input.dispatchEvent(new Event("input", {bubbles: true}));
      input.dispatchEvent(new Event("change", {bubbles: true}));
    });
    input.setAttribute("data-cliproxy-page-size", "1");
    input.value = "50";
    input.style.display = "none";
    input.parentElement.appendChild(select);
    input.dispatchEvent(new Event("input", {bubbles: true}));
    input.dispatchEvent(new Event("change", {bubbles: true}));
  }

  function quotaCards() {
    if (route() !== "/quota") return;
    var cards = document.querySelectorAll('[class*="QuotaCard-module__card"]');
    Array.prototype.forEach.call(cards, function (card) {
      var nameNode = card.querySelector('[class*="QuotaCard-module__fileName"]');
      var name = text(nameNode);
      if (!name || card.querySelector("[data-cliproxy-quota-actions]")) return;
      var footer = card.querySelector('[class*="QuotaCard-module__actionRow"]');
      if (!footer) {
        footer = document.createElement("footer");
        footer.className = "QuotaCard-module__actionRow___mzLCF";
        card.appendChild(footer);
      }
      var actions = document.createElement("span");
      actions.setAttribute("data-cliproxy-quota-actions", "1");
      actions.className = "QuotaCard-module__actionRow___mzLCF";
      var actionButton = footer.querySelector('button[class*="QuotaCard-module__actionPill"]');
      var actionClassName = actionButton ? actionButton.className : "QuotaCard-module__actionPill___GQddc";
      actions.innerHTML = '<button type="button" class="' + actionClassName + '" data-cliproxy-quota-disable>\u505c\u7528</button><button type="button" class="' + actionClassName + '" data-cliproxy-quota-delete>\u5220\u9664</button>';
      footer.appendChild(actions);
      actions.querySelector("[data-cliproxy-quota-disable]").addEventListener("click", function () { mutate("disable", name, card, actions); });
      actions.querySelector("[data-cliproxy-quota-delete]").addEventListener("click", function () {
        if (window.confirm("\u786e\u5b9a\u8981\u5220\u9664\u8ba4\u8bc1\u6587\u4ef6\u201c" + name + "\u201d\u5417\uff1f")) mutate("delete", name, card, actions);
      });
    });
  }

  function mutate(kind, name, card, actions) {
    if (busy) return;
    var client = api();
    if (!client) return;
    busy = true;
    Array.prototype.forEach.call(actions.querySelectorAll("button"), function (button) { button.disabled = true; });
    var request = kind === "delete"
      ? client.delete("/auth-files", {data: {names: [name]}})
      : client.patch("/auth-files/status", {name: name, disabled: true});
    Promise.resolve(request).then(function () {
      if (kind === "delete") {
        card.remove();
        return;
      }
      card.classList.add("QuotaCard-module__cardDisabled");
      var disableButton = actions.querySelector("[data-cliproxy-quota-disable]");
      if (disableButton) disableButton.textContent = "\u5df2\u505c\u7528";
    }).catch(function (error) {
      window.alert((kind === "delete" ? "\u5220\u9664" : "\u505c\u7528") + "\u5931\u8d25\uff1a" + (error && error.message ? error.message : error));
      Array.prototype.forEach.call(actions.querySelectorAll("button"), function (button) { button.disabled = false; });
    }).finally(function () { busy = false; });
  }

  function sync() { authPageSize(); quotaCards(); }
  window.addEventListener("hashchange", sync);
  new MutationObserver(sync).observe(document.documentElement, {childList: true, subtree: true});
  sync();
})();
</script>`
