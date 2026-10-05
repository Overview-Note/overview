package sitegen

import "html/template"

var pageTmpl = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en" data-theme="{{.Theme}}">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>{{.Title}} · {{.SiteTitle}}</title>
<link rel="stylesheet" href="{{.Base}}style.css" />
<link rel="stylesheet" href="{{.Base}}content.css" />
{{if .HasMath}}<link rel="stylesheet" href="{{.Base}}vendor/katex/katex.min.css" />
{{end}}</head>
<body data-base="{{.Base}}">
<header class="topbar">
  <a class="brand" href="{{.Base}}index.html">
    <span class="mark"><svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M14.5 3H7.2A2.2 2.2 0 0 0 5 5.2v13.6A2.2 2.2 0 0 0 7.2 21h9.6a2.2 2.2 0 0 0 2.2-2.2V7.5Z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/><path d="M14.5 3v3.1a1.4 1.4 0 0 0 1.4 1.4H19" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/><path d="M8.5 11h3.5M8.5 14.5h7M8.5 18h7" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg></span><span>{{.SiteTitle}}</span>
  </a>
  <div class="search-box">
    <input id="search" type="search" placeholder="Search" autocomplete="off" aria-label="Search" />
    <div id="search-results" class="search-results"></div>
  </div>
  <a class="home-link" href="{{.Base}}index.html">All notes</a>
</header>
<div class="layout">
  <nav class="sidebar">
    {{range .Nav}}<a class="nav-item" href="{{.Href}}">{{.Title}}</a>
    {{end}}
  </nav>
  <main class="content">
    <div class="crumbs">Notes / {{.Title}}</div>
    <article class="doc tiptap-content">{{.Content}}</article>
  </main>
  {{if .Headings}}
  <aside class="toc">
    <h4>On this page</h4>
    {{range .Headings}}<a class="toc-item lvl-{{.Level}}" href="#{{.ID}}">{{.Text}}</a>
    {{end}}
  </aside>
  {{end}}
</div>
<script src="{{.Base}}search.js" defer></script>
{{if .HasCode}}<script src="{{.Base}}vendor/highlight/highlight.min.js" defer></script>
{{end}}{{if .HasMath}}<script src="{{.Base}}vendor/katex/katex.min.js" defer></script>
{{end}}{{if .HasMermaid}}<script src="{{.Base}}vendor/mermaid/mermaid.min.js" defer></script>
{{end}}<script src="{{.Base}}readonly-enhance.js" defer></script>
</body>
</html>
`))

var indexTmpl = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="en" data-theme="{{.Theme}}">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>{{.SiteTitle}}</title>
<link rel="stylesheet" href="{{.Base}}style.css" />
<link rel="stylesheet" href="{{.Base}}content.css" />
</head>
<body data-base="{{.Base}}">
<header class="topbar">
  <a class="brand" href="{{.Base}}index.html">
    <span class="mark"><svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M14.5 3H7.2A2.2 2.2 0 0 0 5 5.2v13.6A2.2 2.2 0 0 0 7.2 21h9.6a2.2 2.2 0 0 0 2.2-2.2V7.5Z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/><path d="M14.5 3v3.1a1.4 1.4 0 0 0 1.4 1.4H19" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/><path d="M8.5 11h3.5M8.5 14.5h7M8.5 18h7" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg></span><span>{{.SiteTitle}}</span>
  </a>
  <div class="search-box">
    <input id="search" type="search" placeholder="Search" autocomplete="off" aria-label="Search" />
    <div id="search-results" class="search-results"></div>
  </div>
</header>
<div class="layout">
  <nav class="sidebar">
    {{range .Nav}}<a class="nav-item" href="{{.Href}}">{{.Title}}</a>
    {{end}}
  </nav>
  <main class="content">
    <h1 class="home-title">{{.SiteTitle}}</h1>
    <ul class="note-list">
      {{range .Pages}}<li><a href="{{$.Base}}{{.Slug}}.html">{{.Title}}</a></li>
      {{end}}
    </ul>
  </main>
</div>
<script src="{{.Base}}search.js" defer></script>
</body>
</html>
`))

