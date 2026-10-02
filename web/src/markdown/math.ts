// KaTeX integration for the Markdown pipeline.
//
// Markdown keeps the standard `$inline$` / `$$block$$` syntax so it stays
// portable; rendering turns math into HTML that Tiptap can hold. Math is
// processed *after* Markdown parsing so `$` inside code blocks is untouched,
// and the same code drives both the editor and the public read-only pages.

import katex from "katex";
import "katex/dist/katex.min.css";

export function renderMath(tex: string, displayMode: boolean): string {
  try {
    return katex.renderToString(tex, {
      displayMode,
      throwOnError: false,
      output: "html",
    });
  } catch {
    return `<code class="math-error">${escapeHtml(tex)}</code>`;
  }
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function escapeAttr(s: string): string {
  return escapeHtml(s).replace(/"/g, "&quot;");
}

/**
 * Replaces `$$...$$` and `$...$` spans in rendered HTML with KaTeX markup.
 * Only text outside of `<code>`/`<pre>` blocks is considered.
 */
export function katexToHtml(html: string): string {
  const segments = html.split(/(<pre[\s\S]*?<\/pre>|<code[\s\S]*?<\/code>)/g);
  return segments
    .map((seg) => {
      if (seg.startsWith("<pre") || seg.startsWith("<code")) return seg;
      let out = seg.replace(/\$\$([\s\S]+?)\$\$/g, (_m, tex: string) => {
        const t = tex.trim();
        return `<div class="math-block" data-tex="${escapeAttr(t)}">${renderMath(
          t,
          true,
        )}</div>`;
      });
      out = out.replace(/(^|[^\\$])\$([^\n$]+?)\$/g, (_m, pre: string, tex: string) => {
        const t = tex.trim();
        return `${pre}<span class="math-inline" data-tex="${escapeAttr(
          t,
        )}">${renderMath(t, false)}</span>`;
      });
      return out;
    })
    .join("");
}
