// Normalizes the HTML produced by the Tiptap editor before converting it back
// to Markdown.
//
// Tiptap's resizable tables add <colgroup>/<col> and wrap cell content in <p>.
// turndown-plugin-gfm decides a table "has a heading row" by checking whether
// the first <tbody> is the first child of the table — the <colgroup> breaks that
// check, so the table is kept as raw HTML instead of a Markdown table. Removing
// the layout-only markup makes the round-trip lossless again.
export function sanitizeEditorHtml(html: string): string {
  return html
    .replace(/<colgroup[\s\S]*?<\/colgroup>/gi, "")
    .replace(/<(col|colgroup)\b[^>]*>/gi, "")
    .replace(/\s(colspan|rowspan)="1"/gi, "")
    .replace(/\sstyle="[^"]*"/gi, "")
    .replace(/<t([hd])([^>]*)><p>([\s\S]*?)<\/p><\/t\1>/gi, "<t$1$2>$3</t$1>");
}
