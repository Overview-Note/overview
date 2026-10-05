// Read-only document rendering for the public share pages.
//
// Produces the same shape as the generated documentation site: sanitized HTML
// with heading anchors plus a heading list for the "On this page" outline.

import { marked } from "marked";
import { resolveAssetSrc } from "./assets";
import { mermaidToDivs } from "./diagrams";
import { footnotesToHtml } from "./footnotes";
import { renderMathInMarkdown } from "./math";
import { trimFencedCodeNewline } from "./pipeline";
import { normalizeTaskLists } from "./tasks";
import { wikiToHtml } from "./wiki";

export interface DocHeading {
  level: number;
  id: string;
  text: string;
}

export interface RenderedDoc {
  html: string;
  headings: DocHeading[];
}

const HEADING_RE = /<h([1-4])>([\s\S]*?)<\/h\1>/g;

function stripTags(html: string): string {
  return html
    .replace(/<[^>]*>/g, "")
    .replace(/\s+/g, " ")
    .trim();
}

function slugify(text: string): string {
  const slug = text
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\s-]/gu, "")
    .replace(/\s+/g, "-");
  return slug || "";
}

function addCodeBlockClass(html: string): string {
  return html.replace(/<pre>/g, '<pre class="code-block">');
}

function postProcessDocument(html: string): string {
  const hasTable = /<t[hd][\s>]/.test(html);
  const hasTasks = html.includes('data-type="taskList"');
  if (!hasTable && !hasTasks) return html;
  const doc = new DOMParser().parseFromString(html, "text/html");
  if (hasTable) {
    for (const cell of Array.from(doc.querySelectorAll("th, td"))) {
      const first = cell.firstChild;
      if (
        first &&
        first === cell.lastChild &&
        first.nodeType === 1 &&
        (first as Element).tagName === "P"
      ) {
        continue;
      }
      const p = doc.createElement("p");
      while (cell.firstChild) p.appendChild(cell.firstChild);
      cell.appendChild(p);
    }
  }
  if (hasTasks) {
    for (const box of Array.from(
      doc.querySelectorAll('ul[data-type="taskList"] input[type="checkbox"]'),
    )) {
      (box as HTMLInputElement).disabled = true;
    }
  }
  return doc.body.innerHTML;
}

/** Renders public Markdown to HTML and extracts heading anchors. */
export function renderPublicDoc(markdown: string): RenderedDoc {
  const prepared = renderMathInMarkdown(footnotesToHtml(wikiToHtml(markdown)));
  const parsed = marked.parse(prepared, { async: false }) as string;
  const html = postProcessDocument(
    addCodeBlockClass(
      normalizeTaskLists(mermaidToDivs(resolveAssetSrc(trimFencedCodeNewline(parsed)))),
    ),
  );

  const headings: DocHeading[] = [];
  const counts = new Map<string, number>();
  const out = html.replace(HEADING_RE, (_match, level: string, inner: string) => {
    const text = stripTags(inner);
    let id = slugify(text);
    if (!id) id = "section";
    const n = (counts.get(id) ?? 0) + 1;
    counts.set(id, n);
    if (n > 1) id = `${id}-${n}`;
    // The page title (h1) is excluded from the "On this page" outline.
    if (Number(level) >= 2) headings.push({ level: Number(level), id, text });
    return `<h${level} id="${id}">${inner}</h${level}>`;
  });

  return { html: out, headings };
}
