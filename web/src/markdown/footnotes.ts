// GFM footnotes.
//
// Footnotes are represented as atom nodes (reference + definition) so the
// editor round-trips `[^id]` / `[^id]: text` losslessly. Definitions are moved
// to the document end during pre-processing, matching GFM behaviour.

const REF_RE = /\[\^([^\]]+)\]/g;
const DEF_RE = /^\[\^([^\]]+)\]:\s?([\s\S]*?)(?=\n\[\^|\n\n|\s*$)/gm;

function escapeAttr(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;");
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

/**
 * Splits footnote definitions out of the source and renders references and a
 * definitions block as custom HTML nodes. Returns HTML ready for marked.
 */
export function footnotesToHtml(markdown: string): string {
  const defs: Array<{ id: string; text: string }> = [];
  const body = markdown.replace(DEF_RE, (_m, id: string, text: string) => {
    defs.push({ id, text: text.replace(/\s+$/g, "") });
    return "";
  });
  if (defs.length === 0) return body;
  const withRefs = body.replace(REF_RE, (_m, id: string) => {
    if (!defs.some((d) => d.id === id)) return _m;
    return `<sup class="fn-ref" data-id="${escapeAttr(id)}">[${escapeHtml(id)}]</sup>`;
  });
  const block = defs
    .map(
      (d) =>
        `<div class="fn-def" data-id="${escapeAttr(d.id)}">${escapeHtml(d.text)}</div>`,
    )
    .join("\n");
  return `${withRefs}\n\n<div class="fn-defs">${block}</div>`;
}
