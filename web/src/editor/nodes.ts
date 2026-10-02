// Custom Tiptap nodes for math (KaTeX), Mermaid diagrams and footnotes.
//
// All are "atom" nodes: they keep their source in a data attribute so the
// Markdown round-trip is lossless, while the editor shows the rendered result
// through a lightweight NodeView (no ProseMirror content inside).

import { Node, mergeAttributes } from "@tiptap/core";
import { renderMermaidElement } from "../markdown/diagrams";
import { renderMath } from "../markdown/math";

declare module "@tiptap/core" {
  interface Commands<ReturnType> {
    math: {
      insertInlineMath: (tex: string) => ReturnType;
      insertBlockMath: (tex: string) => ReturnType;
    };
  }
}

function mathView(displayMode: boolean) {
  return ({ node }: { node: { type: { name: string }; attrs: { tex: string } } }) => {
    const dom = document.createElement(displayMode ? "div" : "span");
    dom.className = displayMode ? "math-block" : "math-inline";
    dom.innerHTML = renderMath(node.attrs.tex, displayMode);
    return {
      dom,
      update: (updated: { type: { name: string }; attrs: { tex: string } }) => {
        if (updated.type.name !== node.type.name) return false;
        dom.innerHTML = renderMath(updated.attrs.tex, displayMode);
        return true;
      },
      ignoreMutation: () => true,
    };
  };
}

export const InlineMath = Node.create({
  name: "inlineMath",
  group: "inline",
  inline: true,
  atom: true,
  selectable: true,

  addAttributes() {
    return {
      tex: {
        default: "",
        parseHTML: (el) => el.getAttribute("data-tex") ?? el.textContent ?? "",
        renderHTML: (attrs) => ({ "data-tex": attrs.tex }),
      },
    };
  },

  parseHTML() {
    return [{ tag: "span.math-inline" }, { tag: "span[data-tex]" }];
  },

  renderHTML({ node, HTMLAttributes }) {
    return [
      "span",
      mergeAttributes(HTMLAttributes, { class: "math-inline" }),
      `$${node.attrs.tex}$`,
    ];
  },

  addNodeView() {
    return mathView(false) as never;
  },

  addCommands() {
    return {
      insertInlineMath:
        (tex) =>
        ({ commands }) =>
          commands.insertContent({
            type: this.name,
            attrs: { tex },
          }),
      insertBlockMath:
        (tex) =>
        ({ commands }) =>
          commands.insertContent({
            type: BlockMath.name,
            attrs: { tex },
          }),
    };
  },
});

export const BlockMath = Node.create({
  name: "blockMath",
  group: "block",
  atom: true,

  addAttributes() {
    return {
      tex: {
        default: "",
        parseHTML: (el) => el.getAttribute("data-tex") ?? el.textContent ?? "",
        renderHTML: (attrs) => ({ "data-tex": attrs.tex }),
      },
    };
  },

  parseHTML() {
    return [{ tag: "div.math-block" }];
  },

  renderHTML({ node, HTMLAttributes }) {
    return [
      "div",
      mergeAttributes(HTMLAttributes, { class: "math-block" }),
      `$$${node.attrs.tex}$$`,
    ];
  },

  addNodeView() {
    return mathView(true) as never;
  },
});

export const FootnoteRef = Node.create({
  name: "footnoteRef",
  group: "inline",
  inline: true,
  atom: true,
  selectable: true,

  addAttributes() {
    return {
      id: {
        default: "",
        parseHTML: (el) => el.getAttribute("data-id") ?? "",
        renderHTML: (attrs) => ({ "data-id": attrs.id }),
      },
    };
  },

  parseHTML() {
    return [{ tag: "sup.fn-ref" }];
  },

  renderHTML({ node, HTMLAttributes }) {
    return [
      "sup",
      mergeAttributes(HTMLAttributes, { class: "fn-ref" }),
      `[${node.attrs.id}]`,
    ];
  },
});

export const FootnoteDefs = Node.create({
  name: "footnoteDefs",
  group: "block",
  atom: true,

  addAttributes() {
    return {
      items: {
        default: [] as Array<{ id: string; text: string }>,
        parseHTML: (el) => {
          const items: Array<{ id: string; text: string }> = [];
          el.querySelectorAll<HTMLElement>(".fn-def").forEach((d) => {
            items.push({
              id: d.getAttribute("data-id") ?? "",
              text: d.textContent ?? "",
            });
          });
          return items;
        },
        renderHTML: (attrs) => ({ "data-items": JSON.stringify(attrs.items) }),
      },
    };
  },

  parseHTML() {
    return [{ tag: "div.fn-defs" }];
  },

  renderHTML({ node, HTMLAttributes }) {
    return [
      "div",
      mergeAttributes(HTMLAttributes, {
        class: "fn-defs",
        "data-items": JSON.stringify(node.attrs.items ?? []),
      }),
    ];
  },
});

function mermaidView() {
  return ({ node }: { node: { type: { name: string }; attrs: { source: string } } }) => {
    const dom = document.createElement("div");
    dom.className = "mermaid";
    dom.dataset.source = node.attrs.source;
    dom.textContent = node.attrs.source;
    void renderMermaidElement(dom);
    return {
      dom,
      update: (updated: { type: { name: string }; attrs: { source: string } }) => {
        if (updated.type.name !== node.type.name) return false;
        if (updated.attrs.source === dom.dataset.source) return true;
        dom.dataset.source = updated.attrs.source;
        dom.removeAttribute("data-processed");
        dom.classList.remove("mermaid-rendered", "mermaid-error");
        dom.textContent = updated.attrs.source;
        void renderMermaidElement(dom);
        return true;
      },
      ignoreMutation: () => true,
    };
  };
}

export const MermaidBlock = Node.create({
  name: "mermaidBlock",
  group: "block",
  atom: true,

  addAttributes() {
    return {
      source: {
        default: "",
        parseHTML: (el) => el.getAttribute("data-source") ?? el.textContent ?? "",
        renderHTML: (attrs) => ({ "data-source": attrs.source }),
      },
    };
  },

  parseHTML() {
    return [{ tag: "div.mermaid" }];
  },

  renderHTML({ node, HTMLAttributes }) {
    return [
      "div",
      mergeAttributes(HTMLAttributes, {
        class: "mermaid",
        "data-source": node.attrs.source,
      }),
      node.attrs.source,
    ];
  },

  addNodeView() {
    return mermaidView() as never;
  },
});
