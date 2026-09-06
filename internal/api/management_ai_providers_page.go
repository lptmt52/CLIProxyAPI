package api

import (
	"regexp"
	"strings"
)

var managementClientPattern = regexp.MustCompile("([;}]),([A-Za-z_$][A-Za-z0-9_$]*)=\\{checkLatest:\\(\\)=>([A-Za-z_$][A-Za-z0-9_$]*)\\.get\\(")

func injectManagementAIProvidersEntry(data []byte) []byte {
	if len(data) == 0 {
		return []byte(managementAIProvidersEntryScript)
	}
	html := string(data)
	html = injectManagementClient(html)
	if strings.Contains(html, "cliproxy-ai-providers-entry") {
		return []byte(html)
	}
	lower := strings.ToLower(html)
	index := strings.LastIndex(lower, "</body>")
	if index < 0 {
		return []byte(html + managementAIProvidersEntryScript)
	}
	return []byte(html[:index] + managementAIProvidersEntryScript + html[index:])
}

func injectManagementClient(html string) string {
	if strings.Contains(html, "window.__cliproxyManagementClient=") {
		return html
	}
	matches := managementClientPattern.FindStringSubmatchIndex(html)
	if len(matches) < 8 {
		return html
	}
	matched := html[matches[0]:matches[1]]
	client := html[matches[6]:matches[7]]
	replacement := matched[:1] + ";window.__cliproxyManagementClient=" + client + ";var " + matched[2:]
	return html[:matches[0]] + replacement + html[matches[1]:]
}

