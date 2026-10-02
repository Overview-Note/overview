package sitegen

import "html/template"

var pageTmpl = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>{{.Title}} · {{.SiteTitle}}</title>
<link rel="stylesheet" href="{{.Base}}style.css" />
</head>
<body>
<header class="topbar">
  <a class="brand" href="{{.Base}}index.html">
    <span class="mark">O</span><span>{{.SiteTitle}}</span>
  </a>
  <a class="home-link" href="{{.Base}}index.html">All notes</a>
</header>
<div class="layout">
  <nav class="sidebar">
    {{range .Nav}}<a class="nav-item" href="{{.Href}}">{{.Title}}</a>
    {{end}}
  </nav>
  <main class="content">
    <div class="crumbs">Notes / {{.Title}}</div>
    <article class="doc">{{.Content}}</article>
  </main>
  {{if .Headings}}
  <aside class="toc">
    <h4>On this page</h4>
    {{range .Headings}}<a class="toc-item lvl-{{.Level}}" href="#{{.ID}}">{{.Text}}</a>
    {{end}}
  </aside>
  {{end}}
</div>
</body>
</html>
`))

var indexTmpl = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>{{.SiteTitle}}</title>
<link rel="stylesheet" href="{{.Base}}style.css" />
</head>
<body>
<header class="topbar">
  <a class="brand" href="{{.Base}}index.html">
    <span class="mark">O</span><span>{{.SiteTitle}}</span>
  </a>
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
</body>
</html>
`))

const siteCSS = `:root{
  --bg:#fff;--panel:#fafafa;--soft:#f5f6f7;--hover:#eef0f2;--border:#e7e9ec;--border-strong:#d6d9dd;
  --text:#1a1d21;--muted:#5b6470;--soft-text:#8a93a0;--accent:#2f6fed;--accent-soft:#eaf0fe;
  --code:#f4f6f8;--mono:ui-monospace,"SFMono-Regular",Consolas,Menlo,monospace;
  --sans:-apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",Roboto,Helvetica,Arial,sans-serif;
}
@media (prefers-color-scheme:dark){:root{
  --bg:#16181c;--panel:#1b1e23;--soft:#212429;--hover:#282c32;--border:#2b2f36;--border-strong:#3a3f48;
  --text:#e8eaed;--muted:#a4acb8;--soft-text:#838b97;--accent:#6a9bf5;--accent-soft:#22304a;--code:#1d2026;
}}
*{box-sizing:border-box}
html,body{margin:0}
body{font-family:var(--sans);color:var(--text);background:var(--bg);font-size:16px;line-height:1.7;-webkit-font-smoothing:antialiased}
a{color:var(--accent);text-decoration:none}a:hover{text-decoration:underline}
.topbar{display:flex;align-items:center;gap:16px;height:52px;padding:0 20px;border-bottom:1px solid var(--border);background:var(--bg);position:sticky;top:0;z-index:10}
.brand{display:flex;align-items:center;gap:10px;color:var(--text);font-weight:750;font-size:18px;letter-spacing:-.02em}
.mark{width:30px;height:30px;border-radius:9px;background:var(--accent);color:#fff;font-weight:800;font-size:18px;display:inline-flex;align-items:center;justify-content:center}
.home-link{margin-left:auto;font-size:14px;color:var(--muted)}
.layout{display:flex;align-items:flex-start;max-width:1240px;margin:0 auto}
.sidebar{position:sticky;top:52px;width:240px;flex-shrink:0;padding:24px 12px;max-height:calc(100vh - 52px);overflow:auto}
.nav-item{display:block;padding:5px 10px;border-left:2px solid transparent;border-radius:0 6px 6px 0;color:var(--muted);font-size:14.5px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.nav-item:hover{background:var(--hover);color:var(--text);text-decoration:none}
.content{flex:1;min-width:0;padding:40px 48px 120px}
.crumbs{font-family:var(--mono);font-size:12px;color:var(--soft-text);margin-bottom:16px}
.home-title{font-size:2.1em;margin:0 0 20px;letter-spacing:-.02em}
.note-list{list-style:none;padding:0;display:flex;flex-direction:column;gap:6px}
.note-list a{font-size:16px}
.doc{max-width:760px}
.doc h1{font-size:2.05em;letter-spacing:-.02em;margin:.2em 0 .5em}
.doc h2{font-size:1.4em;border-bottom:1px solid var(--border);padding-bottom:.28em;margin-top:1.7em}
.doc h3{font-size:1.16em;margin-top:1.5em}
.doc code{font-family:var(--mono);font-size:.87em;background:var(--code);border:1px solid var(--border);padding:.12em .36em;border-radius:5px}
.doc pre{background:var(--code);border:1px solid var(--border);border-radius:9px;padding:14px 16px;overflow:auto}
.doc pre code{background:transparent;border:none;padding:0}
.doc blockquote{border-left:3px solid var(--border-strong);padding-left:14px;color:var(--muted);margin-left:0}
.doc table{border-collapse:collapse;width:100%;margin:1em 0}
.doc th,.doc td{border:1px solid var(--border);padding:8px 10px;text-align:left}
.doc th{background:var(--soft)}
.doc img{max-width:100%;border-radius:9px;border:1px solid var(--border)}
.doc a{color:var(--accent)}
.toc{position:sticky;top:52px;width:220px;flex-shrink:0;padding:40px 16px;max-height:calc(100vh - 52px);overflow:auto}
.toc h4{margin:0 0 8px;font-size:12px;text-transform:uppercase;letter-spacing:.06em;color:var(--soft-text);font-weight:650}
.toc-item{display:block;padding:3px 10px;border-left:2px solid transparent;color:var(--muted);font-size:13.5px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.toc-item:hover{color:var(--text);text-decoration:none}
.toc-item.lvl-3{padding-left:22px}
.toc-item.lvl-4{padding-left:34px}
@media(max-width:1100px){.toc{display:none}}
@media(max-width:860px){.sidebar{display:none}.content{padding:28px 20px 100px}}
`
