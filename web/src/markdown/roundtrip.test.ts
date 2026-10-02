// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { Editor } from "@tiptap/core";

import { baseExtensions } from "../editor/extensions";
import { createTurndown, htmlToMarkdown, mdToHtml } from "./pipeline";

// Full pipeline: Markdown -> editor HTML -> Tiptap -> HTML -> Markdown.
function roundTrip(markdown: string): string {
  const editor = new Editor({
    extensions: baseExtensions(),
    content: mdToHtml(markdown),
  });
  const html = editor.getHTML();
  editor.destroy();
  return htmlToMarkdown(createTurndown(), html);
}

describe("editor markdown round-trip", () => {
  it("preserves task lists and their checked state", () => {
    const out = roundTrip("- [x] done\n- [ ] todo\n");
    expect(out).toContain("- [x] done");
    expect(out).toContain("- [ ] todo");
    expect(out).not.toContain("&amp;");
  });

  it("preserves tables", () => {
    const out = roundTrip("| a | b |\n| --- | --- |\n| 1 | 2 |\n");
    expect(out).toMatch(/\|\s*a\s*\|\s*b\s*\|/);
    expect(out).toContain("| 1 | 2 |");
    expect(out).not.toContain("<table");
  });

  it("preserves wiki links", () => {
    const out = roundTrip("See [[Target|Alias]] here\n");
    expect(out).toContain("[[Target|Alias]]");
    expect(out).not.toContain("[Alias](#)");
  });

  it("preserves inline and block math exactly", () => {
    const out = roundTrip(
      "Inline $E = mc^2$ end.\n\n$$\n\\int_0^1 x\\,dx = \\frac12\n$$\n",
    );
    expect(out).toContain("$E = mc^2$");
    expect(out).toContain("\\int_0^1 x\\,dx = \\frac12");
    expect(out).not.toContain("&amp;");
  });

  it("preserves footnote references and definitions", () => {
    const out = roundTrip("Text with a note[^1] here.\n\n[^1]: the definition\n");
    expect(out).toContain("[^1]");
    expect(out).toContain("[^1]: the definition");
  });

  it("preserves mermaid code blocks", () => {
    const out = roundTrip("```mermaid\ngraph TD\n  A --> B\n```\n");
    expect(out).toContain("```mermaid");
    expect(out).toContain("A --> B");
  });

  it("preserves code blocks with language and task nesting", () => {
    const out = roundTrip("```go\nfunc main() {}\n```\n\n- [ ] outer\n  - [x] inner\n");
    expect(out).toContain("```go");
    expect(out).toContain("func main() {}");
    expect(out).toContain("- [ ] outer");
    expect(out).toContain("- [x] inner");
  });
});
