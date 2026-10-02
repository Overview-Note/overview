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
      const defs = Array.from(
        (node as HTMLElement).querySelectorAll<HTMLElement>(".fn-def"),
      );
      const lines = defs.map(
        (d) => `[^${d.getAttribute("data-id") ?? ""}]: ${(d.textContent ?? "").trim()}`,
      );
      return `\n\n${lines.join("\n")}\n\n`;
    },
  });
}

export function registerTaskRules(turndown: TurndownService): void {
  turndown.addRule("taskList", {
    filter: (node) =>
      node.nodeName === "UL" &&
      (node as HTMLElement).getAttribute("data-type") === "taskList",
    replacement: (content) => `\n${content.replace(/^\n+|\n+$/g, "")}\n`,
  });
  turndown.addRule("taskItem", {
    filter: (node) =>
      node.nodeName === "LI" &&
      (node as HTMLElement).getAttribute("data-type") === "taskItem",
    replacement: (content, node) => {
      const li = node as HTMLElement;
      const input = li.querySelector('input[type="checkbox"]') as HTMLInputElement | null;
      const checked = input ? input.checked : li.getAttribute("data-checked") === "true";
      // First line is the item text; the rest are nested lists, indented.
      const lines = content.replace(/^\n+|\n+$/g, "").split("\n");
      const first = (lines.shift() ?? "").trim();
      let out = `\n- ${checked ? "[x]" : "[ ]"} ${first}`;
      for (const line of lines) {
        if (line.trim() === "") continue;
        out += `\n    ${line}`;
      }
      return `${out}\n`;
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
