// Read-only document rendering for the public share pages.
//
// Produces the same shape as the generated documentation site: sanitized HTML
// with heading anchors plus a heading list for the "On this page" outline.

import { marked } from "marked";
import { resolveAssetSrc } from "./assets";
import { mermaidToDivs } from "./diagrams";
import { footnotesToHtml } from "./footnotes";
import { katexToHtml } from "./math";

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

/** Renders public Markdown to HTML and extracts heading anchors. */
export function renderPublicDoc(markdown: string): RenderedDoc {
  // Wiki-links degrade to plain text for anonymous readers (no target leak).
  const md = markdown.replace(/\[\[([^[\]]+?)\]\]/g, (_m, inner: string) => {
    const [target, display] = inner.split("|");
    return (display ?? target).trim() || target;
  });
  const parsed = marked.parse(footnotesToHtml(md), { async: false }) as string;
  const html = mermaidToDivs(katexToHtml(resolveAssetSrc(parsed)));

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
