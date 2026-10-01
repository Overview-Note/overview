// Attachment references are stored vault-relative (e.g. "assets/2026/10/x.png")
// so a data/ directory can be opened directly by other Markdown tools. For
// rendering in the SPA we resolve them to the served "/assets/" path.

const PREFIXES = ["/api/v1/assets/", "/api/assets/", "/assets/"];

/** Rewrites image/link sources found in generated HTML to served asset URLs. */
export function resolveAssetSrc(html: string): string {
  let out = html;
  for (const prefix of PREFIXES) {
    out = out.split(`src="${prefix}`).join('src="/assets/');
  }
  out = out.split('src="assets/').join('src="/assets/');
  return out;
}

/** Rewrites served asset URLs back to vault-relative paths before saving. */
export function toVaultMarkdown(markdown: string): string {
  let out = markdown;
  for (const prefix of ["/api/v1/assets/", "/api/assets/", "/assets/"]) {
    out = out.split(`](${prefix}`).join("](assets/");
  }
  return out;
}