const managementAIProvidersEntryScript = `<script id="cliproxy-ai-providers-entry">
(function () {
  if (window.__cliproxyAIProvidersEntryMounted) return;
  window.__cliproxyAIProvidersEntryMounted = true;

  var route = "/ai-providers";
  var defaultLabel = "可用";
  var records = [];
  var recordsReady = false;
  var loading = false;
  var lastNativeSignature = "";
  var enhanceTimer = null;
  var reloadQueued = false;
  var routeReloadTimer = null;
  var toastTimer = null;
  var clientMutationWrapped = false;

  var specs = [
    { brand: "gemini", endpoint: "/gemini-api-key", field: "gemini-api-key" },
    { brand: "codex", endpoint: "/codex-api-key", field: "codex-api-key" },
    { brand: "xai", endpoint: "/xai-api-key", field: "xai-api-key" },
    { brand: "claude", endpoint: "/claude-api-key", field: "claude-api-key" },
    { brand: "vertex", endpoint: "/vertex-api-key", field: "vertex-api-key" },
    { brand: "openaiCompatibility", endpoint: "/openai-compatibility", field: "openai-compatibility" }
  ];

  function currentRoute() {
    var hash = (window.location.hash || "").replace(/^#/, "").split("?")[0].replace(/\/$/, "");
    if (hash) return hash;
    return (window.location.pathname || "").replace(/\/$/, "");
  }

  function isActive() {
    return currentRoute() === route;
  }

  function textOf(element) {
    return element ? String(element.textContent || "").trim() : "";
  }

  function maskKey(value) {
    var key = String(value || "").trim();
    if (!key) return "";
    var edge = key.length < 4 ? 1 : 2;
    return key.slice(0, edge) + new Array(Math.max(10 - edge * 2, 1) + 1).join("*") + key.slice(-edge);
  }

  function classElement(root, fragment) {
    return root && root.querySelector("[class*=\"" + fragment + "\"]");
  }

  function panelBrand(row) {
    var panel = row.closest("[class*=\"ProviderResourcePanel-module__panel\"]");
    var heading = panel && panel.querySelector("h2");
    var value = textOf(heading).toLowerCase();
    if (value.indexOf("gemini") >= 0) return "gemini";
    if (value.indexOf("codex") >= 0) return "codex";
    if (value.indexOf("xai") >= 0 || value.indexOf("grok") >= 0) return "xai";
    if (value.indexOf("claude") >= 0 || value.indexOf("anthropic") >= 0) return "claude";
    if (value.indexOf("vertex") >= 0) return "vertex";
    if (value.indexOf("openai") >= 0 || value.indexOf("兼容") >= 0) return "openaiCompatibility";
    return "";
  }

  function normalizeURL(value) {
    var raw = String(value || "").trim().replace(/[),.;]+$/, "");
    var marker = raw.indexOf("://");
    if (marker < 0) return "";
    var rest = raw.slice(marker + 3);
    var cut = rest.search(/[/?#]/);
    if (cut < 0) cut = rest.length;
    var origin = raw.slice(0, marker + 3 + cut);
    return origin.replace(/\/$/, "");
  }

  function findURL(value) {
    var match = String(value || "").match(/[a-z][a-z0-9+.-]*:\/\/[^\s]+/i);
    return match ? match[0].replace(/[),.;]+$/, "") : "";
  }

  function copyText(value) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      return navigator.clipboard.writeText(value);
    }
    return new Promise(function (resolve, reject) {
      var input = document.createElement("textarea");
      input.value = value;
      input.style.position = "fixed";
      input.style.opacity = "0";
      document.body.appendChild(input);
      input.focus();
      input.select();
      try {
        document.execCommand("copy") ? resolve() : reject(new Error("copy failed"));
      } catch (error) {
        reject(error);
      } finally {
        input.remove();
      }
    });
  }

  function copyIcon() {
    return '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="9" y="9" width="11" height="11" rx="2"></rect><path d="M15 9V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v7a2 2 0 0 0 2 2h3"></path></svg>';
  }

  function checkIcon() {
    return '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m5 12 4 4L19 6"></path></svg>';
  }

  function client() {
    return window.__cliproxyManagementClient;
  }

  function recordsFromResponse(spec, response) {
    var values = response && response[spec.field];
    if (!Array.isArray(values)) return [];
    return values.map(function (item, index) {
      item = item && typeof item === "object" ? item : {};
      var keys = [];
      var priority = Number(item.priority);
      if (!isFinite(priority)) priority = 0;
      if (spec.brand === "openaiCompatibility") {
        keys = Array.isArray(item["api-key-entries"]) ? item["api-key-entries"].map(function (entry) {
          return String(entry && entry["api-key"] || "").trim();
        }).filter(Boolean) : [];
      } else if (item["api-key"]) {
        keys = [String(item["api-key"]).trim()];
      }
      return {
        brand: spec.brand,
        endpoint: spec.endpoint,
        index: index,
        name: String(item.name || "").trim(),
        key: keys[0] || "",
        keys: keys,
        preview: maskKey(keys[0]),
        baseUrl: String(item["base-url"] || "").trim(),
        label: String(item.label || "").trim(),
        priority: priority
      };
    });
  }

  function installClientMutationRefresh() {
    var api = client();
    if (!api || typeof api.put !== "function" || clientMutationWrapped || api.__cliproxyProviderPutWrapped) return;
    var originalPut = api.put;
    api.put = function () {
      var result = originalPut.apply(this, arguments);
      if (result && typeof result.then === "function") {
        result.then(function () {
          if (!isActive()) return;
          records = [];
          recordsReady = false;
          loadRecords(true);
          scheduleEnhance();
        }, function () {});
      }
      return result;
    };
    api.__cliproxyProviderPutWrapped = true;
    clientMutationWrapped = true;
  }

  function loadRecords(force) {
    if (!isActive()) return;
    var api = client();
    if (!api || typeof api.get !== "function") return;
    installClientMutationRefresh();
    if (loading) {
      if (force) reloadQueued = true;
      return;
    }
    loading = true;
    Promise.all(specs.map(function (spec) {
      return api.get(spec.endpoint).then(function (response) {
        return { spec: spec, response: response };
      }).catch(function () {
        return null;
      });
    })).then(function (results) {
      var next = [];
      var successCount = 0;
      results.forEach(function (result) {
        if (!result) return;
        successCount++;
        next = next.concat(recordsFromResponse(result.spec, result.response));
      });
      if (successCount > 0) records = next;
    }).finally(function () {
      recordsReady = true;
      loading = false;
      enhance();
      if (reloadQueued && isActive()) {
        reloadQueued = false;
        setTimeout(function () { loadRecords(true); }, 0);
      }    });
  }

  function nativeSignature() {
    var tables = document.querySelectorAll("table[class*=\"ProviderResourceTable-module__providerTable\"]");
    var parts = [];
    Array.prototype.forEach.call(tables, function (table) {
      Array.prototype.forEach.call(table.querySelectorAll("tbody tr"), function (row) {
        var cells = row.querySelectorAll("td");
        if (cells.length < 2) return;
        var primaryName = classElement(cells[0], "ProviderResourceTable-module__primaryName");
        var primarySub = classElement(cells[0], "ProviderResourceTable-module__primarySub");
        parts.push(textOf(primaryName) + "|" + textOf(primarySub) + "|" + textOf(cells[1]));
      });
    });
    return parts.join("\n");
  }

  function findRecord(row) {
    var cells = row.querySelectorAll("td");
    if (cells.length < 2) return null;
    var primaryName = classElement(cells[0], "ProviderResourceTable-module__primaryName");
    var primarySub = classElement(cells[0], "ProviderResourceTable-module__primarySub");
    var nameText = textOf(primaryName);
    var subText = textOf(primarySub);
    var baseText = textOf(cells[1]);
    var brand = panelBrand(row);
    var best = null;
    var bestScore = 0;

    records.forEach(function (record) {
      var score = 0;
      if (brand && record.brand === brand) score += 20;
      if (record.preview && (nameText === record.preview || subText.indexOf(record.preview) >= 0)) score += 100;
      if (record.name && nameText === record.name) score += 80;
      if (record.name && nameText.indexOf(record.name) >= 0) score += 30;
      if (record.baseUrl && baseText.indexOf(record.baseUrl) >= 0) score += 40;
      if (!record.baseUrl && record.brand === "claude" && baseText.indexOf("anthropic") >= 0) score += 10;
      if (score > bestScore) {
        bestScore = score;
        best = record;
      }
    });
    return bestScore >= 30 ? best : null;
  }

  function decorateKey(cell, record) {
    if (!cell || !record || !record.key) return;
    var primaryName = classElement(cell, "ProviderResourceTable-module__primaryName");
    var primarySub = classElement(cell, "ProviderResourceTable-module__primarySub");
    var target = null;
    [primaryName, primarySub].some(function (element) {
      if (!element) return false;
      if (!target || (record.preview && textOf(element).indexOf(record.preview) >= 0)) {
        target = element;
      }
      return !!(record.preview && textOf(element).indexOf(record.preview) >= 0);
    });
    if (!target) target = primaryName || primarySub || cell;
    target.classList.add("cliproxy-provider-key-target");
    target.title = record.keys.length > 1 ? record.keys.join("\n") : record.key;
    var button = target.querySelector("[data-cliproxy-provider-copy]");
    if (!button) {
      button = document.createElement("button");
      button.type = "button";
      button.setAttribute("data-cliproxy-provider-copy", "true");
      button.setAttribute("aria-label", "复制密钥");
      button.title = "复制密钥";
      button.innerHTML = copyIcon();
      target.appendChild(button);
    }
    button.onclick = function (event) {
      event.preventDefault();
      event.stopPropagation();
      var value = record.key;
      copyText(value).then(function () {
        button.classList.add("cliproxy-provider-copy-done");
        button.innerHTML = checkIcon();
        button.title = "已复制";
        setTimeout(function () {
          button.classList.remove("cliproxy-provider-copy-done");
          button.innerHTML = copyIcon();
          button.title = "复制密钥";
        }, 1600);
      }).catch(function () {
        button.title = "复制失败";
      });
    };
  }

  function decorateBaseURL(cell, record) {
    if (!cell) return;
    var display = cell.querySelector("[class*=\"ProviderResourceTable-module__baseUrl\"]") || cell;
    var text = textOf(display);
    var raw = record && record.baseUrl ? record.baseUrl : findURL(text);
    var origin = normalizeURL(raw);
    if (!origin) return;
    var link = display.querySelector("[data-cliproxy-provider-base-link]");
    if (!link) {
      link = document.createElement("a");
      link.setAttribute("data-cliproxy-provider-base-link", "true");
      link.target = "_blank";
      link.rel = "noopener noreferrer";
      link.title = "打开服务地址";
      link.className = "cliproxy-provider-base-link";
      while (display.firstChild) link.appendChild(display.firstChild);
      display.appendChild(link);
    }
    link.href = origin;
  }

  function decorateLabel(cell, record) {
    if (!cell) return;
    var container = classElement(cell, "ProviderResourceTable-module__primaryCell") || cell;
    var label = container.querySelector("[data-cliproxy-provider-label]");
    if (!label) {
      label = document.createElement("button");
      label.type = "button";
      label.setAttribute("data-cliproxy-provider-label", "true");
      label.title = "点击修改标签";
      container.appendChild(label);
    }
    var current = record && record.label ? record.label : defaultLabel;
    if (label.textContent !== current) label.textContent = current;
    label.disabled = !record;
    label.onclick = function (event) {
      event.preventDefault();
      event.stopPropagation();
      if (!record) return;
      var next = window.prompt("请输入标签（留空恢复为“可用”）", record.label || defaultLabel);
      if (next === null) return;
      next = String(next).trim();
      if (next.length > 64) next = next.slice(0, 64);
      var api = client();
      if (!api || typeof api.patch !== "function") return;
      label.disabled = true;
      api.patch(record.endpoint, {
        index: record.index,
        value: { label: next }
      }).then(function () {
        record.label = next;
        label.textContent = next || defaultLabel;
      }).catch(function (error) {
        window.alert("标签保存失败：" + (error && error.message ? error.message : "未知错误"));
      }).finally(function () {
        label.disabled = false;
      });
    };
  }

  function nativeTableElements(root, selector, marker) {
    return Array.prototype.filter.call(root.querySelectorAll(selector), function (element) {
      return !element.hasAttribute(marker);
    });
  }

  function ensurePriorityColumn(table) {
    if (!table) return;
    table.classList.add("cliproxy-provider-priority-table");

    var colgroup = table.querySelector("colgroup");
    if (colgroup) {
      var priorityCol = colgroup.querySelector("[data-cliproxy-provider-priority-col]");
      if (!priorityCol) {
        priorityCol = document.createElement("col");
        priorityCol.setAttribute("data-cliproxy-provider-priority-col", "true");
        priorityCol.style.width = "88px";
      }
      var nativeCols = nativeTableElements(colgroup, "col", "data-cliproxy-provider-priority-col");
      var colReference = nativeCols[3] || null;
      if (priorityCol.parentNode !== colgroup || priorityCol.nextElementSibling !== colReference) {
        colgroup.insertBefore(priorityCol, colReference);
      }
    }

    var headerRow = table.querySelector("thead tr");
    if (!headerRow) return;
    var priorityHeader = headerRow.querySelector("th[data-cliproxy-provider-priority-header]");
    if (!priorityHeader) {
      priorityHeader = document.createElement("th");
      priorityHeader.setAttribute("data-cliproxy-provider-priority-header", "true");
      priorityHeader.textContent = "优先级";
    }
    var nativeHeaders = nativeTableElements(headerRow, "th", "data-cliproxy-provider-priority-header");
    var headerReference = nativeHeaders[3] || null;
    if (headerReference && headerReference.className) priorityHeader.className = headerReference.className;
    if (priorityHeader.parentNode !== headerRow || priorityHeader.nextElementSibling !== headerReference) {
      headerRow.insertBefore(priorityHeader, headerReference);
    }
  }

  function ensurePriorityCell(row) {
    if (!row) return null;
    var priorityCell = row.querySelector("td[data-cliproxy-provider-priority-cell]");
    if (!priorityCell) {
      priorityCell = document.createElement("td");
      priorityCell.setAttribute("data-cliproxy-provider-priority-cell", "true");
    }
    var nativeCells = nativeTableElements(row, "td", "data-cliproxy-provider-priority-cell");
    var cellReference = nativeCells[3] || null;
    if (cellReference && cellReference.className) priorityCell.className = cellReference.className;
    if (priorityCell.parentNode !== row || priorityCell.nextElementSibling !== cellReference) {
      row.insertBefore(priorityCell, cellReference);
    }
    return priorityCell;
  }

  function showToast(message, kind) {
    var toast = document.querySelector("[data-cliproxy-provider-toast]");
    if (!toast) {
      toast = document.createElement("div");
      toast.setAttribute("data-cliproxy-provider-toast", "true");
      toast.setAttribute("role", "status");
      toast.setAttribute("aria-live", "polite");
      document.body.appendChild(toast);
    }
    toast.className = "cliproxy-provider-toast cliproxy-provider-toast-" + (kind || "success");
    toast.textContent = message;
    toast.classList.add("cliproxy-provider-toast-visible");
    if (toastTimer !== null) clearTimeout(toastTimer);
    toastTimer = setTimeout(function () {
      toast.classList.remove("cliproxy-provider-toast-visible");
      toastTimer = null;
    }, 1800);
  }

  function savePriority(input, record) {
    if (!input || !record || record.prioritySaving) return;
    var raw = String(input.value || "").trim();
    var next = raw === "" ? 0 : Number(raw);
    if (!isFinite(next) || Math.floor(next) !== next) {
      input.value = String(record.priority);
      return;
    }
    if (next === record.priority) return;
    var api = client();
    if (!api || typeof api.patch !== "function") {
      input.value = String(record.priority);
      return;
    }
    var previous = record.priority;
    record.prioritySaving = true;
    input.disabled = true;
    api.patch(record.endpoint, {
      index: record.index,
      value: { priority: next }
    }).then(function () {
      record.priority = next;
      input.value = String(next);
      showToast("\u4f18\u5148\u7ea7\u5df2\u4fdd\u5b58", "success");
    }).catch(function (error) {
      input.value = String(previous);
      showToast("\u4f18\u5148\u7ea7\u4fdd\u5b58\u5931\u8d25", "error");
    }).finally(function () {
      record.prioritySaving = false;
      enhance();
    });
  }

  function decoratePriority(cell, record) {
    if (!cell) return;
    var priority = cell.querySelector("[data-cliproxy-provider-priority-value]");
    if (!priority) {
      priority = document.createElement("input");
      priority.type = "number";
      priority.step = "1";
      priority.inputMode = "numeric";
      priority.setAttribute("data-cliproxy-provider-priority-value", "true");
      priority.setAttribute("aria-label", "优先级");
      cell.appendChild(priority);
    }
    var value = record ? String(record.priority) : "";
    var title = record ? "优先级：" + record.priority + "；数值越高，选择优先级越高" : "正在加载优先级";
    if (document.activeElement !== priority && priority.value !== value) priority.value = value;
    priority.placeholder = record ? "0" : "--";
    priority.title = title;
    priority.disabled = !record || !!record.prioritySaving;
    priority.onchange = function () {
      savePriority(priority, record);
    };
    priority.onkeydown = function (event) {
      if (event.key === "Enter") {
        event.preventDefault();
        priority.blur();
      } else if (event.key === "Escape" && record) {
        event.preventDefault();
        priority.value = String(record.priority);
        priority.blur();
      }
    };
  }

  function enhance() {
    if (!isActive()) return;
    var tables = document.querySelectorAll("table[class*=\"ProviderResourceTable-module__providerTable\"]");
    Array.prototype.forEach.call(tables, function (table) {
      ensurePriorityColumn(table);
      Array.prototype.forEach.call(table.querySelectorAll("tbody tr"), function (row) {
        var cells = row.querySelectorAll("td");
        if (cells.length < 2) return;
        var record = findRecord(row);
        decorateKey(cells[0], record);
        decorateBaseURL(cells[1], record);
        decorateLabel(cells[0], record);
        decoratePriority(ensurePriorityCell(row), record);
      });
    });
  }

  function scheduleEnhance() {
    if (!isActive() || enhanceTimer !== null) return;
    enhanceTimer = setTimeout(function () {
      enhanceTimer = null;
      var signature = nativeSignature();
      enhance();
      if (isActive() && (signature !== lastNativeSignature || !recordsReady)) {
        lastNativeSignature = signature;
        loadRecords();
      }
    }, 80);
  }

  var style = document.createElement("style");
  style.textContent =
    ".cliproxy-provider-key-target{display:inline-flex;align-items:center;gap:5px;max-width:100%;}" +
    ".cliproxy-provider-key-target button{display:inline-flex;align-items:center;justify-content:center;width:20px;height:20px;margin:0;padding:2px;border:0;border-radius:5px;background:transparent;color:inherit;opacity:.55;cursor:pointer;}" +
    ".cliproxy-provider-key-target button:hover{opacity:1;background:color-mix(in srgb,currentColor 12%,transparent);}" +
    ".cliproxy-provider-key-target button svg{width:14px;height:14px;fill:none;stroke:currentColor;stroke-width:2;stroke-linecap:round;stroke-linejoin:round;}" +
    ".cliproxy-provider-copy-done{color:#16a34a!important;opacity:1!important;}" +
    ".cliproxy-provider-base-link{color:inherit;text-decoration:underline;text-decoration-style:dotted;text-underline-offset:3px;cursor:pointer;}" +
    "[data-cliproxy-provider-label]{align-self:flex-start;margin-top:4px;padding:2px 8px;border:1px solid color-mix(in srgb,var(--primary-color,#2563eb) 28%,var(--border-color,#d1d5db));border-radius:999px;background:color-mix(in srgb,var(--primary-color,#2563eb) 9%,transparent);color:var(--primary-color,#2563eb);font-family:inherit;font-size:11px;font-weight:600;line-height:1.4;cursor:pointer;}" +
    "[data-cliproxy-provider-label]:hover{background:color-mix(in srgb,var(--primary-color,#2563eb) 16%,transparent);}" +
    "[data-cliproxy-provider-label]:disabled{cursor:default;opacity:.85;}" +
    ".cliproxy-provider-toast{position:fixed;top:20px;right:20px;z-index:9999;max-width:320px;padding:10px 14px;border:1px solid transparent;border-radius:8px;box-shadow:0 10px 28px rgba(15,23,42,.18);font:600 13px/1.4 inherit;opacity:0;transform:translateY(-8px);pointer-events:none;transition:opacity .18s ease,transform .18s ease;}" +
    ".cliproxy-provider-toast-visible{opacity:1;transform:translateY(0);}" +
    ".cliproxy-provider-toast-success{background:#ecfdf3;border-color:#86efac;color:#166534;}" +
    ".cliproxy-provider-toast-error{background:#fef2f2;border-color:#fca5a5;color:#991b1b;}" +
    ".cliproxy-provider-priority-table{min-width:1048px!important;}" +
    "[data-cliproxy-provider-priority-value]{box-sizing:border-box;width:72px;height:30px;padding:3px 7px;border:1px solid var(--border-color,#d1d5db);border-radius:var(--radius-md,6px);background:var(--muted-bg,rgba(148,163,184,.12));color:var(--text-primary,#111827);font:600 12px/1.4 inherit;text-align:center;}" +
    "[data-cliproxy-provider-priority-value]:focus{border-color:var(--primary-color,#2563eb);outline:2px solid color-mix(in srgb,var(--primary-color,#2563eb) 20%,transparent);outline-offset:0;}";
  document.head.appendChild(style);

  function scheduleRouteReload() {
    if (!isActive()) return;
    if (routeReloadTimer !== null) clearTimeout(routeReloadTimer);
    routeReloadTimer = setTimeout(function () {
      routeReloadTimer = null;
      if (!isActive()) return;
      recordsReady = false;
      loadRecords(true);
      scheduleEnhance();
    }, 320);
  }

  function handleRouteChange() {
    lastNativeSignature = "";
    recordsReady = false;
    records = [];
    if (isActive()) {
      loadRecords();
      scheduleEnhance();
      scheduleRouteReload();
    }
  }

  window.addEventListener("hashchange", handleRouteChange);
  window.addEventListener("popstate", handleRouteChange);
  window.addEventListener("cliproxy-route-change", handleRouteChange);
  ["pushState", "replaceState"].forEach(function (method) {
    var original = window.history[method];
    if (typeof original !== "function") return;
    window.history[method] = function () {
      var result = original.apply(this, arguments);
      window.dispatchEvent(new Event("cliproxy-route-change"));
      return result;
    };
  });
  var observer = new MutationObserver(scheduleEnhance);
  observer.observe(document.documentElement, { childList: true, subtree: true });
  setInterval(function () {
    if (!isActive()) return;
    var signature = nativeSignature();
    if (signature !== lastNativeSignature || !recordsReady) {
      lastNativeSignature = signature;
      loadRecords();
    } else {
      enhance();
    }
  }, 1000);
  loadRecords();
  scheduleEnhance();
})();
</script>`
