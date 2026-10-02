package server

import (
	"net/http"
	"strings"
)

// baseURL reconstructs the absolute origin of the current request, honouring a
// reverse proxy's X-Forwarded-Proto/Host headers.
func baseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = fwd
	}
	return scheme + "://" + host
}

// handleRobots serves a permissive robots.txt that points crawlers at the
// sitemap. In private mode it disallows everything.
func (s *Server) handleRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	var b strings.Builder
	if s.opts.Render {
		b.WriteString("User-agent: *\nAllow: /\n\n")
	} else {
		b.WriteString("User-agent: *\nDisallow: /\n\n")
	}
	b.WriteString("Sitemap: " + baseURL(r) + "/sitemap.xml\n")
	_, _ = w.Write([]byte(b.String()))
}

// handleSitemap lists public notes as sitemap.xml when running as a
// documentation/render site. Outside render mode it returns an empty sitemap so
// private instances do not leak note paths.
func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	base := baseURL(r)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	if s.opts.Render {
		b.WriteString("  <url><loc>" + xmlEscape(base+"/public") + "</loc></url>\n")
		notes, err := s.svc.PublicNotes(r.Context())
		if err == nil {
			for _, n := range notes {
				loc := base + "/public/" + escapePath(n.Path)
				b.WriteString("  <url><loc>" + xmlEscape(loc) + "</loc>")
				if !n.Updated.IsZero() {
					b.WriteString("<lastmod>" + n.Updated.UTC().Format("2006-01-02") + "</lastmod>")
				}
				b.WriteString("</url>\n")
			}
		}
	}
	b.WriteString("</urlset>\n")
	_, _ = w.Write([]byte(b.String()))
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, seg := range parts {
		parts[i] = pathEscape(seg)
	}
	return strings.Join(parts, "/")
}

func pathEscape(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0f])
		}
	}
	return b.String()
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}
