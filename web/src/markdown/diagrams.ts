// Mermaid diagram rendering.
//
// Diagrams stay as ```mermaid fenced code blocks in Markdown (portable, and
// other tools show the source). In the editor and public pages we render them
// to inline SVG. Rendering is lazy and de-duplicated so importing mermaid does
// not block the initial bundle.

let mermaidPromise: Promise<typeof import("mermaid").default> | null = null;

async function getMermaid() {
  if (!mermaidPromise) {
    mermaidPromise = import("mermaid").then((mod) => {
      const mermaid = mod.default;
      mermaid.initialize({
        startOnLoad: false,
        securityLevel: "strict",
        theme: document.documentElement.dataset.theme === "dark" ? "dark" : "default",
        fontFamily: "var(--font-sans)",
      });
      return mermaid;
    });
  }
  return mermaidPromise;
}

let renderSeq = 0;

/** Renders every unprocessed `.mermaid` element inside `root`. */
export async function renderMermaid(root: HTMLElement | null): Promise<void> {
  if (!root) return;
  const targets = Array.from(
    root.querySelectorAll<HTMLElement>(".mermaid:not([data-processed])"),
  );
  await Promise.all(targets.map((el) => renderMermaidElement(el)));
}

/** Renders a single `.mermaid` element to inline SVG. */
export async function renderMermaidElement(el: HTMLElement): Promise<void> {
  if (el.dataset.processed) return;
  el.dataset.processed = "1";
  const source = el.dataset.source ?? el.textContent ?? "";
  let mermaid: Awaited<ReturnType<typeof getMermaid>>;
  try {
    mermaid = await getMermaid();
  } catch {
    return;
  }
  try {
    const id = `mmd-${Date.now()}-${renderSeq++}`;
    const { svg } = await mermaid.render(id, source);
    el.innerHTML = svg;
    el.classList.add("mermaid-rendered");
  } catch (e) {
    el.classList.add("mermaid-error");
    el.textContent = `${source}\n\n${(e as Error).message}`;
  }
}

/** Converts ```mermaid fenced blocks into `.mermaid` divs for rendering. */
export function mermaidToDivs(html: string): string {
  return html.replace(
    /<pre><code class="language-mermaid">([\s\S]*?)<\/code><\/pre>/g,
    (_m, code: string) => {
      const source = decodeEntities(code);
      return `<div class="mermaid" data-source="${escapeAttr(source)}">${escapeHtml(
        source,
      )}</div>`;
    },
  );
}

function decodeEntities(s: string): string {
  return s
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&amp;/g, "&");
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function escapeAttr(s: string): string {
  return escapeHtml(s).replace(/"/g, "&quot;");
}
