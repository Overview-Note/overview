// The editor's Markdown <-> HTML pipeline, shared by the editor component and
// the round-trip tests.
import { marked } from "marked";
import TurndownService from "turndown";
import { gfm } from "turndown-plugin-gfm";

import { resolveAssetSrc, toVaultMarkdown } from "./assets";
import { mermaidToDivs } from "./diagrams";
import { footnotesToHtml } from "./footnotes";
import { sanitizeEditorHtml } from "./html";
import { renderMathInMarkdown } from "./math";
import {
  registerFootnoteRules,
  registerMathRules,
  registerMermaidRules,
  registerTaskRules,
} from "./rules";
import { normalizeTaskLists } from "./tasks";
import { registerWikiRule, wikiToHtml } from "./wiki";

marked.setOptions({ gfm: true, breaks: false });

/** Builds a Turndown service configured to emit Overview-flavoured Markdown. */
export function createTurndown(): TurndownService {
  const turndown = new TurndownService({
    headingStyle: "atx",
    codeBlockStyle: "fenced",
    bulletListMarker: "-",
    emDelimiter: "*",
    hr: "---",
  });
  turndown.use(gfm);
  registerWikiRule(turndown);
  registerMathRules(turndown);
  registerMermaidRules(turndown);
  registerFootnoteRules(turndown);
  registerTaskRules(turndown);
  return turndown;
}

/** Converts vault Markdown to the HTML the editor loads. */
export function mdToHtml(markdown: string): string {
  const prepared = renderMathInMarkdown(wikiToHtml(footnotesToHtml(markdown)));
  const html = marked.parse(prepared, { async: false }) as string;
  return normalizeTaskLists(
    mermaidToDivs(resolveAssetSrc(trimFencedCodeNewline(html))),
  );
}

// marked terminates the last line of every fenced code block with a newline.
// Because code is rendered with `white-space: pre`, that newline shows up as an
// extra blank line inside the block, so drop exactly one trailing newline per
// <pre><code> while preserving any intentional blank lines in the source.
function trimFencedCodeNewline(html: string): string {
  return html.replace(
    /(<pre><code\b[^>]*>)([\s\S]*?)\r?\n(<\/code><\/pre>)/gi,
    "$1$2$3",
  );
}

/** Converts the editor's HTML back to vault Markdown. */
export function htmlToMarkdown(turndown: TurndownService, html: string): string {
  return toVaultMarkdown(turndown.turndown(sanitizeEditorHtml(html)));
}
