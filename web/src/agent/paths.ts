const PATH_KEYS = new Set(["path", "from", "to", "target"]);

function looksLikePath(value: string): boolean {
  const v = value.trim();
  if (!v || v.length > 400) return false;
  if (/\s/.test(v)) return false;
  return /\.md$/i.test(v) || v.includes("/") || v.includes("\\");
}

function parseJSON(value: string): unknown {
  try {
    return JSON.parse(value);
  } catch {
    return undefined;
  }
}

function collect(value: unknown, out: Set<string>): void {
  if (typeof value === "string") {
    const trimmed = value.trim();
    const asJSON = trimmed.startsWith("{") || trimmed.startsWith("[")
      ? parseJSON(trimmed)
      : undefined;
    if (asJSON !== undefined) collect(asJSON, out);
    else if (looksLikePath(trimmed)) out.add(trimmed);
    return;
  }
  if (Array.isArray(value)) {
    for (const item of value) collect(item, out);
    return;
  }
  if (value && typeof value === "object") {
    for (const [key, item] of Object.entries(value as Record<string, unknown>)) {
      if (PATH_KEYS.has(key) && typeof item === "string" && item.trim()) {
        out.add(item.trim());
      } else if (item && typeof item === "object") {
        collect(item, out);
      }
    }
  }
}

// Extracts candidate note paths from tool arguments and result summaries. The
// agent reports `args` as raw JSON and `summary` as the tool's textual output,
// so both plain and JSON-stringified payloads are handled.
export function extractPaths(...values: unknown[]): string[] {
  const out = new Set<string>();
  for (const value of values) collect(value, out);
  return [...out];
}
