import type { Editor, Range } from "@tiptap/core";
import { Extension } from "@tiptap/core";
import Suggestion from "@tiptap/suggestion";

export interface SlashItem {
  title: string;
  hint: string;
  run: (editor: Editor, range: Range) => void;
}

export function slashItems(): SlashItem[] {
  return [
    {
      title: "标题 1",
      hint: "一级标题",
      run: (editor, range) =>
        editor.chain().focus().deleteRange(range).setNode("heading", { level: 1 }).run(),
    },
    {
      title: "标题 2",
      hint: "二级标题",
      run: (editor, range) =>
        editor.chain().focus().deleteRange(range).setNode("heading", { level: 2 }).run(),
    },
    {
      title: "标题 3",
      hint: "三级标题",
      run: (editor, range) =>
        editor.chain().focus().deleteRange(range).setNode("heading", { level: 3 }).run(),
    },
    {
      title: "无序列表",
      hint: "项目符号列表",
      run: (editor, range) =>
        editor.chain().focus().deleteRange(range).toggleBulletList().run(),
    },
    {
      title: "有序列表",
      hint: "编号列表",
      run: (editor, range) =>
        editor.chain().focus().deleteRange(range).toggleOrderedList().run(),
    },
    {
      title: "引用",
      hint: "引用块",
      run: (editor, range) =>
        editor.chain().focus().deleteRange(range).toggleBlockquote().run(),
    },
    {
      title: "代码块",
      hint: "代码片段",
      run: (editor, range) =>
        editor.chain().focus().deleteRange(range).toggleCodeBlock().run(),
    },
    {
      title: "表格",
      hint: "插入 3×3 表格",
      run: (editor, range) =>
        editor
          .chain()
          .focus()
          .deleteRange(range)
          .insertTable({ rows: 3, cols: 3, withHeaderRow: true })
          .run(),
    },
    {
      title: "分割线",
      hint: "水平分割线",
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
          slashItems().filter((item) =>
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
