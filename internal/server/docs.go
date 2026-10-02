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
  body { font-family: system-ui, sans-serif; max-width: 880px; margin: 0 auto; padding: 32px 20px; }
  h1 { margin-bottom: 4px; }
  .tag { font-size: 12px; text-transform: uppercase; letter-spacing: .06em; color: #6b7280; margin-top: 28px; }
  .op { border: 1px solid #e5e7eb; border-radius: 8px; padding: 10px 12px; margin: 8px 0; }
  .method { font-weight: 700; font-family: ui-monospace, monospace; margin-right: 8px; }
  .get { color: #2563eb; } .post { color: #16a34a; } .put { color: #d97706; } .delete { color: #dc2626; }
  .path { font-family: ui-monospace, monospace; }
  .summary { color: #6b7280; font-size: 13px; }
  @media (prefers-color-scheme: dark) {
    .op { border-color: #33363c; } .summary { color: #9aa0a6; }
  }
</style>
</head>
<body>
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
