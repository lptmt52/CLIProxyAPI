package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) serveManagementTokenUsagePage(c *gin.Context) {
	cfg := s.cfg
	if cfg == nil || cfg.Home.Enabled || cfg.RemoteManagement.DisableControlPanel {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(managementTokenUsageHTML))
}

func injectManagementTokenUsageEntry(data []byte) []byte {
	if len(data) == 0 {
		return []byte(managementTokenUsagePreScript + managementTokenUsageEntryScript)
	}
	html := string(data)
	if strings.Contains(html, "cliproxy-token-usage-entry") {
		return data
	}
	html = insertManagementTokenUsagePreScript(html)
	lower := strings.ToLower(html)
	index := strings.LastIndex(lower, "</body>")
	if index < 0 {
		out := make([]byte, 0, len(html)+len(managementTokenUsageEntryScript))
		out = append(out, []byte(html)...)
		out = append(out, managementTokenUsageEntryScript...)
		return out
	}
	return []byte(html[:index] + managementTokenUsageEntryScript + html[index:])
}

func insertManagementTokenUsagePreScript(html string) string {
	lower := strings.ToLower(html)
	headIndex := strings.Index(lower, "<head")
	if headIndex < 0 {
		return managementTokenUsagePreScript + html
	}
	closeIndex := strings.Index(lower[headIndex:], ">")
	if closeIndex < 0 {
		return managementTokenUsagePreScript + html
	}
	insertAt := headIndex + closeIndex + 1
	return html[:insertAt] + managementTokenUsagePreScript + html[insertAt:]
}

const managementTokenUsagePreScript = `<script id="cliproxy-token-usage-pre">
(function () {
  var tokenRoute = "/token-usage";
  var pendingKey = "cliproxy.tokenUsage.pendingRoute";
  var hashPath = (window.location.hash || "").replace(/^#/, "").split("?")[0].replace(/\/$/, "");
  if (hashPath === tokenRoute) {
    try { sessionStorage.setItem(pendingKey, "1"); } catch (error) {}
    history.replaceState(null, document.title, window.location.pathname + window.location.search + "#/");
  }
})();
</script>`

