package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) serveManagementTokenUsagePage(c *gin.Context) {
	cfg := s.getConfig()
	if cfg == nil || cfg.Home.Enabled || cfg.RemoteManagement.DisableControlPanel {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(managementTokenUsageHTML))
}

func injectManagementTokenUsageEntry(data []byte) []byte {
	if len(data) == 0 {
		return []byte(managementTokenUsageEntryScript)
	}
	html := string(data)
	if strings.Contains(html, "cliproxy-token-usage-entry") {
		return data
	}
	lower := strings.ToLower(html)
	index := strings.LastIndex(lower, "</body>")
	if index < 0 {
		return []byte(html + managementTokenUsageEntryScript)
	}
	return []byte(html[:index] + managementTokenUsageEntryScript + html[index:])
}

const managementTokenUsageEntryScript = `<script id="cliproxy-token-usage-entry">
(function () {
  if (window.__cliproxyTokenUsageEntryMounted) return;
  window.__cliproxyTokenUsageEntryMounted = true;

  var route = "/dashboard";
  var viewMarker = "cliproxy-view=token-usage";
  var href = "#/dashboard?" + viewMarker;
  var apiPath = "/token-usage";
  var syncTimer = null;
  var refreshTimer = null;
  var retryTimer = null;
  var active = false;
  var loading = false;
  var lastData = null;

  var style = document.createElement("style");
  style.textContent = "#cliproxy-token-usage-nav .cliproxy-token-usage-dot{width:18px;height:18px;border-radius:999px;background:var(--primary-color,#2563eb);display:inline-block;box-shadow:inset 0 0 0 2px rgba(255,255,255,.45)}#cliproxy-token-usage-view{width:100%;animation:cliproxy-token-usage-rise .2s ease-out both}#cliproxy-token-usage-view .tu-page{display:grid;gap:16px}#cliproxy-token-usage-view .tu-hero,#cliproxy-token-usage-view .tu-card,#cliproxy-token-usage-view .tu-panel{border:1px solid var(--border-color,#e5e7eb);border-radius:12px;background:var(--bg-primary,#fff)}#cliproxy-token-usage-view .tu-hero{display:flex;align-items:flex-end;justify-content:space-between;gap:18px;padding:22px}#cliproxy-token-usage-view .tu-eyebrow{margin:0 0 5px;color:var(--primary-color,#2563eb);font-size:12px;font-weight:700;letter-spacing:.08em;text-transform:uppercase}#cliproxy-token-usage-view h1{margin:0;color:var(--text-primary,#111827);font-size:30px}#cliproxy-token-usage-view .tu-subtitle{margin:8px 0 0;color:var(--text-secondary,#6b7280)}#cliproxy-token-usage-view .tu-actions{display:flex;align-items:center;gap:10px;flex-wrap:wrap}#cliproxy-token-usage-view button{border:0;border-radius:8px;background:var(--primary-color,#2563eb);color:var(--primary-contrast,#fff);padding:9px 14px;cursor:pointer;font:inherit;font-weight:600}#cliproxy-token-usage-view button:disabled{cursor:wait;opacity:.65}#cliproxy-token-usage-view .tu-status{margin-top:10px;color:var(--text-secondary,#6b7280);font-size:12px}#cliproxy-token-usage-view .tu-status.ok{color:#16a34a}#cliproxy-token-usage-view .tu-status.err{color:#dc2626}#cliproxy-token-usage-view .tu-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px}#cliproxy-token-usage-view .tu-card{padding:17px}#cliproxy-token-usage-view .tu-label{color:var(--text-secondary,#6b7280);font-size:13px;font-weight:600}#cliproxy-token-usage-view .tu-value{margin-top:9px;color:var(--text-primary,#111827);font-size:30px;font-weight:800;font-variant-numeric:tabular-nums}#cliproxy-token-usage-view .tu-meta,#cliproxy-token-usage-view .tu-mini{margin-top:8px;color:var(--text-secondary,#6b7280);font-size:12px}#cliproxy-token-usage-view .tu-mini{display:flex;gap:12px;flex-wrap:wrap}#cliproxy-token-usage-view .tu-panel{overflow:hidden}#cliproxy-token-usage-view .tu-panel-head{display:flex;align-items:center;justify-content:space-between;padding:15px 17px;border-bottom:1px solid var(--border-color,#e5e7eb)}#cliproxy-token-usage-view .tu-panel h2{margin:0;color:var(--text-primary,#111827);font-size:17px}#cliproxy-token-usage-view .tu-table-wrap{overflow-x:auto}#cliproxy-token-usage-view table{width:100%;min-width:850px;border-collapse:collapse}#cliproxy-token-usage-view th,#cliproxy-token-usage-view td{padding:12px 13px;border-bottom:1px solid var(--border-color,#e5e7eb);text-align:right;font-variant-numeric:tabular-nums}#cliproxy-token-usage-view th:first-child,#cliproxy-token-usage-view td:first-child{text-align:left}#cliproxy-token-usage-view th{color:var(--text-secondary,#6b7280);font-size:12px;background:var(--bg-secondary,#f9fafb)}#cliproxy-token-usage-view .tu-provider{font-weight:700;color:var(--text-primary,#111827)}#cliproxy-token-usage-view .muted,#cliproxy-token-usage-view .empty{color:var(--text-secondary,#6b7280)}#cliproxy-token-usage-view .empty{padding:24px;text-align:center}@keyframes cliproxy-token-usage-rise{from{opacity:0;transform:translateY(6px)}to{opacity:1;transform:translateY(0)}}@media(max-width:900px){#cliproxy-token-usage-view .tu-grid{grid-template-columns:repeat(2,minmax(0,1fr))}#cliproxy-token-usage-view .tu-hero{align-items:flex-start;flex-direction:column}}@media(max-width:560px){#cliproxy-token-usage-view .tu-grid{grid-template-columns:1fr}}";
  document.head.appendChild(style);

  function currentRoute() {
    return (window.location.hash || "").replace(/^#/, "").split("?")[0].replace(/\/$/, "");
  }

  function isActive() {
    return currentRoute() === route && (window.location.hash || "").indexOf(viewMarker) !== -1;
  }

  function contentHost() {
    return document.querySelector(".main-content") || document.querySelector("main") || null;
  }

  function addSidebar() {
    if (document.getElementById("cliproxy-token-usage-nav")) return;
    var section = document.querySelector(".nav-section");
    if (!section) return;
    var group = document.createElement("div");
    group.id = "cliproxy-token-usage-nav";
    group.className = "nav-group";
    group.innerHTML = '<a class="nav-item" href="' + href + '" title="Token 统计"><span class="nav-icon"><span class="cliproxy-token-usage-dot"></span></span><span class="nav-text"><span class="nav-label">Token 统计</span><span class="nav-meta">按提供商查看用量</span></span></a>';
    section.appendChild(group);
  }

  function setNavActive(value) {
    var link = document.querySelector("#cliproxy-token-usage-nav .nav-item");
    if (link) link.classList.toggle("active", !!value);
    if (value) {
      Array.prototype.forEach.call(document.querySelectorAll(".nav-item.active"), function (item) {
        if (item !== link) item.classList.remove("active");
      });
    }
  }

  function hideOriginalContent(host, root) {
    Array.prototype.forEach.call(host.children, function (child) {
      if (child === root) return;
      if (!child.hasAttribute("data-cliproxy-token-usage-old-display")) {
        child.setAttribute("data-cliproxy-token-usage-old-display", child.style.display || "");
      }
      if (child.style.display !== "none") child.style.display = "none";
    });
  }

  function restoreOriginalContent() {
    Array.prototype.forEach.call(document.querySelectorAll("[data-cliproxy-token-usage-old-display]"), function (element) {
      element.style.display = element.getAttribute("data-cliproxy-token-usage-old-display") || "";
      element.removeAttribute("data-cliproxy-token-usage-old-display");
    });
  }

  function createView(host) {
    var root = document.createElement("section");
    root.id = "cliproxy-token-usage-view";
    root.innerHTML = '<div class="tu-page"><section class="tu-hero"><div><p class="tu-eyebrow">CLI Proxy API</p><h1>Token 使用统计</h1><p class="tu-subtitle">统计数据来自本地 SQLite，管理中心会复用当前登录状态，不需要重新输入密钥。</p><div id="tu-status" class="tu-status">正在读取统计</div></div><div class="tu-actions"><button id="tu-refresh" type="button">刷新数据</button></div></section><section id="tu-cards" class="tu-grid"></section><section class="tu-panel"><div class="tu-panel-head"><h2>按 AI 提供商统计</h2><span id="tu-provider-count" class="muted">-</span></div><div class="tu-table-wrap"><table><thead><tr><th>提供商</th><th>总 Token</th><th>今日</th><th>本周</th><th>本月</th><th>请求</th><th>失败</th><th>输入</th><th>输出</th><th>推理</th></tr></thead><tbody id="tu-provider-body"><tr><td colspan="10" class="empty">等待数据</td></tr></tbody></table></div></section></div>';
    host.appendChild(root);
    root.querySelector("#tu-refresh").addEventListener("click", function () {
      refresh(root, false);
    });
    renderEmptyCards(root);
    if (lastData) renderData(root, lastData);
    return root;
  }

  function ensureView() {
    var host = contentHost();
    if (!host) return null;
    var root = document.getElementById("cliproxy-token-usage-view");
    if (!root || root.parentElement !== host) {
      if (root) root.remove();
      root = createView(host);
    }
    hideOriginalContent(host, root);
    return root;
  }

  function clearView() {
    var root = document.getElementById("cliproxy-token-usage-view");
    if (root) root.remove();
    restoreOriginalContent();
    setNavActive(false);
  }

  function scheduleSync() {
    if (syncTimer !== null) return;
    syncTimer = setTimeout(syncRoute, 80);
  }

  function syncRoute() {
    syncTimer = null;
    addSidebar();
    if (!isActive()) {
      if (active) {
        active = false;
        if (refreshTimer !== null) clearInterval(refreshTimer);
        refreshTimer = null;
      }
      clearView();
      return;
    }

    var root = ensureView();
    setNavActive(true);
    if (!root) {
      syncTimer = setTimeout(syncRoute, 160);
      return;
    }
    if (!active) {
      active = true;
      refresh(root, false);
      refreshTimer = setInterval(function () {
        if (active && document.visibilityState === "visible") {
          var current = document.getElementById("cliproxy-token-usage-view");
          if (current) refresh(current, true);
        }
      }, 15000);
    }
  }

  function managementClient() {
    return window.__cliproxyManagementClient;
  }

  function refresh(root, silent) {
    if (!root || loading) return;
    var api = managementClient();
    if (!api || typeof api.get !== "function") {
      setStatus(root, "", "正在等待管理中心完成初始化");
      if (retryTimer !== null) clearTimeout(retryTimer);
      retryTimer = setTimeout(function () {
        retryTimer = null;
        if (active) refresh(document.getElementById("cliproxy-token-usage-view"), true);
      }, 180);
      return;
    }

    loading = true;
    var button = root.querySelector("#tu-refresh");
    if (button) button.disabled = true;
    if (!silent) setStatus(root, "", "正在读取统计");
    api.get(apiPath).then(function (data) {
      lastData = data || {};
      var current = document.getElementById("cliproxy-token-usage-view");
      if (current) renderData(current, lastData);
      setStatus(current, lastData.enabled ? "ok" : "", lastData.enabled ? "统计已开启" : "统计开关未开启");
    }).catch(function (error) {
      setStatus(document.getElementById("cliproxy-token-usage-view"), "err", error && error.message ? error.message : "读取统计失败");
    }).finally(function () {
      loading = false;
      var current = document.getElementById("cliproxy-token-usage-view");
      var currentButton = current && current.querySelector("#tu-refresh");
      if (currentButton) currentButton.disabled = false;
    });
  }

  function renderData(root, data) {
    if (!root) return;
    var summaries = [
      { title: "总使用", hint: "SQLite 历史累计", bucket: data.total },
      { title: "今日", hint: periodHint(data.day), bucket: data.day },
      { title: "本周", hint: periodHint(data.week), bucket: data.week },
      { title: "本月", hint: periodHint(data.month), bucket: data.month }
    ];
    root.querySelector("#tu-cards").innerHTML = summaries.map(function (item) {
      var bucket = normalizeBucket(item.bucket);
      return '<article class="tu-card"><div class="tu-label">' + escapeHtml(item.title) + '</div><div class="tu-value">' + number(bucket.tokens.total_tokens) + '</div><div class="tu-meta">' + escapeHtml(item.hint) + '</div><div class="tu-mini"><span>请求 <strong>' + number(bucket.requests) + '</strong></span><span>失败 <strong>' + number(bucket.failed_requests) + '</strong></span><span>输出 <strong>' + number(bucket.tokens.output_tokens) + '</strong></span></div></article>';
    }).join("");

    var providers = Array.isArray(data.providers) ? data.providers.slice() : [];
    providers.sort(function (left, right) {
      return tokenTotal(right.total) - tokenTotal(left.total) || String(left.provider).localeCompare(String(right.provider));
    });
    root.querySelector("#tu-provider-count").textContent = providers.length ? providers.length + " 个提供商" : "暂无提供商";
    var body = root.querySelector("#tu-provider-body");
    if (!providers.length) {
      body.innerHTML = '<tr><td colspan="10" class="empty">还没有 Token 使用记录</td></tr>';
      return;
    }
    body.innerHTML = providers.map(function (provider) {
      var total = normalizeBucket(provider.total);
      return '<tr><td><span class="tu-provider">' + escapeHtml(provider.provider || "unknown") + '</span></td><td>' + number(total.tokens.total_tokens) + '</td><td>' + number(tokenTotal(provider.day)) + '</td><td>' + number(tokenTotal(provider.week)) + '</td><td>' + number(tokenTotal(provider.month)) + '</td><td>' + number(total.requests) + '</td><td>' + number(total.failed_requests) + '</td><td>' + number(total.tokens.input_tokens) + '</td><td>' + number(total.tokens.output_tokens) + '</td><td>' + number(total.tokens.reasoning_tokens) + '</td></tr>';
    }).join("");
  }

  function renderEmptyCards(root) {
    root.querySelector("#tu-cards").innerHTML = ["总使用", "今日", "本周", "本月"].map(function (title) {
      return '<article class="tu-card"><div class="tu-label">' + title + '</div><div class="tu-value">0</div><div class="tu-meta">等待数据</div></article>';
    }).join("");
  }

  function normalizeBucket(bucket) {
    bucket = bucket || {};
    var tokens = bucket.tokens || {};
    return {
      requests: bucket.requests || 0,
      failed_requests: bucket.failed_requests || 0,
      tokens: {
        input_tokens: tokens.input_tokens || 0,
        output_tokens: tokens.output_tokens || 0,
        reasoning_tokens: tokens.reasoning_tokens || 0,
        total_tokens: tokens.total_tokens || 0
      }
    };
  }

  function tokenTotal(bucket) {
    return normalizeBucket(bucket).tokens.total_tokens;
  }

  function periodHint(bucket) {
    return bucket && bucket.label ? bucket.label : "当前周期";
  }

  function number(value) {
    return new Intl.NumberFormat("zh-CN").format(Number(value) || 0);
  }

  function escapeHtml(value) {
    return String(value == null ? "" : value).replace(/[&<>"']/g, function (character) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[character];
    });
  }

  function setStatus(root, type, message) {
    if (!root) return;
    var status = root.querySelector("#tu-status");
    if (!status) return;
    status.className = "tu-status" + (type ? " " + type : "");
    if (status.textContent !== message) status.textContent = message;
  }

  window.addEventListener("hashchange", scheduleSync);
  window.addEventListener("popstate", scheduleSync);
  var observer = new MutationObserver(scheduleSync);
  observer.observe(document.documentElement, { childList: true, subtree: true });
  scheduleSync();
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
          <p class="subtitle">按 AI 提供商汇总本地累计的 token 用量，包含总使用、今日、本周和本月数据。统计只在 <code>usage-statistics-enabled</code> 开启后采集；统计数据存储在本地 SQLite 数据库中。</p>
          <div class="status-row">
            <span class="pill"><span id="status-dot" class="dot warn"></span><span id="status-text">等待认证</span></span>
            <span class="pill">更新时间：<span id="updated-at">-</span></span>
            <span class="pill">统计范围：本地 SQLite</span>
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
      var apiPath = "/v8/management/observability/usage/token-usage";
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
          { title: "总使用", hint: "SQLite 历史累计", bucket: data.total },
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