const siteCSS = `:root{
  --bg:#ffffff;--bg-panel:#fafaf8;--bg-soft:#f5f3f0;--bg-hover:#f5f3f0;--border:#e8e8e6;--border-strong:#d4d4d2;
  --text:#37352f;--text-strong:#1a1a1a;--text-muted:#6b6b6b;--text-soft:#9b9b9b;--accent:#d4870e;--accent-soft:#fdf4e3;
  --code-bg:#f7f6f3;--font-mono:ui-monospace,"SFMono-Regular",Consolas,Menlo,monospace;
  --font-sans:-apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",Roboto,Helvetica,Arial,sans-serif;
}
:root[data-theme="dark"]{
  --bg:#1f1e1c;--bg-panel:#262523;--bg-soft:#2d2b28;--bg-hover:#33312d;--border:#33312d;--border-strong:#454340;
  --text:#e9e6e0;--text-strong:#f7f5f0;--text-muted:#b3afa8;--text-soft:#918c85;--accent:#e9a23b;--accent-soft:#3a2f16;--code-bg:#26241f;
}
@media (prefers-color-scheme:dark){:root[data-theme="auto"]{
  --bg:#1f1e1c;--bg-panel:#262523;--bg-soft:#2d2b28;--bg-hover:#33312d;--border:#33312d;--border-strong:#454340;
  --text:#e9e6e0;--text-strong:#f7f5f0;--text-muted:#b3afa8;--text-soft:#918c85;--accent:#e9a23b;--accent-soft:#3a2f16;--code-bg:#26241f;
}}
*{box-sizing:border-box}
html,body{margin:0}
body{font-family:var(--font-sans);color:var(--text);background:var(--bg);font-size:var(--base-size);line-height:1.7;-webkit-font-smoothing:antialiased}
a{color:var(--accent);text-decoration:none}a:hover{text-decoration:underline}
.topbar{display:flex;align-items:center;gap:16px;height:52px;padding:0 20px;border-bottom:1px solid var(--border);background:var(--bg);position:sticky;top:0;z-index:10}
.brand{display:flex;align-items:center;gap:10px;color:var(--text);font-weight:750;font-size:18px;letter-spacing:-.02em}
.mark{width:30px;height:30px;border-radius:9px;background:linear-gradient(145deg,#e9a23b,#d4870e);color:#fff;display:inline-flex;align-items:center;justify-content:center}
.mark svg{width:62%;height:62%;display:block}
.home-link{margin-left:auto;font-size:14px;color:var(--text-muted)}
.search-box{margin-left:auto;position:relative;width:min(360px,40vw)}
.search-box input{width:100%;padding:6px 12px;border:1px solid var(--border);border-radius:8px;background:var(--bg-panel);color:var(--text);font:inherit;font-size:14px;outline:none}
.search-box input:focus{border-color:var(--accent);background:var(--bg)}
.search-results{position:absolute;top:calc(100% + 6px);left:0;right:0;background:var(--bg);border:1px solid var(--border);border-radius:10px;box-shadow:0 12px 32px rgba(0,0,0,.14);max-height:60vh;overflow:auto;display:none;z-index:20}
.search-results.open{display:block}
.search-results a{display:block;padding:8px 12px;border-bottom:1px solid var(--border);color:var(--text);text-decoration:none}
.search-results a:last-child{border-bottom:none}
.search-results a:hover{background:var(--bg-hover)}
.search-results .sr-title{font-weight:600;font-size:14px}
.search-results .sr-snippet{display:block;color:var(--text-soft);font-size:12.5px;margin-top:2px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.search-results .sr-empty{padding:10px 12px;color:var(--text-soft);font-size:13px}
@media(max-width:860px){.search-box{width:auto;flex:1}}
.layout{display:flex;align-items:flex-start;max-width:1240px;margin:0 auto}
.sidebar{position:sticky;top:52px;width:240px;flex-shrink:0;padding:24px 12px;max-height:calc(100vh - 52px);overflow:auto}
.nav-item{display:block;padding:5px 10px;border-left:2px solid transparent;border-radius:6px;color:var(--text-muted);font-size:14px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.nav-item:hover{background:var(--bg-hover);color:var(--text);text-decoration:none}
.nav-item.active{background:var(--accent-soft);border-left-color:var(--accent);color:var(--accent)}
.content{flex:1;min-width:0;padding:40px 48px 120px}
.crumbs{font-family:var(--font-mono);font-size:12px;color:var(--text-soft);margin-bottom:16px}
.home-title{font-size:2.1em;margin:0 0 20px;letter-spacing:-.02em;color:var(--text-strong)}
.note-list{list-style:none;padding:0;display:flex;flex-direction:column;gap:6px}
.note-list a{font-size:var(--base-size)}
.doc{max-width:760px}
.toc{position:sticky;top:52px;width:220px;flex-shrink:0;padding:40px 16px;max-height:calc(100vh - 52px);overflow:auto}
.toc h4{margin:0 0 8px;font-size:12px;text-transform:uppercase;letter-spacing:.06em;color:var(--text-soft);font-weight:650}
.toc-item{display:block;padding:3px 10px;border-left:2px solid transparent;color:var(--text-muted);font-size:13.5px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.toc-item:hover{color:var(--text);text-decoration:none}
.toc-item.lvl-3{padding-left:22px}
.toc-item.lvl-4{padding-left:34px}
@media(max-width:1100px){.toc{display:none}}
@media(max-width:860px){.sidebar{display:none}.content{padding:28px 20px 100px}}
`

