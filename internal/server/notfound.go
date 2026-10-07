package server

import "net/http"

// notFoundPage is a self-contained HTML 404 page served when the embedded
// frontend is unavailable (not built, or index.html missing/damaged). Its CSS
// is inlined and it references no external resource, so it renders even when
// the asset bundle cannot be loaded. Light/dark follow the OS preference.
const notFoundPage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>404 — Overview</title>
<style>
:root {
  color-scheme: light dark;
  --bg: #ffffff;
  --panel: #fafaf8;
  --border: #e8e8e6;
  --text: #37352f;
  --strong: #1a1a1a;
  --muted: #6b6b6b;
  --accent: #d4870e;
  --btn-bg: #1a1a1a;
  --btn-fg: #ffffff;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #1f1e1c;
    --panel: #262523;
    --border: #33312d;
    --text: #e9e6e0;
    --strong: #f7f5f0;
    --muted: #b3afa8;
    --accent: #e9a23b;
    --btn-bg: #e9e6e0;
    --btn-fg: #1f1e1c;
  }
}
* { box-sizing: border-box; }
html, body { height: 100%; margin: 0; }
body {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC",
    "Hiragino Sans GB", "Microsoft YaHei", Roboto, Helvetica, Arial, sans-serif;
  color: var(--text);
  background: var(--bg);
  -webkit-font-smoothing: antialiased;
}
.card {
  width: min(440px, 100%);
  padding: 40px 32px;
  text-align: center;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 12px;
  box-shadow: 0 8px 24px rgba(15, 15, 15, 0.1);
}
.brand {
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--muted);
}
.code {
  margin: 16px 0 4px;
  font-size: 72px;
  font-weight: 700;
  line-height: 1;
  letter-spacing: -0.03em;
  color: var(--accent);
}
h1 {
  margin: 0 0 8px;
  font-size: 20px;
  letter-spacing: -0.02em;
  color: var(--strong);
}
p {
  margin: 0 0 24px;
  font-size: 15px;
  line-height: 1.6;
  color: var(--muted);
}
a {
  display: inline-block;
  padding: 10px 18px;
  font-size: 15px;
  font-weight: 600;
  text-decoration: none;
  color: var(--btn-fg);
  background: var(--btn-bg);
  border-radius: 8px;
}
</style>
</head>
<body>
  <main class="card">
    <div class="brand">Overview</div>
    <div class="code">404</div>
    <h1>Page not found</h1>
    <p>The page you are looking for does not exist or has been moved.</p>
    <a href="/">Back to home</a>
  </main>
</body>
</html>
`

// writeNotFoundPage writes the self-contained HTML 404 page with a 404 status
// and an HTML content type. It is the fallback for the document routes when the
// SPA cannot be served; JSON is still used for /api/ and non-HTML 404s for
// missing assets.
func writeNotFoundPage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(notFoundPage))
}
