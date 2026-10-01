import type TurndownService from "turndown";

const WIKI = /\[\[([^[\]]+?)\]\]/g;

function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (c) => {
    switch (c) {
      case "&":
        return "&amp;";
      case "<":
        return "&lt;";
      case ">":
        return "&gt;";
      case '"':
        return "&quot;";
      default:
        return "&#39;";
    }
  });
}

/**
 * Converts [[Target]] / [[Target|Display]] wiki-links into anchors before the
 * markdown is parsed, so Tiptap can display them as links.
 */
export function wikiToHtml(markdown: string): string {
  return markdown.replace(WIKI, (_match, inner: string) => {
    const [rawTarget, rawDisplay] = inner.split("|");
    const target = rawTarget.trim();
    if (!target) return _match;
    const display = (rawDisplay ?? target).trim() || target;
    return `<a href="#" data-wiki="${escapeHtml(target)}" class="wiki-link">${escapeHtml(
      display,
    )}</a>`;
  });
}

/** Registers a Turndown rule that writes wiki-links back to [[...]] syntax. */
export function registerWikiRule(turndown: TurndownService): void {
  turndown.addRule("wikiLink", {
    filter: (node) =>
      node.nodeName === "A" && (node as HTMLElement).hasAttribute("data-wiki"),
    replacement: (content, node) => {
      const target = (node as HTMLElement).getAttribute("data-wiki") ?? "";
      const display = content.trim();
      if (!display || display === target) return `[[${target}]]`;
      return `[[${target}|${display}]]`;
    },
  });
}