// siteJS is a small dependency-free client-side search over search-index.json.
const siteJS = `(function () {
  var input = document.getElementById('search');
  var box = document.getElementById('search-results');
  if (!input || !box) return;

  var index = [];
  var base = document.body.getAttribute('data-base') || '';
  fetch(base + 'search-index.json')
    .then(function (r) { return r.json(); })
    .then(function (data) { index = data; })
    .catch(function () { index = []; });

  var active = -1;

  function render(query) {
    var q = query.trim().toLowerCase();
    box.innerHTML = '';
    active = -1;
    if (!q) { box.classList.remove('open'); return; }
    var matches = [];
    for (var i = 0; i < index.length && matches.length < 12; i++) {
      var e = index[i];
      var title = e.title || '';
      var text = e.text || '';
      var ti = title.toLowerCase().indexOf(q);
      var xi = text.toLowerCase().indexOf(q);
      if (ti === -1 && xi === -1) continue;
      var snippet = '';
      if (xi !== -1) {
        var start = Math.max(0, xi - 40);
        snippet = (start > 0 ? '…' : '') + text.slice(start, start + 120);
        if (start + 120 < text.length) snippet += '…';
      }
      matches.push({ url: e.url, title: title, snippet: snippet });
    }
    if (matches.length === 0) {
      box.innerHTML = '<div class="sr-empty">No results</div>';
      box.classList.add('open');
      return;
    }
    matches.forEach(function (m) {
      var a = document.createElement('a');
      a.href = m.url;
      var t = document.createElement('span');
      t.className = 'sr-title';
      t.textContent = m.title;
      a.appendChild(t);
      if (m.snippet) {
        var s = document.createElement('span');
        s.className = 'sr-snippet';
        s.textContent = m.snippet;
        a.appendChild(s);
      }
      box.appendChild(a);
    });
    box.classList.add('open');
  }

  input.addEventListener('input', function () { render(input.value); });
  input.addEventListener('focus', function () { if (input.value) render(input.value); });
  input.addEventListener('keydown', function (ev) {
    var links = box.querySelectorAll('a');
    if (ev.key === 'Down') { active = Math.min(active + 1, links.length - 1); }
    else if (ev.key === 'Up') { active = Math.max(active - 1, 0); }
    else if (ev.key === 'Enter') { if (links[active]) window.location.href = links[active].href; return; }
    else if (ev.key === 'Escape') { box.classList.remove('open'); return; }
    else { return; }
    ev.preventDefault();
    links.forEach(function (l, i) { l.style.background = i === active ? 'var(--hover)' : ''; });
  });
  document.addEventListener('click', function (ev) {
    if (!box.contains(ev.target) && ev.target !== input) box.classList.remove('open');
  });
})();
`
