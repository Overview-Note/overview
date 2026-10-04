// Shared Tiptap extension set. Used by the editor component and by the
// round-trip tests so both parse/serialize notes identically.
import CodeBlockLowlight from "@tiptap/extension-code-block-lowlight";
import Image from "@tiptap/extension-image";
import Placeholder from "@tiptap/extension-placeholder";
import Table from "@tiptap/extension-table";
import TableCell from "@tiptap/extension-table-cell";
import TableHeader from "@tiptap/extension-table-header";
import TableRow from "@tiptap/extension-table-row";
import TaskItem from "@tiptap/extension-task-item";
import TaskList from "@tiptap/extension-task-list";
import StarterKit from "@tiptap/starter-kit";
import type { Extensions } from "@tiptap/core";

import { t } from "../i18n";
import { WikiLink } from "./link";
import { lowlight } from "./lowlight";
import { BlockMath, FootnoteDefs, FootnoteRef, InlineMath, MermaidBlock } from "./nodes";

export function baseExtensions(): Extensions {
  return [
    StarterKit.configure({
      heading: { levels: [1, 2, 3, 4] },
      codeBlock: false,
    }),
    CodeBlockLowlight.configure({
      lowlight,
      HTMLAttributes: { class: "code-block" },
    }),
    TaskList,
    TaskItem.configure({ nested: true }),
    InlineMath,
    BlockMath,
    MermaidBlock,
    FootnoteRef,
    FootnoteDefs,
    Image.configure({
      inline: false,
      allowBase64: false,
      HTMLAttributes: { loading: "lazy", decoding: "async" },
    }),
    WikiLink.configure({
      openOnClick: false,
      autolink: true,
      // Vault-relative attachment hrefs ("assets/...") are valid in notes but
      // rejected by the stock protocol allowlist; accept them so links pasted
      // or loaded in that form survive the editor round-trip.
      isAllowedUri: (url, ctx) => /^assets\//i.test(url.trim()) || ctx.defaultValidate(url),
    }),
    Placeholder.configure({ placeholder: t("editor.placeholder") }),
    Table.configure({ resizable: true, HTMLAttributes: { class: "md-table" } }),
    TableRow,
    TableHeader,
    TableCell,
  ];
}
