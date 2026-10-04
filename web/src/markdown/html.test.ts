import { describe, expect, it } from "vitest";
import { decorateImages, resolveAssetSrc, toVaultMarkdown } from "./assets";
import { sanitizeEditorHtml } from "./html";

describe("sanitizeEditorHtml", () => {
  it("strips layout-only table markup so tables round-trip to Markdown", () => {
    const html =
      '<table class="md-table" style="min-width: 75px;">' +
      '<colgroup><col style="min-width: 25px;"></colgroup>' +
      '<tbody><tr><th colspan="1" rowspan="1"><p>A</p></th></tr></tbody></table>';
    const out = sanitizeEditorHtml(html);
    expect(out).not.toContain("colgroup");
    expect(out).not.toContain("style=");
    expect(out).not.toContain('colspan="1"');
    expect(out).not.toContain("<p>");
    expect(out).toContain("A</th>");
  });

  it("leaves inline cell formatting intact", () => {
    const html = "<td><p><code>x = 1</code></p></td>";
    expect(sanitizeEditorHtml(html)).toBe("<td><code>x = 1</code></td>");
  });
});

describe("resolveAssetSrc / decorateImages", () => {
  it("resolves vault-relative asset paths and marks images lazy/async", () => {
    const out = resolveAssetSrc('<p><img src="assets/2026/10/pic.png" alt="p"></p>');
    expect(out).toContain('src="/assets/2026/10/pic.png"');
    expect(out).toContain('loading="lazy"');
    expect(out).toContain('decoding="async"');
  });

  it("does not double-decorate or touch <image> nodes", () => {
    const once = decorateImages('<img src="x.png" loading="lazy" decoding="async">');
    expect(once.match(/loading=/g)?.length).toBe(1);
    expect(decorateImages("<image href='x'>")).toBe("<image href='x'>");
  });

  it("rewrites vault-relative attachment links to served asset URLs", () => {
    const out = resolveAssetSrc('<a href="assets/2026/10/x.pdf">x</a>');
    expect(out).toContain('href="/assets/2026/10/x.pdf"');
    expect(out).not.toContain('href="assets/');
  });

  it("rewrites served asset link prefixes and leaves other links alone", () => {
    const out = resolveAssetSrc(
      '<a href="/api/v1/assets/2026/10/x.pdf">a</a>' +
        '<a href="https://example.com/x">b</a>',
    );
    expect(out).toContain('href="/assets/2026/10/x.pdf"');
    expect(out).toContain('href="https://example.com/x"');
  });
});

describe("toVaultMarkdown", () => {
  it("rewrites served asset links back to vault-relative paths before saving", () => {
    const out = toVaultMarkdown(
      "see [x](/assets/2026/10/x.pdf) and [y](/api/v1/assets/old.pdf)",
    );
    expect(out).toContain("](assets/2026/10/x.pdf)");
    expect(out).toContain("](assets/old.pdf)");
    expect(out).not.toContain("](/assets/");
    expect(out).not.toContain("](/api/");
  });
});
