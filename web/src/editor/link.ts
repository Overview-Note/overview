import Link from "@tiptap/extension-link";

// Extends the Tiptap Link mark so wiki-links keep their data-wiki attribute
// through the editor. Without this, Tiptap drops the attribute on parse and the
// Turndown rule can no longer tell a wiki-link from a normal link, so
// [[Note]] would be saved as [Note](#).
export const WikiLink = Link.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      wiki: {
        default: null,
        parseHTML: (element: HTMLElement) => element.getAttribute("data-wiki"),
        renderHTML: (attributes: Record<string, unknown>) =>
          attributes.wiki
            ? { "data-wiki": attributes.wiki as string, class: "wiki-link" }
            : {},
      },
    };
  },
});
