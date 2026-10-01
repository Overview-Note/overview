// Overview service worker: caches the app shell for offline launch.
// API responses and notes are never cached (they must stay fresh and private).
const CACHE = "overview-shell-v1";
const SHELL = ["/", "/index.html", "/manifest.webmanifest", "/icon.svg"];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE).then((cache) => cache.addAll(SHELL)).then(() => self.skipWaiting()),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (event) => {
  const { request } = event;
  const url = new URL(request.url);

  // Only handle same-origin GETs.
  if (request.method !== "GET" || url.origin !== self.location.origin) return;
  // Never cache API, WebDAV or MCP responses.
  if (
    url.pathname.startsWith("/api/") ||
    url.pathname.startsWith("/dav") ||
    url.pathname.startsWith("/mcp")
  ) {
    return;
  }

  // Network-first for navigations, falling back to the cached shell.
  if (request.mode === "navigate") {
    event.respondWith(
      fetch(request).catch(() => caches.match("/index.html").then((r) => r || Response.error())),
    );
    return;
  }

  // Cache-first only for hashed build artifacts (index-<hash>.js/css); user
  // uploads share the /assets/ prefix but must never be cached.
  const isBuild = /\/assets\/index-[A-Za-z0-9_-]+\.(js|css)$/.test(url.pathname);
  if (!isBuild && url.pathname !== "/icon.svg") return;

  event.respondWith(
    caches.match(request).then((cached) => {
      if (cached) return cached;
      return fetch(request).then((response) => {
        if (response.ok) {
          const copy = response.clone();
          caches.open(CACHE).then((cache) => cache.put(request, copy));
        }
        return response;
      });
    }),
  );
});
