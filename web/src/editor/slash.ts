import type { Editor, Range } from "@tiptap/core";
import { Extension } from "@tiptap/core";
import Suggestion from "@tiptap/suggestion";

export interface SlashItem {
  title: string;
  hint: string;
  run: (editor: Editor, range: Range) => void;
}

export function slashItems(tr: (key: string) => string): SlashItem[] {
  return [
    {
      title: tr("slash.h1"),
      hint: tr("slash.h1hint"),
      run: (editor, range) =>
        editor.chain().focus().deleteRange(range).setNode("heading", { level: 1 }).run(),
    },
    {
      title: tr("slash.h2"),
      hint: tr("slash.h2hint"),
      run: (editor, range) =>
        editor.chain().focus().deleteRange(range).setNode("heading", { level: 2 }).run(),
    },
    {
      title: tr("slash.h3"),
      hint: tr("slash.h3hint"),
      run: (editor, range) =>
        editor.chain().focus().deleteRange(range).setNode("heading", { level: 3 }).run(),
    },
    {
      title: tr("slash.bullet"),
      hint: tr("slash.bullethint"),
      run: (editor, range) =>
        editor.chain().focus().deleteRange(range).toggleBulletList().run(),
    },
    {
      title: tr("slash.ordered"),
      hint: tr("slash.orderedhint"),
      run: (editor, range) =>
        editor.chain().focus().deleteRange(range).toggleOrderedList().run(),
    },
    {
      title: tr("slash.quote"),
      hint: tr("slash.quotehint"),
      run: (editor, range) =>
        editor.chain().focus().deleteRange(range).toggleBlockquote().run(),
    },
    {
      title: tr("slash.code"),
      hint: tr("slash.codehint"),
      run: (editor, range) =>
        editor.chain().focus().deleteRange(range).toggleCodeBlock().run(),
    },
    {
      title: tr("slash.table"),
      hint: tr("slash.tablehint"),
      run: (editor, range) =>
        editor
          .chain()
          .focus()
          .deleteRange(range)
          .insertTable({ rows: 3, cols: 3, withHeaderRow: true })
          .run(),
    },
    {
      title: tr("slash.task"),
      hint: tr("slash.taskhint"),
      run: (editor, range) =>
        editor.chain().focus().deleteRange(range).toggleTaskList().run(),
    },
    {
      title: tr("slash.hr"),
      hint: tr("slash.hrhint"),
      run: (editor, range) =>
        editor.chain().focus().deleteRange(range).setHorizontalRule().run(),
    },
  ];
}

export const SlashCommand = Extension.create({
  name: "slashCommand",
  addOptions() {
    return {
      suggestion: {
        char: "/",
        startOfLine: true,
        allowSpaces: false,
        items: ({ query }: { query: string }) =>
          slashItems((key) => key).filter((item) =>
            item.title.toLowerCase().includes(query.toLowerCase()),
          ),
        command: ({
          editor,
          range,
          props,
        }: {
          editor: Editor;
          range: Range;
          props: SlashItem;
        }) => props.run(editor, range),
      },
    };
  },
  addProseMirrorPlugins() {
    return [Suggestion({ editor: this.editor, ...this.options.suggestion })];
  },
});
