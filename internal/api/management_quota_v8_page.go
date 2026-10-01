package api

import (
	"regexp"
	"strings"
)

var (
	quotaV8PageHook   = regexp.MustCompile(`\[([\w$]+),([\w$]+)\]=\(0,([\w$]+)\.useState\)\(1\)`)
	quotaV8Pagination = regexp.MustCompile(`\(\(\)=>[\w$]+\(([\w$]+),([\w$]+),20\),\[([\w$]+),([\w$]+)\]\)`)
	quotaV8Sort       = regexp.MustCompile("\\(0,([\\w$]+)\\.jsx\\)\\(([\\w$]+),\\{value:([\\w$]+),options:([\\w$]+),onChange:([\\w$]+),ariaLabel:([\\w$]+)\\(`quota_management.sort_label`\\),size:`sm`\\}\\)")
)

// patchManagementQuotaV8Page updates the quota component atomically. A future
// bundle with different anchors is served unchanged rather than partially patched.
func patchManagementQuotaV8Page(html string) string {
	if strings.Contains(html, "setCliproxyQuotaPageSize") {
		return html
	}
	marker := strings.Index(html, "quota_management.sort_label")
	if marker < 0 {
		return html
	}
	start := strings.LastIndex(html[:marker], "function ")
	if start < 0 {
		return html
	}
	end := len(html)
	if next := strings.Index(html[marker:], "function "); next >= 0 {
		end = marker + next
	}
	component := html[start:end]
	hook := quotaV8PageHook.FindStringSubmatch(component)
	pagination := quotaV8Pagination.FindStringSubmatch(component)
	sort := quotaV8Sort.FindStringSubmatch(component)
	if len(hook) != 4 || len(pagination) != 5 || len(sort) != 7 || pagination[2] != hook[1] || pagination[1] != pagination[3] || pagination[2] != pagination[4] {
		return html
	}
	component = strings.Replace(component, hook[0], hook[0]+",[cliproxyQuotaPageSize,setCliproxyQuotaPageSize]=(0,"+hook[3]+".useState)(50)", 1)
	page := strings.Replace(pagination[0], ",20)", ",cliproxyQuotaPageSize)", 1)
	page = strings.Replace(page, ",["+pagination[3]+","+pagination[4]+"])", ",["+pagination[3]+","+pagination[4]+",cliproxyQuotaPageSize])", 1)
	component = strings.Replace(component, pagination[0], page, 1)
	jsx := "(0," + sort[1] + ".jsx)"
	control := "(0," + sort[1] + ".jsxs)(`div`,{style:{display:`flex`,gap:8,alignItems:`center`},children:[" + sort[0] + "," + jsx + "(`select`,{value:cliproxyQuotaPageSize,\"aria-label\":`Per page`,onChange:e=>{let n=Number(e.target.value);[20,30,50,100].includes(n)&&(setCliproxyQuotaPageSize(n)," + hook[2] + "(1))},children:[20,30,50,100].map(n=>" + jsx + "(`option`,{value:n,children:n},n))})]})"
	component = strings.Replace(component, sort[0], control, 1)
	return html[:start] + component + html[end:]
}
