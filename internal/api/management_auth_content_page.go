package api

import "strings"

func injectManagementAuthContentEntry(data []byte) []byte {
	if len(data) == 0 {
		return []byte(managementAuthContentEntryScript)
	}
	html := string(data)
	if strings.Contains(html, "cliproxy-auth-content-entry") {
		return data
	}
	lower := strings.ToLower(html)
	index := strings.LastIndex(lower, "</body>")
	if index < 0 {
		return []byte(html + managementAuthContentEntryScript)
	}
	return []byte(html[:index] + managementAuthContentEntryScript + html[index:])
}

const managementAuthContentEntryScript = `<script id="cliproxy-auth-content-entry">
(function () {
  if (window.__cliproxyAuthContentEntryMounted) return;
  window.__cliproxyAuthContentEntryMounted = true;

  var route = "/auth-files";
  var panelId = "cliproxy-auth-content-panel";
  var updateTimer = null;

  function currentRoute() {
    return (window.location.hash || "").replace(/^#/, "").split("?")[0].replace(/\/$/, "");
  }

  function contentHost() {
    return document.querySelector(".main-content") || document.querySelector("main") || document.getElementById("root") || document.body;
  }

  function setStatus(panel, message, isError) {
    var status = panel.querySelector("[data-auth-content-status]");
    status.textContent = message || "";
    status.style.color = isError ? "var(--danger-color, #dc2626)" : "var(--text-secondary, #6b7280)";
  }

  function createPanel() {
    var panel = document.createElement("section");
    panel.id = panelId;
    panel.setAttribute("aria-label", "Create auth file");
    panel.innerHTML = '<div data-auth-content-panel-inner>' +
      '<div data-auth-content-heading><h2>\u586b\u5199\u8ba4\u8bc1\u5185\u5bb9</h2><span>\u5185\u5bb9\u4f1a\u6309\u4e0a\u4f20\u6587\u4ef6\u7684\u89c4\u5219\u68c0\u6d4b\u5e76\u751f\u6210 auth \u6587\u4ef6\uff0c\u6587\u4ef6\u540d\u4f1a\u6839\u636e\u8d26\u53f7\u3001\u7c7b\u578b\u548c\u5f53\u524d\u65f6\u95f4\u81ea\u52a8\u751f\u6210</span></div>' +
      '<form data-auth-content-form>' +
      '<label>\u6587\u4ef6\u540d<input data-auth-content-name type="text" value="auth-content.json" readonly autocomplete="off" spellcheck="false"></label>' +
      '<label>\u8ba4\u8bc1\u5185\u5bb9<textarea data-auth-content-value rows="8" placeholder="\u7c98\u8d34 JSON \u8ba4\u8bc1\u5185\u5bb9"></textarea></label>' +
      '<div data-auth-content-actions><button type="submit">\u751f\u6210 auth \u6587\u4ef6</button><span data-auth-content-status role="status"></span></div>' +
      '</form></div>';
    return panel;
  }

  function ensurePanel() {
    var host = contentHost();
    if (!host) return null;
    var panel = document.getElementById(panelId);
    if (!panel) {
      panel = createPanel();
      host.insertBefore(panel, host.firstChild);
      bindPanel(panel);
    } else if (panel.parentElement !== host) {
      host.insertBefore(panel, host.firstChild);
    }
    return panel;
  }

  function submitAsAuthFile(panel) {
    var nameInput = panel.querySelector("[data-auth-content-name]");
    var valueInput = panel.querySelector("[data-auth-content-value]");
    var name = (nameInput.value || "").trim();
    var value = valueInput.value || "";
    if (!name) {
      setStatus(panel, "\u8bf7\u8f93\u5165\u6587\u4ef6\u540d", true);
      nameInput.focus();
      return;
    }
    if (!/\.json$/i.test(name)) name += ".json";
    if (!value.trim()) {
      setStatus(panel, "\u8bf7\u8f93\u5165\u8ba4\u8bc1\u5185\u5bb9", true);
      valueInput.focus();
      return;
    }

    var fileInput = document.querySelector('input[type="file"][multiple]');
    if (!fileInput || typeof DataTransfer === "undefined") {
      setStatus(panel, "\u5f53\u524d\u9875\u9762\u672a\u51c6\u5907\u597d\u4e0a\u4f20\u63a7\u4ef6\uff0c\u8bf7\u5237\u65b0\u540e\u91cd\u8bd5", true);
      return;
    }
    var transfer = new DataTransfer();
    transfer.items.add(new File([value], name, { type: "application/json" }));
    fileInput.files = transfer.files;
    setStatus(panel, "\u6b63\u5728\u68c0\u6d4b\u5e76\u751f\u6210...", false);
    fileInput.dispatchEvent(new Event("change", { bubbles: true }));
    valueInput.value = "";
  }

  function bindPanel(panel) {
    panel.querySelector("[data-auth-content-form]").addEventListener("submit", function (event) {
      event.preventDefault();
      submitAsAuthFile(panel);
    });
  }

  function updateVisibility() {
    var active = currentRoute() === route;
    var panel = document.getElementById(panelId);
    if (!active) {
      if (panel) panel.remove();
      return;
    }
    panel = ensurePanel();
    if (!panel) return;
    if (panel.style.display !== "block") panel.style.display = "block";
  }

  function scheduleVisibilityUpdate() {
    if (updateTimer !== null) return;
    updateTimer = setTimeout(function () {
      updateTimer = null;
      updateVisibility();
    }, 80);
  }

  var style = document.createElement("style");
  style.textContent = "#" + panelId + "{margin:0 0 18px;padding:18px;border:1px solid var(--border-color,#e5e7eb);border-radius:12px;background:var(--bg-primary,#fff);box-shadow:0 8px 24px rgba(0,0,0,.06)}#" + panelId + " [data-auth-content-heading]{display:flex;align-items:baseline;justify-content:space-between;gap:16px;margin-bottom:14px}#" + panelId + " h2{margin:0;color:var(--text-primary,#111827);font-size:18px}#" + panelId + " [data-auth-content-heading] span,#" + panelId + " [data-auth-content-status]{color:var(--text-secondary,#6b7280);font-size:12px}#" + panelId + " form{display:grid;gap:12px}#" + panelId + " label{display:grid;gap:6px;color:var(--text-primary,#111827);font-size:13px;font-weight:600}#" + panelId + " input,#" + panelId + " textarea{box-sizing:border-box;width:100%;border:1px solid var(--border-color,#d1d5db);border-radius:8px;background:var(--bg-secondary,#f9fafb);color:var(--text-primary,#111827);padding:9px 10px;font:inherit;font-weight:400}#" + panelId + " textarea{resize:vertical;min-height:150px;font-family:ui-monospace,SFMono-Regular,Consolas,monospace;font-size:12px;line-height:1.5}#" + panelId + " [data-auth-content-actions]{display:flex;align-items:center;gap:12px;flex-wrap:wrap}#" + panelId + " button{border:0;border-radius:8px;background:var(--primary-color,#2563eb);color:var(--primary-contrast,#fff);padding:9px 14px;cursor:pointer;font:inherit;font-weight:600}#" + panelId + " button:hover{filter:brightness(.95)}@media(max-width:640px){#" + panelId + " [data-auth-content-heading]{display:block}#" + panelId + " [data-auth-content-heading] span{display:block;margin-top:5px}}";
  document.head.appendChild(style);

  window.addEventListener("hashchange", scheduleVisibilityUpdate);
  var observer = new MutationObserver(function () {
    if (currentRoute() === route || document.getElementById(panelId)) scheduleVisibilityUpdate();
  });
  observer.observe(document.documentElement, { childList: true, subtree: true });
  scheduleVisibilityUpdate();
})();
</script>`
