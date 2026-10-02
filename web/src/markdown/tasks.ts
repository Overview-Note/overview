// Converts the plain GFM task-list HTML that marked emits into the markup the
// Tiptap TaskList/TaskItem extensions expect. Without this, marked's
// `<li><input type="checkbox">…` is parsed as a normal bullet list and the
// checkbox state is lost on the next save.
export function normalizeTaskLists(html: string): string {
  if (!html.includes('type="checkbox"')) return html;

  const doc = new DOMParser().parseFromString(html, "text/html");
  // Process nested lists first so an outer <li> keeps its already-converted
  // descendant lists when we rebuild its markup.
  const lists = Array.from(doc.querySelectorAll("ul")).reverse();
  for (const ul of lists) {
    const items = Array.from(ul.children).filter(
      (child): child is HTMLLIElement => child.tagName === "LI",
    );
    if (items.length === 0) continue;
    const allTasks = items.every((li) =>
      li.querySelector(':scope > input[type="checkbox"]'),
    );
    if (!allTasks) continue;

    ul.setAttribute("data-type", "taskList");
    for (const li of items) {
      const input = li.querySelector(
        ':scope > input[type="checkbox"]',
      ) as HTMLInputElement;
      const checked = input.checked || input.hasAttribute("checked");
      const nested = Array.from(li.children).filter(
        (child) => child.tagName === "UL" || child.tagName === "OL",
      );
      const own = li.innerHTML
        .replace(/^\s*<input[^>]*>/i, "")
        .replace(/<(ul|ol)\b[\s\S]*$/i, "")
        .trim();

      let inner =
        `<label><input type="checkbox"${checked ? " checked" : ""}><span></span></label>` +
        `<div><p>${own}</p>`;
      for (const child of nested) inner += child.outerHTML;
      inner += `</div>`;

      li.setAttribute("data-type", "taskItem");
      li.setAttribute("data-checked", checked ? "true" : "false");
      li.innerHTML = inner;
    }
  }
  return doc.body.innerHTML;
}
