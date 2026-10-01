/**
 * Minimal shape of a table-of-contents entry. Defining our own type keeps the
 * UI decoupled from the library's overly-specific `TableOfContentDataItem`
 * (whose `editor` field type is not assignable across editor instances).
 */
export interface TocItem {
  id: string;
  level: number;
  textContent: string;
  isActive: boolean;
  isScrolledOver: boolean;
  dom: HTMLElement;
}