const managementTokenUsageEntryScript = `<script id="cliproxy-token-usage-entry">
(function () {
  if (window.__cliproxyTokenUsageEntryMounted) return;
  window.__cliproxyTokenUsageEntryMounted = true;
  var routeHash = "#/token-usage";
  var href = "/management.html" + routeHash;
  var apiPath = "/v0/management/token-usage";
  var storageKey = "cliproxy.tokenUsage.managementKey";
  var pendingKey = "cliproxy.tokenUsage.pendingRoute";
  var refreshTimer = null;
  var style = document.createElement("style");
  style.textContent = "#cliproxy-token-usage-floating{position:fixed;right:22px;bottom:22px;z-index:2147483000;display:inline-flex;align-items:center;gap:8px;border:1px solid color-mix(in srgb,var(--border-color,#d7d2c8) 72%,transparent);border-radius:999px;background:color-mix(in srgb,var(--bg-primary,#fff) 92%,transparent);color:var(--text-primary,#2d2a26);box-shadow:0 16px 38px rgba(0,0,0,.16);padding:10px 14px;font:600 13px/1.2 system-ui,-apple-system,Segoe UI,sans-serif;text-decoration:none;backdrop-filter:blur(12px)}#cliproxy-token-usage-floating:hover{transform:translateY(-1px)}#cliproxy-token-usage-nav .cliproxy-token-usage-dot{width:18px;height:18px;border-radius:999px;background:linear-gradient(135deg,#0f766e,#f59e0b);display:inline-block;box-shadow:inset 0 0 0 2px rgba(255,255,255,.45)}#cliproxy-token-usage-nav .nav-item.active{border-color:color-mix(in srgb,var(--primary-color,#0f766e) 30%,transparent);background:color-mix(in srgb,var(--primary-color,#0f766e) 10%,transparent);color:var(--primary-color,#0f766e)}#cliproxy-token-usage-view{width:100%;animation:cliproxy-token-usage-rise .32s ease-out both}#cliproxy-token-usage-view .tu-page{display:flex;flex-direction:column;gap:18px}#cliproxy-token-usage-view .tu-hero,#cliproxy-token-usage-view .tu-card,#cliproxy-token-usage-view .tu-panel{border:1px solid color-mix(in srgb,var(--border-color,#d7d2c8) 70%,transparent);border-radius:16px;background:linear-gradient(135deg,color-mix(in srgb,var(--bg-primary,#fff) 92%,transparent),color-mix(in srgb,var(--bg-secondary,#f6f2ea) 80%,transparent));box-shadow:0 18px 42px rgba(0,0,0,.12);backdrop-filter:blur(12px)}#cliproxy-token-usage-view .tu-hero{position:relative;overflow:hidden;padding:30px}#cliproxy-token-usage-view .tu-hero:after{content:'TOKENS';position:absolute;right:20px;top:-20px;font-size:min(14vw,132px);line-height:1;font-weight:900;color:color-mix(in srgb,var(--text-primary,#2d2a26) 6%,transparent);pointer-events:none}#cliproxy-token-usage-view .tu-hero-inner{position:relative;z-index:1;display:flex;justify-content:space-between;gap:20px;align-items:flex-end}#cliproxy-token-usage-view .tu-eyebrow{margin:0 0 8px;color:var(--primary-color,#0f766e);font-size:12px;font-weight:800;letter-spacing:.12em;text-transform:uppercase}#cliproxy-token-usage-view h1{margin:0;color:var(--text-primary,#2d2a26);font-size:clamp(28px,5vw,48px);line-height:1.06;letter-spacing:-.04em}#cliproxy-token-usage-view .tu-subtitle{max-width:700px;margin:12px 0 0;color:var(--text-secondary,#756b5f);font-size:14px;line-height:1.6}#cliproxy-token-usage-view .tu-actions{display:flex;flex-wrap:wrap;gap:10px;justify-content:flex-end}#cliproxy-token-usage-view button{border:1px solid var(--border-color,#d7d2c8);border-radius:999px;background:var(--bg-primary,#fff);color:var(--text-primary,#2d2a26);padding:9px 13px;cursor:pointer;font:inherit;font-weight:700}#cliproxy-token-usage-view button.primary{border-color:transparent;background:var(--primary-color,#0f766e);color:var(--primary-contrast,#fff)}#cliproxy-token-usage-view .tu-status{display:flex;flex-wrap:wrap;gap:10px;margin-top:16px}#cliproxy-token-usage-view .tu-pill{display:inline-flex;align-items:center;gap:8px;border:1px solid var(--border-color,#d7d2c8);border-radius:999px;background:color-mix(in srgb,var(--bg-primary,#fff) 70%,transparent);color:var(--text-secondary,#756b5f);padding:6px 11px;font-size:12px}#cliproxy-token-usage-view .tu-dot{width:8px;height:8px;border-radius:50%;background:#f59e0b}#cliproxy-token-usage-view .tu-dot.ok{background:#10b981;box-shadow:0 0 0 4px rgba(16,185,129,.14)}#cliproxy-token-usage-view .tu-dot.err{background:#ef4444;box-shadow:0 0 0 4px rgba(239,68,68,.14)}#cliproxy-token-usage-view .tu-login{display:none;padding:18px;border:1px solid var(--border-color,#d7d2c8);border-radius:14px;background:color-mix(in srgb,var(--bg-primary,#fff) 78%,transparent)}#cliproxy-token-usage-view .tu-login.visible{display:block}#cliproxy-token-usage-view .tu-form{display:flex;gap:10px;flex-wrap:wrap}#cliproxy-token-usage-view input{min-width:min(420px,100%);flex:1;border:1px solid var(--border-color,#d7d2c8);border-radius:999px;background:var(--bg-primary,#fff);color:var(--text-primary,#2d2a26);padding:10px 13px;font:inherit}#cliproxy-token-usage-view .tu-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:14px}#cliproxy-token-usage-view .tu-card{position:relative;overflow:hidden;min-height:142px;padding:18px}#cliproxy-token-usage-view .tu-card:after{content:'';position:absolute;right:-40px;bottom:-48px;width:120px;height:120px;border-radius:50%;background:color-mix(in srgb,var(--primary-color,#0f766e) 10%,transparent)}#cliproxy-token-usage-view .tu-label{color:var(--text-secondary,#756b5f);font-weight:700}#cliproxy-token-usage-view .tu-value{margin-top:12px;color:var(--text-primary,#2d2a26);font-size:clamp(26px,4vw,38px);line-height:1;font-weight:900;letter-spacing:-.04em}#cliproxy-token-usage-view .tu-meta{margin-top:12px;color:var(--text-secondary,#756b5f);font-size:12px}#cliproxy-token-usage-view .tu-mini{display:flex;flex-wrap:wrap;gap:12px;margin-top:14px;color:var(--text-secondary,#756b5f);font-size:12px}#cliproxy-token-usage-view .tu-mini strong{color:var(--text-primary,#2d2a26)}#cliproxy-token-usage-view .tu-panel{overflow:hidden}#cliproxy-token-usage-view .tu-panel-head{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:16px 18px;border-bottom:1px solid var(--border-color,#d7d2c8)}#cliproxy-token-usage-view .tu-panel h2{margin:0;font-size:18px;color:var(--text-primary,#2d2a26)}#cliproxy-token-usage-view .tu-table-wrap{overflow-x:auto}#cliproxy-token-usage-view table{width:100%;min-width:820px;border-collapse:collapse}#cliproxy-token-usage-view th,#cliproxy-token-usage-view td{padding:13px 14px;border-bottom:1px solid var(--border-color,#d7d2c8);text-align:right;font-variant-numeric:tabular-nums}#cliproxy-token-usage-view th:first-child,#cliproxy-token-usage-view td:first-child{text-align:left}#cliproxy-token-usage-view th{color:var(--text-secondary,#756b5f);font-size:12px;text-transform:uppercase;background:color-mix(in srgb,var(--bg-secondary,#f6f2ea) 72%,transparent)}#cliproxy-token-usage-view .tu-provider{display:inline-flex;align-items:center;gap:10px;font-weight:800;color:var(--text-primary,#2d2a26)}#cliproxy-token-usage-view .tu-mark{width:26px;height:26px;border-radius:9px;background:linear-gradient(135deg,#0f766e,#f59e0b)}#cliproxy-token-usage-view .muted{color:var(--text-secondary,#756b5f)}#cliproxy-token-usage-view .empty{padding:24px;text-align:center;color:var(--text-secondary,#756b5f)}@keyframes cliproxy-token-usage-rise{from{opacity:0;transform:translateY(10px)}to{opacity:1;transform:translateY(0)}}@media(max-width:980px){#cliproxy-token-usage-view .tu-grid{grid-template-columns:repeat(2,minmax(0,1fr))}#cliproxy-token-usage-view .tu-hero-inner{flex-direction:column;align-items:flex-start}#cliproxy-token-usage-view .tu-actions{justify-content:flex-start}}@media(max-width:560px){#cliproxy-token-usage-view .tu-grid{grid-template-columns:1fr}#cliproxy-token-usage-view .tu-hero{padding:22px}#cliproxy-token-usage-view .tu-form{display:block}#cliproxy-token-usage-view .tu-form button{margin-top:10px;width:100%}}";
  document.head.appendChild(style);

  function addFloating() {
    if (document.getElementById("cliproxy-token-usage-floating")) return;
    var link = document.createElement("a");
    link.id = "cliproxy-token-usage-floating";
    link.href = href;
    link.target = "_self";
    link.setAttribute("data-cliproxy-token-usage-link", "1");
    link.textContent = "Token 统计";
    document.body.appendChild(link);
  }

  function addSidebar() {
    if (document.getElementById("cliproxy-token-usage-nav")) return true;
    var section = document.querySelector(".nav-section");
    if (!section) return false;
    var group = document.createElement("div");
    group.id = "cliproxy-token-usage-nav";
    group.className = "nav-group";
    group.innerHTML = '<a class="nav-item" data-cliproxy-token-usage-link="1" href="' + href + '" title="Token 统计"><span class="nav-icon"><span class="cliproxy-token-usage-dot"></span></span><span class="nav-text"><span class="nav-label">Token 统计</span><span class="nav-meta">按提供商查看用量</span></span></a>';
    section.appendChild(group);
    setNavActive(isTokenUsageRoute());
    return true;
  }

  function navigateTokenUsage() {
    history.pushState(null, document.title, window.location.pathname + window.location.search + routeHash);
    renderTokenUsageView();
  }

  function isTokenUsageRoute() {
    var hashPath = (window.location.hash || "").replace(/^#/, "").split("?")[0].replace(/\/$/, "");
    return hashPath === "/token-usage";
  }

  function showOriginalContent(show) {
    var host = findContentHost();
    if (!host) return;
    Array.prototype.forEach.call(host.children, function (child) {
      if (child.id === "cliproxy-token-usage-view") return;
      if (show) {
        if (child.hasAttribute("data-cliproxy-token-usage-old-display")) {
          child.style.display = child.getAttribute("data-cliproxy-token-usage-old-display");
          child.removeAttribute("data-cliproxy-token-usage-old-display");
        } else {
          child.style.display = "";
        }
      } else {
        if (!child.hasAttribute("data-cliproxy-token-usage-old-display")) {
          child.setAttribute("data-cliproxy-token-usage-old-display", child.style.display || "");
        }
        child.style.display = "none";
      }
    });
  }

  function findContentHost() {
    return document.querySelector(".main-content") || document.querySelector("main") || document.getElementById("root") || document.body;
  }

  function ensureViewRoot() {
    var host = findContentHost();
    if (!host) return null;
    showOriginalContent(false);
    var root = host.querySelector("#cliproxy-token-usage-view");
    if (!root) {
      root = document.createElement("section");
      root.id = "cliproxy-token-usage-view";
      host.appendChild(root);
    }
    root.style.display = "";
    return root;
  }

  function clearTokenUsageView() {
    var root = document.getElementById("cliproxy-token-usage-view");
    if (root) root.style.display = "none";
    showOriginalContent(true);
    setNavActive(false);
    if (refreshTimer) {
      clearInterval(refreshTimer);
      refreshTimer = null;
    }
  }

  function setNavActive(active) {
    var link = document.querySelector("#cliproxy-token-usage-nav .nav-item");
    if (link) link.classList.toggle("active", !!active);
  }

  function renderTokenUsageView() {
    var root = ensureViewRoot();
    if (!root) return;
    setNavActive(true);
    if (!root.getAttribute("data-ready")) {
      root.setAttribute("data-ready", "1");
      root.innerHTML = '<div class="tu-page"><section class="tu-hero"><div class="tu-hero-inner"><div><p class="tu-eyebrow">CLI Proxy API</p><h1>Token 使用统计</h1><p class="tu-subtitle">按 AI 提供商汇总当前进程内的 token 用量，包含总使用、今日、本周和本月数据。统计只在 usage-statistics-enabled 开启后采集。</p><div class="tu-status"><span class="tu-pill"><span id="tu-status-dot" class="tu-dot"></span><span id="tu-status-text">等待认证</span></span><span class="tu-pill">更新时间：<span id="tu-updated-at">-</span></span><span class="tu-pill">统计范围：当前进程内存</span></div></div><div class="tu-actions"><button id="tu-refresh" class="primary" type="button">刷新数据</button><button id="tu-clear-key" type="button">清除密钥</button></div></div></section><section id="tu-login" class="tu-login"><form id="tu-form" class="tu-form"><input id="tu-key" type="password" autocomplete="current-password" placeholder="输入管理密钥"><button class="primary" type="submit">保存并读取统计</button></form><p class="muted">如果你已经在管理中心登录，本页会尽量自动读取同源本地存储中的管理密钥；不同面板版本不兼容时，可在这里手动输入。</p></section><section id="tu-cards" class="tu-grid"></section><section class="tu-panel"><div class="tu-panel-head"><h2>按 AI 提供商统计</h2><span id="tu-provider-count" class="muted">-</span></div><div class="tu-table-wrap"><table><thead><tr><th>提供商</th><th>总 Token</th><th>今日</th><th>本周</th><th>本月</th><th>请求</th><th>失败</th><th>输入</th><th>输出</th><th>推理</th></tr></thead><tbody id="tu-provider-body"><tr><td colspan="10" class="empty">等待数据</td></tr></tbody></table></div></section></div>';
      bindTokenUsageView(root);
    }
    refreshTokenUsage(root, true);
    if (!refreshTimer) {
      refreshTimer = setInterval(function () {
        if (isTokenUsageRoute() && document.visibilityState === "visible") refreshTokenUsage(root, true);
      }, 15000);
    }
  }

  function bindTokenUsageView(root) {
    renderEmptyCards(root);
    var key = findInitialKey();
    var login = root.querySelector("#tu-login");
    var input = root.querySelector("#tu-key");
    input.value = key || "";
    login.classList.toggle("visible", !key);
    root.querySelector("#tu-form").addEventListener("submit", function (event) {
      event.preventDefault();
      var value = input.value.trim();
      if (!value) {
        setStatus(root, "err", "请输入管理密钥");
        return;
      }
      try { localStorage.setItem(storageKey, value); } catch (error) {}
      login.classList.remove("visible");
      refreshTokenUsage(root, false);
    });
    root.querySelector("#tu-refresh").addEventListener("click", function () { refreshTokenUsage(root, false); });
    root.querySelector("#tu-clear-key").addEventListener("click", function () {
      try { localStorage.removeItem(storageKey); } catch (error) {}
      input.value = "";
      login.classList.add("visible");
      setStatus(root, "", "已清除密钥");
    });
  }

  async function refreshTokenUsage(root, silent) {
    var key = (root.querySelector("#tu-key").value || findInitialKey()).trim();
    if (!key) {
      root.querySelector("#tu-login").classList.add("visible");
      setStatus(root, "", "需要管理密钥");
      return;
    }
    if (!silent) setStatus(root, "", "正在读取统计");
    var button = root.querySelector("#tu-refresh");
    if (button) button.disabled = true;
    try {
      var response = await fetch(apiPath, { headers: { "Authorization": "Bearer " + key, "X-Management-Key": key }, cache: "no-store" });
      if (response.status === 401 || response.status === 403) throw new Error("认证失败，请重新输入管理密钥");
      if (response.status === 404) throw new Error("当前服务未提供 token 统计接口，请确认已经启动新编译的 exe");
      if (!response.ok) throw new Error("读取失败 HTTP " + response.status);
      var data = await response.json();
      renderUsageData(root, data);
      try { localStorage.setItem(storageKey, key); } catch (error) {}
      root.querySelector("#tu-login").classList.remove("visible");
      setStatus(root, data.enabled ? "ok" : "", data.enabled ? "统计已开启" : "统计开关未开启");
    } catch (error) {
      setStatus(root, "err", error.message || "读取失败");
      if (/认证失败/.test(error.message || "")) root.querySelector("#tu-login").classList.add("visible");
    } finally {
      if (button) button.disabled = false;
    }
  }

  function renderUsageData(root, data) {
    var updated = root.querySelector("#tu-updated-at");
    updated.textContent = formatDateTime(data && data.generated_at);
    var summaries = [
      { title: "总使用", hint: "当前进程累计", bucket: data && data.total },
      { title: "今日", hint: periodHint(data && data.day), bucket: data && data.day },
      { title: "本周", hint: periodHint(data && data.week), bucket: data && data.week },
      { title: "本月", hint: periodHint(data && data.month), bucket: data && data.month }
    ];
    root.querySelector("#tu-cards").innerHTML = summaries.map(function (item) {
      var bucket = normalizeBucket(item.bucket);
      return '<article class="tu-card"><div class="tu-label">' + escapeHtml(item.title) + '</div><div class="tu-value">' + number(bucket.tokens.total_tokens) + '</div><div class="tu-meta">' + escapeHtml(item.hint) + '</div><div class="tu-mini"><span>请求 <strong>' + number(bucket.requests) + '</strong></span><span>失败 <strong>' + number(bucket.failed_requests) + '</strong></span><span>输出 <strong>' + number(bucket.tokens.output_tokens) + '</strong></span></div></article>';
    }).join("");
    var providers = Array.isArray(data && data.providers) ? data.providers.slice() : [];
    providers.sort(function (left, right) { return tokenTotal(right.total) - tokenTotal(left.total) || String(left.provider).localeCompare(String(right.provider)); });
    root.querySelector("#tu-provider-count").textContent = providers.length ? providers.length + " 个提供商" : "暂无提供商";
    if (!providers.length) {
      root.querySelector("#tu-provider-body").innerHTML = '<tr><td colspan="10" class="empty">还没有 token 使用记录。请通过代理发起一次模型请求后再刷新。</td></tr>';
      return;
    }
    root.querySelector("#tu-provider-body").innerHTML = providers.map(function (provider) {
      var total = normalizeBucket(provider.total);
      return '<tr><td><span class="tu-provider"><span class="tu-mark"></span>' + escapeHtml(provider.provider || "unknown") + '</span></td><td>' + number(total.tokens.total_tokens) + '</td><td>' + number(tokenTotal(provider.day)) + '</td><td>' + number(tokenTotal(provider.week)) + '</td><td>' + number(tokenTotal(provider.month)) + '</td><td>' + number(total.requests) + '</td><td>' + number(total.failed_requests) + '</td><td>' + number(total.tokens.input_tokens) + '</td><td>' + number(total.tokens.output_tokens) + '</td><td>' + number(total.tokens.reasoning_tokens) + '</td></tr>';
    }).join("");
  }

  function renderEmptyCards(root) {
    root.querySelector("#tu-cards").innerHTML = ["总使用", "今日", "本周", "本月"].map(function (title) {
      return '<article class="tu-card"><div class="tu-label">' + title + '</div><div class="tu-value">0</div><div class="tu-meta">等待数据</div><div class="tu-mini"><span>请求 <strong>0</strong></span><span>失败 <strong>0</strong></span></div></article>';
    }).join("");
  }

  function findInitialKey() {
    var direct = cleanKey(localStorage.getItem(storageKey) || sessionStorage.getItem(storageKey));
    if (direct) return direct;
    var names = ["managementKey", "management-key", "management_key", "cliproxy-management-key", "cpa-management-key"];
    for (var i = 0; i < names.length; i++) {
      var value = cleanKey(localStorage.getItem(names[i]) || sessionStorage.getItem(names[i]));
      if (value) return value;
    }
    return scanStorageForManagementKey(localStorage) || scanStorageForManagementKey(sessionStorage) || "";
  }

  function scanStorageForManagementKey(storage) {
    try {
      for (var i = 0; i < storage.length; i++) {
        var name = storage.key(i);
        var value = storage.getItem(name);
        if (/management.*key/i.test(name || "")) {
          var direct = cleanKey(value);
          if (direct && direct.charAt(0) !== "{" && direct.charAt(0) !== "[") return direct;
        }
        var found = extractKey(tryJSON(value), 0);
        if (found) return found;
      }
    } catch (error) {}
    return "";
  }

  function extractKey(value, depth) {
    if (!value || typeof value !== "object" || depth > 6) return "";
    var preferred = ["managementKey", "management_key", "management-key"];
    for (var i = 0; i < preferred.length; i++) {
      var candidate = cleanKey(value[preferred[i]]);
      if (candidate) return candidate;
    }
    if (value.state) {
      var nested = extractKey(value.state, depth + 1);
      if (nested) return nested;
    }
    for (var keyName in value) {
      if (!Object.prototype.hasOwnProperty.call(value, keyName)) continue;
      if (/management.*key/i.test(keyName)) {
        var direct = cleanKey(value[keyName]);
        if (direct) return direct;
      }
      var found = extractKey(value[keyName], depth + 1);
      if (found) return found;
    }
    return "";
  }

  function tryJSON(value) { try { return value ? JSON.parse(value) : null; } catch (error) { return null; } }
  function cleanKey(value) { return typeof value === "string" ? value.trim().replace(/^['"]|['"]$/g, "").trim() : ""; }
  function normalizeBucket(bucket) {
    bucket = bucket || {};
    bucket.tokens = bucket.tokens || {};
    return { requests: bucket.requests || 0, successful_requests: bucket.successful_requests || 0, failed_requests: bucket.failed_requests || 0, tokens: { input_tokens: bucket.tokens.input_tokens || 0, output_tokens: bucket.tokens.output_tokens || 0, reasoning_tokens: bucket.tokens.reasoning_tokens || 0, cached_tokens: bucket.tokens.cached_tokens || 0, cache_read_tokens: bucket.tokens.cache_read_tokens || 0, cache_creation_tokens: bucket.tokens.cache_creation_tokens || 0, total_tokens: bucket.tokens.total_tokens || 0 } };
  }
  function tokenTotal(bucket) { return normalizeBucket(bucket).tokens.total_tokens; }
  function periodHint(bucket) { return bucket && bucket.label ? bucket.label : "当前周期"; }
  function formatDateTime(value) { var date = value ? new Date(value) : null; return date && !Number.isNaN(date.getTime()) ? date.toLocaleString() : "-"; }
  function number(value) { return new Intl.NumberFormat("zh-CN").format(Number(value) || 0); }
  function escapeHtml(value) { return String(value == null ? "" : value).replace(/[&<>"']/g, function (char) { return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[char]; }); }
  function setStatus(root, type, text) {
    var dot = root.querySelector("#tu-status-dot");
    dot.className = "tu-dot" + (type ? " " + type : "");
    root.querySelector("#tu-status-text").textContent = text;
  }

  document.addEventListener("click", function (event) {
    var link = event.target && event.target.closest ? event.target.closest("[data-cliproxy-token-usage-link]") : null;
    if (link) {
      event.preventDefault();
      navigateTokenUsage();
      return;
    }
    if (event.target && event.target.closest && event.target.closest(".nav-item")) {
      setTimeout(function () { if (!isTokenUsageRoute()) clearTokenUsageView(); }, 60);
    }
  });

  window.addEventListener("popstate", function () {
    if (isTokenUsageRoute()) renderTokenUsageView();
    else clearTokenUsageView();
  });
  window.addEventListener("hashchange", function () {
    if (isTokenUsageRoute()) renderTokenUsageView();
    else clearTokenUsageView();
  });

  addFloating();
  addSidebar();
  try {
    if (sessionStorage.getItem(pendingKey) === "1") {
      sessionStorage.removeItem(pendingKey);
      history.replaceState(null, document.title, window.location.pathname + window.location.search + routeHash);
      renderTokenUsageView();
    } else if (isTokenUsageRoute()) {
      renderTokenUsageView();
    }
  } catch (error) {
    if (isTokenUsageRoute()) renderTokenUsageView();
  }
  var observer = new MutationObserver(function () {
    addSidebar();
    if (isTokenUsageRoute()) {
      renderTokenUsageView();
    }
  });
  observer.observe(document.documentElement, { childList: true, subtree: true });
})();
</script>`

const managementTokenUsageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Token 使用统计 - CLI Proxy API</title>
  <style>
    :root {
      color-scheme: light dark;
      --bg: #f6f2ea;
      --bg-strong: #ede4d4;
      --card: rgba(255, 252, 246, .82);
      --card-solid: #fffaf2;
      --text: #27221c;
      --muted: #756b5f;
      --line: rgba(86, 69, 49, .16);
      --accent: #0f766e;
      --accent-2: #d97706;
      --danger: #b42318;
      --ok: #13795b;
      --shadow: 0 22px 60px rgba(86, 69, 49, .16);
    }
    @media (prefers-color-scheme: dark) {
      :root {
        --bg: #151412;
        --bg-strong: #26211b;
        --card: rgba(35, 31, 26, .82);
        --card-solid: #211d18;
        --text: #f7efe3;
        --muted: #b6aa9b;
        --line: rgba(246, 239, 227, .13);
        --accent: #2dd4bf;
        --accent-2: #fbbf24;
        --danger: #fb7185;
        --ok: #34d399;
        --shadow: 0 22px 60px rgba(0, 0, 0, .36);
      }
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      color: var(--text);
      font: 14px/1.55 "Segoe UI", "Microsoft YaHei", system-ui, sans-serif;
      background:
        radial-gradient(circle at 10% 0%, color-mix(in srgb, var(--accent) 22%, transparent), transparent 30rem),
        radial-gradient(circle at 90% 10%, color-mix(in srgb, var(--accent-2) 20%, transparent), transparent 26rem),
        linear-gradient(145deg, var(--bg), var(--bg-strong));
    }
    a { color: inherit; }
    .shell { width: min(1180px, calc(100% - 32px)); margin: 0 auto; padding: 34px 0 42px; }
    .hero {
      position: relative;
      overflow: hidden;
      border: 1px solid var(--line);
      border-radius: 28px;
      padding: 30px;
      background: linear-gradient(135deg, var(--card), color-mix(in srgb, var(--card) 65%, transparent));
      box-shadow: var(--shadow);
      backdrop-filter: blur(18px);
    }
    .hero:before {
      content: "TOKENS";
      position: absolute;
      right: 20px;
      top: -24px;
      color: color-mix(in srgb, var(--text) 7%, transparent);
      font-size: clamp(72px, 12vw, 164px);
      font-weight: 900;
      line-height: 1;
      pointer-events: none;
    }
    .hero-inner { position: relative; display: flex; justify-content: space-between; gap: 22px; align-items: flex-end; }
    .eyebrow { margin: 0 0 8px; color: var(--accent); font-size: 12px; letter-spacing: .12em; text-transform: uppercase; font-weight: 800; }
    h1 { margin: 0; font-size: clamp(30px, 5vw, 54px); line-height: 1.04; letter-spacing: -.04em; }
    .subtitle { max-width: 720px; margin: 14px 0 0; color: var(--muted); font-size: 15px; }
    .actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 10px; }
    button, .button {
      border: 1px solid var(--line);
      border-radius: 999px;
      background: var(--card-solid);
      color: var(--text);
      padding: 10px 14px;
      cursor: pointer;
      font: inherit;
      font-weight: 700;
      text-decoration: none;
      transition: transform .15s, border-color .15s, background .15s;
    }
    button:hover, .button:hover { transform: translateY(-1px); border-color: color-mix(in srgb, var(--accent) 45%, var(--line)); }
    button.primary { background: linear-gradient(135deg, var(--accent), #0d9488); border-color: transparent; color: white; }
    .status-row { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 18px; }
    .pill { display: inline-flex; align-items: center; gap: 8px; border: 1px solid var(--line); border-radius: 999px; padding: 7px 11px; background: color-mix(in srgb, var(--card-solid) 78%, transparent); color: var(--muted); font-size: 12px; }
    .dot { width: 8px; height: 8px; border-radius: 50%; background: var(--muted); }
    .dot.ok { background: var(--ok); box-shadow: 0 0 0 4px color-mix(in srgb, var(--ok) 16%, transparent); }
    .dot.warn { background: var(--accent-2); box-shadow: 0 0 0 4px color-mix(in srgb, var(--accent-2) 16%, transparent); }
    .dot.err { background: var(--danger); box-shadow: 0 0 0 4px color-mix(in srgb, var(--danger) 16%, transparent); }
    .login-card, .panel {
      margin-top: 18px;
      border: 1px solid var(--line);
      border-radius: 22px;
      background: var(--card);
      box-shadow: var(--shadow);
      backdrop-filter: blur(16px);
    }
    .login-card { padding: 20px; display: none; }
    .login-card.visible { display: block; }
    .login-form { display: flex; gap: 10px; flex-wrap: wrap; }
    input {
      min-width: min(420px, 100%);
      flex: 1;
      border: 1px solid var(--line);
      border-radius: 999px;
      background: var(--card-solid);
      color: var(--text);
      padding: 11px 14px;
      font: inherit;
      outline: none;
    }
    input:focus { border-color: var(--accent); box-shadow: 0 0 0 4px color-mix(in srgb, var(--accent) 16%, transparent); }
    .grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 14px; margin-top: 18px; }
    .card {
      position: relative;
      overflow: hidden;
      min-height: 150px;
      border: 1px solid var(--line);
      border-radius: 22px;
      padding: 20px;
      background: var(--card);
      box-shadow: var(--shadow);
      backdrop-filter: blur(16px);
      animation: rise .36s ease-out both;
    }
    .card:nth-child(2) { animation-delay: .04s; }
    .card:nth-child(3) { animation-delay: .08s; }
    .card:nth-child(4) { animation-delay: .12s; }
    .card:after {
      content: "";
      position: absolute;
      right: -42px;
      bottom: -52px;
      width: 130px;
      height: 130px;
      border-radius: 50%;
      background: color-mix(in srgb, var(--accent) 10%, transparent);
    }
    .card-label { color: var(--muted); font-weight: 700; }
    .card-value { margin-top: 12px; font-size: clamp(28px, 4vw, 42px); line-height: 1; font-weight: 900; letter-spacing: -.04em; }
    .card-meta { margin-top: 12px; color: var(--muted); font-size: 12px; }
    .mini { display: flex; gap: 12px; flex-wrap: wrap; margin-top: 16px; color: var(--muted); font-size: 12px; }
    .mini strong { color: var(--text); }
    .panel { overflow: hidden; }
    .panel-head { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 18px 20px; border-bottom: 1px solid var(--line); }
    .panel-title { margin: 0; font-size: 18px; }
    .table-wrap { overflow-x: auto; }
    table { width: 100%; border-collapse: collapse; min-width: 860px; }
    th, td { padding: 14px 16px; text-align: right; border-bottom: 1px solid var(--line); vertical-align: middle; }
    th:first-child, td:first-child { text-align: left; }
    th { color: var(--muted); font-size: 12px; font-weight: 800; letter-spacing: .04em; text-transform: uppercase; background: color-mix(in srgb, var(--card-solid) 62%, transparent); }
    td { font-variant-numeric: tabular-nums; }
    tbody tr:hover { background: color-mix(in srgb, var(--accent) 7%, transparent); }
    .provider { display: inline-flex; align-items: center; gap: 10px; font-weight: 800; }
    .provider-mark { width: 28px; height: 28px; border-radius: 10px; background: linear-gradient(135deg, var(--accent), var(--accent-2)); box-shadow: inset 0 0 0 1px rgba(255,255,255,.32); }
    .muted { color: var(--muted); }
    .empty { padding: 28px; color: var(--muted); text-align: center; }
    .error { color: var(--danger); }
    @keyframes rise { from { opacity: 0; transform: translateY(12px) scale(.98); } to { opacity: 1; transform: translateY(0) scale(1); } }
    @media (max-width: 980px) { .grid { grid-template-columns: repeat(2, 1fr); } .hero-inner { align-items: flex-start; flex-direction: column; } .actions { justify-content: flex-start; } }
    @media (max-width: 560px) { .shell { width: min(100% - 20px, 1180px); padding-top: 16px; } .hero { padding: 22px; border-radius: 22px; } .grid { grid-template-columns: 1fr; } .login-form { display: block; } .login-form button { margin-top: 10px; width: 100%; } }
  </style>
</head>
<body>
  <div class="shell">
    <section class="hero">
      <div class="hero-inner">
        <div>
          <p class="eyebrow">CLI Proxy API</p>
          <h1>Token 使用统计</h1>
          <p class="subtitle">按 AI 提供商汇总当前进程内的 token 用量，包含总使用、今日、本周和本月数据。统计只在 <code>usage-statistics-enabled</code> 开启后采集。</p>
          <div class="status-row">
            <span class="pill"><span id="status-dot" class="dot warn"></span><span id="status-text">等待认证</span></span>
            <span class="pill">更新时间：<span id="updated-at">-</span></span>
            <span class="pill">统计范围：当前进程内存</span>
          </div>
        </div>
        <div class="actions">
          <a class="button" href="/management.html#/dashboard">返回管理中心</a>
          <button id="refresh-btn" class="primary" type="button">刷新数据</button>
          <button id="clear-key-btn" type="button">清除密钥</button>
        </div>
      </div>
    </section>

    <section id="login-card" class="login-card">
      <form id="login-form" class="login-form">
        <input id="management-key" type="password" autocomplete="current-password" placeholder="输入管理密钥">
        <button class="primary" type="submit">保存并读取统计</button>
      </form>
      <p class="muted">如果你已经在管理中心登录，本页会尽量自动读取同源本地存储中的管理密钥；不同面板版本不兼容时，可在这里手动输入。</p>
    </section>

    <section id="cards" class="grid" aria-live="polite"></section>

    <section class="panel">
      <div class="panel-head">
        <h2 class="panel-title">按 AI 提供商统计</h2>
        <span id="provider-count" class="muted">-</span>
      </div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>提供商</th>
              <th>总 Token</th>
              <th>今日</th>
              <th>本周</th>
              <th>本月</th>
              <th>请求</th>
              <th>失败</th>
              <th>输入</th>
              <th>输出</th>
              <th>推理</th>
            </tr>
          </thead>
          <tbody id="provider-body">
            <tr><td colspan="10" class="empty">等待数据</td></tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>

  <script>
    (function () {
      var storageKey = "cliproxy.tokenUsage.managementKey";
      var apiPath = "/v0/management/token-usage";
      var formatter = new Intl.NumberFormat("zh-CN");
      var key = findInitialKey();

      var loginCard = document.getElementById("login-card");
      var keyInput = document.getElementById("management-key");
      var loginForm = document.getElementById("login-form");
      var refreshBtn = document.getElementById("refresh-btn");
      var clearKeyBtn = document.getElementById("clear-key-btn");
      var statusDot = document.getElementById("status-dot");
      var statusText = document.getElementById("status-text");
      var updatedAt = document.getElementById("updated-at");
      var cards = document.getElementById("cards");
      var providerBody = document.getElementById("provider-body");
      var providerCount = document.getElementById("provider-count");

      keyInput.value = key || "";
      setLoginVisible(!key);
      renderEmptyCards();
      if (key) refresh();

      loginForm.addEventListener("submit", function (event) {
        event.preventDefault();
        key = keyInput.value.trim();
        if (!key) {
          setStatus("err", "请输入管理密钥");
          return;
        }
        localStorage.setItem(storageKey, key);
        setLoginVisible(false);
        refresh();
      });

      refreshBtn.addEventListener("click", function () {
        if (!key) {
          setLoginVisible(true);
          setStatus("warn", "需要管理密钥");
          keyInput.focus();
          return;
        }
        refresh();
      });

      clearKeyBtn.addEventListener("click", function () {
        key = "";
        keyInput.value = "";
        localStorage.removeItem(storageKey);
        setLoginVisible(true);
        setStatus("warn", "已清除密钥");
      });

      setInterval(function () {
        if (key && document.visibilityState === "visible") refresh(true);
      }, 15000);

      async function refresh(silent) {
        if (!silent) setStatus("warn", "正在读取统计");
        refreshBtn.disabled = true;
        try {
          var response = await fetch(apiPath, {
            headers: {
              "Authorization": "Bearer " + key,
              "X-Management-Key": key
            },
            cache: "no-store"
          });
          if (response.status === 401 || response.status === 403) {
            throw new Error("认证失败，请重新输入管理密钥");
          }
          if (response.status === 404) {
            throw new Error("当前服务未提供 token 统计接口，请确认已经启动新编译的 exe");
          }
          if (!response.ok) {
            throw new Error("读取失败 HTTP " + response.status);
          }
          var data = await response.json();
          render(data);
          setStatus(data.enabled ? "ok" : "warn", data.enabled ? "统计已开启" : "统计开关未开启");
          localStorage.setItem(storageKey, key);
          setLoginVisible(false);
        } catch (error) {
          setStatus("err", error.message || "读取失败");
          if (/认证失败/.test(error.message || "")) setLoginVisible(true);
        } finally {
          refreshBtn.disabled = false;
        }
      }

      function render(data) {
        updatedAt.textContent = formatDateTime(data.generated_at);
        var summaries = [
          { title: "总使用", hint: "当前进程累计", bucket: data.total },
          { title: "今日", hint: periodHint(data.day), bucket: data.day },
          { title: "本周", hint: periodHint(data.week), bucket: data.week },
          { title: "本月", hint: periodHint(data.month), bucket: data.month }
        ];
        cards.innerHTML = summaries.map(function (item) {
          var bucket = normalizeBucket(item.bucket);
          return '<article class="card"><div class="card-label">' + escapeHtml(item.title) + '</div><div class="card-value">' + number(bucket.tokens.total_tokens) + '</div><div class="card-meta">' + escapeHtml(item.hint) + '</div><div class="mini"><span>请求 <strong>' + number(bucket.requests) + '</strong></span><span>失败 <strong>' + number(bucket.failed_requests) + '</strong></span><span>输出 <strong>' + number(bucket.tokens.output_tokens) + '</strong></span></div></article>';
        }).join("");

        var providers = Array.isArray(data.providers) ? data.providers.slice() : [];
        providers.sort(function (left, right) {
          return tokenTotal(right.total) - tokenTotal(left.total) || String(left.provider).localeCompare(String(right.provider));
        });
        providerCount.textContent = providers.length ? providers.length + " 个提供商" : "暂无提供商";
        if (!providers.length) {
          providerBody.innerHTML = '<tr><td colspan="10" class="empty">还没有 token 使用记录。请通过代理发起一次模型请求后再刷新。</td></tr>';
          return;
        }
        providerBody.innerHTML = providers.map(function (provider) {
          var total = normalizeBucket(provider.total);
          return '<tr><td><span class="provider"><span class="provider-mark"></span>' + escapeHtml(provider.provider || "unknown") + '</span></td><td>' + number(total.tokens.total_tokens) + '</td><td>' + number(tokenTotal(provider.day)) + '</td><td>' + number(tokenTotal(provider.week)) + '</td><td>' + number(tokenTotal(provider.month)) + '</td><td>' + number(total.requests) + '</td><td>' + number(total.failed_requests) + '</td><td>' + number(total.tokens.input_tokens) + '</td><td>' + number(total.tokens.output_tokens) + '</td><td>' + number(total.tokens.reasoning_tokens) + '</td></tr>';
        }).join("");
      }

      function renderEmptyCards() {
        cards.innerHTML = ["总使用", "今日", "本周", "本月"].map(function (title) {
          return '<article class="card"><div class="card-label">' + title + '</div><div class="card-value">0</div><div class="card-meta">等待数据</div><div class="mini"><span>请求 <strong>0</strong></span><span>失败 <strong>0</strong></span></div></article>';
        }).join("");
      }

      function findInitialKey() {
        var params = new URLSearchParams(window.location.search);
        var fromURL = (params.get("key") || params.get("management_key") || "").trim();
        if (fromURL) {
          localStorage.setItem(storageKey, fromURL);
          window.history.replaceState(null, document.title, window.location.pathname);
          return fromURL;
        }
        var direct = readDirectStorageKey();
        if (direct) return direct;
        return scanStorageForManagementKey(localStorage) || scanStorageForManagementKey(sessionStorage) || "";
      }

      function readDirectStorageKey() {
        var names = [storageKey, "managementKey", "management-key", "management_key", "cliproxy-management-key", "cpa-management-key"];
        for (var i = 0; i < names.length; i++) {
          var value = localStorage.getItem(names[i]) || sessionStorage.getItem(names[i]);
          value = cleanKey(value);
          if (value) return value;
        }
        return "";
      }

      function scanStorageForManagementKey(storage) {
        try {
          for (var i = 0; i < storage.length; i++) {
            var name = storage.key(i);
            var value = storage.getItem(name);
            if (/management.*key/i.test(name || "")) {
              var direct = cleanKey(value);
              if (direct && direct.charAt(0) !== "{" && direct.charAt(0) !== "[") return direct;
            }
            var found = extractKey(tryJSON(value), 0);
            if (found) return found;
          }
        } catch (error) {
          return "";
        }
        return "";
      }

      function extractKey(value, depth) {
        if (!value || depth > 6) return "";
        if (typeof value !== "object") return "";
        var preferred = ["managementKey", "management_key", "management-key"];
        for (var i = 0; i < preferred.length; i++) {
          var candidate = cleanKey(value[preferred[i]]);
          if (candidate) return candidate;
        }
        if (value.state) {
          var nested = extractKey(value.state, depth + 1);
          if (nested) return nested;
        }
        for (var keyName in value) {
          if (!Object.prototype.hasOwnProperty.call(value, keyName)) continue;
          if (/management.*key/i.test(keyName)) {
            var direct = cleanKey(value[keyName]);
            if (direct) return direct;
          }
          var found = extractKey(value[keyName], depth + 1);
          if (found) return found;
        }
        return "";
      }

      function tryJSON(value) {
        if (!value || typeof value !== "string") return null;
        try { return JSON.parse(value); } catch (error) { return null; }
      }

      function cleanKey(value) {
        if (typeof value !== "string") return "";
        value = value.trim();
        if ((value.charAt(0) === '"' && value.charAt(value.length - 1) === '"') || (value.charAt(0) === "'" && value.charAt(value.length - 1) === "'")) {
          value = value.slice(1, -1).trim();
        }
        return value;
      }

      function normalizeBucket(bucket) {
        bucket = bucket || {};
        bucket.tokens = bucket.tokens || {};
        return {
          requests: bucket.requests || 0,
          successful_requests: bucket.successful_requests || 0,
          failed_requests: bucket.failed_requests || 0,
          tokens: {
            input_tokens: bucket.tokens.input_tokens || 0,
            output_tokens: bucket.tokens.output_tokens || 0,
            reasoning_tokens: bucket.tokens.reasoning_tokens || 0,
            cached_tokens: bucket.tokens.cached_tokens || 0,
            cache_read_tokens: bucket.tokens.cache_read_tokens || 0,
            cache_creation_tokens: bucket.tokens.cache_creation_tokens || 0,
            total_tokens: bucket.tokens.total_tokens || 0
          }
        };
      }

      function tokenTotal(bucket) {
        return normalizeBucket(bucket).tokens.total_tokens;
      }

      function periodHint(bucket) {
        if (!bucket) return "当前周期";
        return bucket.label || "当前周期";
      }

      function formatDateTime(value) {
        if (!value) return "-";
        var date = new Date(value);
        if (Number.isNaN(date.getTime())) return "-";
        return date.toLocaleString();
      }

      function number(value) {
        return formatter.format(Number(value) || 0);
      }

      function escapeHtml(value) {
        return String(value == null ? "" : value).replace(/[&<>"']/g, function (char) {
          return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[char];
        });
      }

      function setLoginVisible(visible) {
        loginCard.classList.toggle("visible", !!visible);
      }

      function setStatus(type, text) {
        statusDot.className = "dot " + type;
        statusText.textContent = text;
      }
    })();
  </script>
</body>
</html>`
