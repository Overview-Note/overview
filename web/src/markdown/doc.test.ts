import { describe, expect, it } from "vitest";
import { renderPublicDoc } from "./doc";

describe("renderPublicDoc", () => {
  it("adds heading anchors and extracts a TOC excluding the h1", () => {
    const { html, headings } = renderPublicDoc(
      "# Title\n\n## Section One\n\ntext\n\n### Deep\n\nmore",
    );
    expect(html).toContain('id="section-one"');
    expect(html).toContain('id="deep"');
    expect(headings.map((h) => h.id)).toEqual(["section-one", "deep"]);
    expect(headings[0].level).toBe(2);
  });

  it("degrades wiki-links to plain text", () => {
    const { html } = renderPublicDoc("See [[Target|Alias]] here");
    expect(html).toContain("Alias");
    expect(html).not.toContain("[[");
  });

  it("renders inline math without throwing", () => {
    const { html } = renderPublicDoc("Value $x^2$ end");
    expect(html).toContain("math-inline");
  });
});
