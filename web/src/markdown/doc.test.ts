// @vitest-environment jsdom
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

  it("keeps wiki-links as anchors", () => {
    const { html } = renderPublicDoc("See [[Target|Alias]] here");
    expect(html).toContain('class="wiki-link"');
    expect(html).toContain('data-wiki="Target"');
    expect(html).toContain("Alias");
    expect(html).not.toContain("[[");
  });

  it("renders inline math without throwing", () => {
    const { html } = renderPublicDoc("Value $x^2$ end");
    expect(html).toContain("math-inline");
  });

  it("adds the code-block class and trims the fenced newline", () => {
    const { html } = renderPublicDoc("```go\nfunc main() {}\n```\n");
    expect(html).toContain('class="code-block"');
    expect(html).toContain('class="language-go"');
    expect(html).not.toContain("\n</code></pre>");
  });

  it("wraps table cells in paragraphs", () => {
    const { html } = renderPublicDoc("| a | b |\n| - | - |\n| 1 | 2 |\n");
    expect(html).toContain("<th><p>a</p></th>");
    expect(html).toContain("<td><p>1</p></td>");
  });

  it("normalizes task lists and keeps checkboxes read-only", () => {
    const { html } = renderPublicDoc("- [x] done\n- [ ] todo\n");
    expect(html).toContain('data-type="taskList"');
    expect(html).toContain('data-type="taskItem"');
    expect(html).toContain('data-checked="true"');
    expect(html).toContain('data-checked="false"');
    expect(html).toContain('disabled=""');
  });
});
