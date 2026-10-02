import { describe, expect, it } from "vitest";
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
