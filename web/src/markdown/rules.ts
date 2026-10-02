// Turndown rules that preserve custom editor nodes back into Markdown.

import type TurndownService from "turndown";

export function registerMathRules(turndown: TurndownService): void {
  turndown.addRule("inlineMath", {
    filter: (node) =>
      node.nodeName === "SPAN" && (node as HTMLElement).classList.contains("math-inline"),
    replacement: (_content, node) => {
      const tex = (node as HTMLElement).getAttribute("data-tex") ?? "";
      return `$${tex}$`;
    },
  });
  turndown.addRule("blockMath", {
    filter: (node) =>
      node.nodeName === "DIV" && (node as HTMLElement).classList.contains("math-block"),
    replacement: (_content, node) => {
      const tex = (node as HTMLElement).getAttribute("data-tex") ?? "";
      return `\n\n$$\n${tex}\n$$\n\n`;
    },
  });
}

export function registerFootnoteRules(turndown: TurndownService): void {
  turndown.addRule("footnoteRef", {
    filter: (node) =>
      node.nodeName === "SUP" && (node as HTMLElement).classList.contains("fn-ref"),
    replacement: (_content, node) => {
      const id = (node as HTMLElement).getAttribute("data-id") ?? "";
      return `[^${id}]`;
    },
  });
  turndown.addRule("footnoteDefs", {
    filter: (node) =>
      node.nodeName === "DIV" && (node as HTMLElement).classList.contains("fn-defs"),
    replacement: (_content, node) => {
      const raw = (node as HTMLElement).getAttribute("data-items") ?? "[]";
      let items: Array<{ id: string; text: string }> = [];
      try {
        items = JSON.parse(raw);
      } catch {
        items = [];
      }
      const lines = items.map((it) => `[^${it.id}]: ${it.text}`).join("\n");
      return `\n\n${lines}\n\n`;
    },
  });
}

export function registerMermaidRules(turndown: TurndownService): void {
  turndown.addRule("mermaidBlock", {
    filter: (node) =>
      node.nodeName === "DIV" && (node as HTMLElement).classList.contains("mermaid"),
    replacement: (_content, node) => {
      const source =
        (node as HTMLElement).getAttribute("data-source") ??
        (node as HTMLElement).textContent ??
        "";
      return `\n\n\`\`\`mermaid\n${source.trim()}\n\`\`\`\n\n`;
    },
  });
}
