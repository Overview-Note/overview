package server

import (
	"net/http"

	"github.com/Overview-Note/overview/internal/openapi"
)

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	spec, err := openapi.JSON()
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(spec)
}

const docsHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>Overview API</title>
<style>
  :root { color-scheme: light dark; }
  body { font-family: system-ui, sans-serif; max-width: 880px; margin: 0 auto; padding: 32px 20px; background: #fcfbf9; color: #2b2a27; }
  .brand { display: inline-flex; align-items: center; gap: 10px; }
  .mark { width: 30px; height: 30px; border-radius: 9px; background: linear-gradient(145deg,#8ea1ea,#5f79d4); color: #fff; display: inline-flex; align-items: center; justify-content: center; }
  .mark svg { width: 62%; height: 62%; display: block; }
  h1 { margin-bottom: 4px; }
  .tag { font-size: 12px; text-transform: uppercase; letter-spacing: .06em; color: #8c857b; margin-top: 28px; }
  .op { border: 1px solid #e8e4dd; border-radius: 8px; padding: 10px 12px; margin: 8px 0; background: #fff; }
  .method { font-weight: 700; font-family: ui-monospace, monospace; margin-right: 8px; }
  .get { color: #4d66c2; } .post { color: #5aa078; } .put { color: #a8823f; } .delete { color: #c96a6a; }
  .path { font-family: ui-monospace, monospace; }
  .summary { color: #67635c; font-size: 13px; }
  @media (prefers-color-scheme: dark) {
    body { background: #1b1a18; color: #e9e6e0; }
    .op { border-color: #33302b; background: #222120; } .summary { color: #8c857b; }
    .get { color: #93a8ef; } .post { color: #79bd93; } .put { color: #e0c48a; } .delete { color: #e28b8b; }
  }
</style>
</head>
<body>
  <div class="brand"><span class="mark"><svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M9 4H6.5A2.5 2.5 0 0 0 4 6.5V9M15 4h2.5A2.5 2.5 0 0 1 20 6.5V9M20 15v2.5a2.5 2.5 0 0 1-2.5 2.5H15M9 20H6.5A2.5 2.5 0 0 1 4 17.5V15" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/><circle cx="12" cy="12" r="2.15" fill="currentColor"/></svg></span></div>
  <h1>Overview API</h1>
  <p class="summary">OpenAPI spec: <a href="/api/v1/openapi.json">/api/v1/openapi.json</a></p>
  <div id="app"></div>
  <script>
    fetch('/api/v1/openapi.json').then(r => r.json()).then(spec => {
      const app = document.getElementById('app');
      const byTag = {};
      for (const [path, methods] of Object.entries(spec.paths)) {
        for (const [method, op] of Object.entries(methods)) {
          const tag = (op.tags && op.tags[0]) || 'other';
          (byTag[tag] = byTag[tag] || []).push({ path, method, op });
        }
      }
      for (const [tag, ops] of Object.entries(byTag)) {
        const h = document.createElement('div');
        h.className = 'tag';
        h.textContent = tag;
        app.appendChild(h);
        for (const { path, method, op } of ops) {
          const div = document.createElement('div');
          div.className = 'op';
          div.innerHTML = '<span class="method ' + method + '">' + method.toUpperCase() + '</span>' +
            '<span class="path"></span>' +
            '<div class="summary"></div>';
          div.querySelector('.path').textContent = '/api/v1' + path;
          div.querySelector('.summary').textContent = op.summary || '';
          app.appendChild(div);
        }
      }
    });
  </script>
</body>
</html>`

func (s *Server) handleAPIDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(docsHTML))
}
