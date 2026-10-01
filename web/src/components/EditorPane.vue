<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { onBeforeRouteLeave, useRouter } from "vue-router";
import { EditorContent, useEditor } from "@tiptap/vue-3";
import StarterKit from "@tiptap/starter-kit";
import Image from "@tiptap/extension-image";
import Link from "@tiptap/extension-link";
import Placeholder from "@tiptap/extension-placeholder";
import Table from "@tiptap/extension-table";
import TableRow from "@tiptap/extension-table-row";
import TableHeader from "@tiptap/extension-table-header";
import TableCell from "@tiptap/extension-table-cell";
import { TableOfContents } from "@tiptap/extension-table-of-contents";
import type { SuggestionKeyDownProps, SuggestionProps } from "@tiptap/suggestion";
import { marked } from "marked";
import TurndownService from "turndown";
import { gfm } from "turndown-plugin-gfm";
import { api, ApiError } from "../api";
import { SlashCommand, slashItems, type SlashItem } from "../editor/slash";
import type { TocItem } from "../editor/toc";
import { t } from "../i18n";
import { resolveAssetSrc, toVaultMarkdown } from "../markdown/assets";
import { registerWikiRule, wikiToHtml } from "../markdown/wiki";
import { useDialogStore } from "../stores/dialog";
import { useWorkspaceStore } from "../stores/workspace";
import AiPanel from "./AiPanel.vue";
import HistoryPanel from "./HistoryPanel.vue";
import LinksPanel from "./LinksPanel.vue";
import SlashMenu from "./SlashMenu.vue";
import TocPanel from "./TocPanel.vue";

const props = defineProps<{ path: string }>();

const store = useWorkspaceStore();
const dialogs = useDialogStore();
const router = useRouter();

const status = ref<"idle" | "saving" | "saved">("idle");
const dirty = ref(false);
const fileInput = ref<HTMLInputElement | null>(null);
const slashMenu = ref<InstanceType<typeof SlashMenu> | null>(null);
const scrollEl = ref<HTMLElement | null>(null);
const toc = ref<TocItem[]>([]);
const showToc = ref(true);
const isPublic = ref(false);
const showAI = ref(false);
const aiEnabled = ref(false);
const showHistory = ref(false);
const aiContent = ref("");
let suppress = false;
let saveTimer: number | undefined;
let currentPath = "";
let baseVersion = "";

marked.setOptions({ gfm: true, breaks: false });

const turndown = new TurndownService({
  headingStyle: "atx",
  codeBlockStyle: "fenced",
  bulletListMarker: "-",
  emDelimiter: "*",
  hr: "---",
});
turndown.use(gfm);
registerWikiRule(turndown);

function mdToHtml(md: string): string {
  const html = marked.parse(wikiToHtml(md), { async: false }) as string;
  return resolveAssetSrc(html);
}

async function uploadAndInsert(file: File) {
  try {
    const asset = await api.uploadAsset(file);
    editor.value?.chain().focus().setImage({ src: asset.url, alt: file.name }).run();
  } catch (e) {
    await dialogs.askConfirm(t("editor.uploadFailed"), (e as Error).message, false);
  }
}

function handleFiles(files?: FileList | null): boolean {
  if (!files || files.length === 0) return false;
  Array.from(files).forEach((f) => void uploadAndInsert(f));
  return true;
}

async function openWiki(target: string) {
  if (!target) return;
  try {
    const meta = await api.resolve(target);
    router.push({ name: "note", params: { path: meta.path } });
  } catch {
    const ok = await dialogs.askConfirm(
      t("editor.createLinkTitle"),
      t("editor.createLinkMessage", { target }),
      false,
    );
    if (!ok) return;
    const path = target.toLowerCase().endsWith(".md") ? target : `${target}.md`;
    const title = target.split("/").pop()?.replace(/\.md$/i, "") ?? target;
    try {
      const created = await api.saveNote(path, `# ${title}\n`, "*");
      await store.refreshTree();
      router.push({ name: "note", params: { path: created.path } });
    } catch (e) {
      await dialogs.askConfirm(t("editor.createFailed"), (e as Error).message, false);
    }
  }
}

const editor = useEditor({
  content: "",
  extensions: [
    StarterKit.configure({
      heading: { levels: [1, 2, 3, 4] },
      codeBlock: { HTMLAttributes: { class: "code-block" } },
    }),
    Image.configure({ inline: false, allowBase64: false }),
    Link.configure({ openOnClick: false, autolink: true }),
    Placeholder.configure({ placeholder: t("editor.placeholder") }),
    Table.configure({ resizable: true, HTMLAttributes: { class: "md-table" } }),
    TableRow,
    TableHeader,
    TableCell,
    TableOfContents.configure({
      scrollParent: () => scrollEl.value ?? window,
      onUpdate: (items) => {
        toc.value = items as unknown as TocItem[];
      },
    }),
    SlashCommand.configure({
      suggestion: {
        items: ({ query }: { query: string }) =>
          slashItems(t).filter((item) =>
            item.title.toLowerCase().includes(query.toLowerCase()),
          ),
        render: () => ({
          onStart: (p: SuggestionProps<SlashItem>) => slashMenu.value?.onStart(p),
          onUpdate: (p: SuggestionProps<SlashItem>) => slashMenu.value?.onUpdate(p),
          onKeyDown: (p: SuggestionKeyDownProps) =>
            slashMenu.value?.onKeyDown(p) ?? false,
          onExit: () => slashMenu.value?.onExit(),
        }),
      },
    }),
  ],
  editorProps: {
    attributes: { class: "tiptap-content" },
    handlePaste: (_view, event) => handleFiles(event.clipboardData?.files),
    handleDrop: (_view, event) => handleFiles((event as DragEvent).dataTransfer?.files),
    handleClick: (_view, _pos, event) => {
      const target = (event.target as HTMLElement | null)?.closest("a[data-wiki]");
      if (target) {
        event.preventDefault();
        void openWiki(target.getAttribute("data-wiki") ?? "");
        return true;
      }
      return false;
    },
  },
  onUpdate: () => {
    if (suppress) return;
    dirty.value = true;
    aiContent.value = editor.value?.getText() ?? "";
    scheduleSave();
  },
});

function insertAI(payload: { text: string; replace: boolean }) {
  if (!editor.value) return;
  if (payload.replace) {
    editor.value.commands.setContent(mdToHtml(payload.text));
  } else {
    editor.value.chain().focus("end").insertContent(`\n${payload.text}`).run();
  }
  dirty.value = true;
  scheduleSave();
}

async function load(path: string) {
  if (!editor.value) return;
  try {
    const loaded = await store.openNote(path);
    baseVersion = loaded.version;
    isPublic.value = loaded.public;
    suppress = true;
    editor.value.commands.setContent(mdToHtml(loaded.body), false);
    suppress = false;
    dirty.value = false;
    status.value = "idle";
  } catch {
    // store.error already set
  }
}

watch(
  [() => props.path, editor],
  async ([path], [oldPath]) => {
    if (!editor.value || !path) return;
    if (oldPath && oldPath !== path) {
      await flush();
    }
    await load(path);
    currentPath = path;
  },
  { immediate: true },
);

function scheduleSave() {
  status.value = "idle";
  window.clearTimeout(saveTimer);
  saveTimer = window.setTimeout(() => void flush(), 700);
}

async function flush() {
  if (!dirty.value || !editor.value || !currentPath) return;
  window.clearTimeout(saveTimer);
  const path = currentPath;
  const version = baseVersion;
  const md = toVaultMarkdown(turndown.turndown(editor.value.getHTML()));
  const previousTitle = store.note?.title;
  dirty.value = false;
  status.value = "saving";
  try {
    const saved = await api.saveNote(path, md, version, isPublic.value);
    baseVersion = saved.version;
    store.applySaved(saved);
    if (saved.title !== previousTitle) {
      await store.refreshTree();
    }
    status.value = "saved";
  } catch (e) {
    dirty.value = true;
    status.value = "idle";
    if (e instanceof ApiError && e.status === 409) {
      await dialogs.askConfirm(
        t("editor.saveConflictTitle"),
        t("editor.saveConflictMessage"),
        false,
      );
    } else {
      await dialogs.askConfirm(t("editor.saveFailed"), (e as Error).message, false);
    }
  }
}

function isActive(name: string, attrs?: Record<string, unknown>) {
  return editor.value?.isActive(name, attrs) ?? false;
}

async function togglePublic() {
  isPublic.value = !isPublic.value;
  dirty.value = true;
  scheduleSave();
  if (isPublic.value) {
    const url = `${location.origin}/public/${currentPath.split("/").map(encodeURIComponent).join("/")}`;
    await navigator.clipboard?.writeText(url).catch(() => undefined);
    await dialogs.askConfirm(t("editor.share"), t("editor.shareMessage", { url }), false);
  }
}

function navigateHeading(item: TocItem) {
  const container = scrollEl.value;
  if (!container) return;
  const top =
    item.dom.getBoundingClientRect().top -
    container.getBoundingClientRect().top +
    container.scrollTop -
    16;
  container.scrollTo({ top, behavior: "smooth" });
}

function setLink() {
  void (async () => {
    const prev = editor.value?.getAttributes("link").href as string | undefined;
    const url = await dialogs.ask(t("editor.linkTitle"), prev ?? "https://");
    if (url === null) return;
    if (url === "") {
      editor.value?.chain().focus().unsetLink().run();
      return;
    }
    editor.value?.chain().focus().extendMarkRange("link").setLink({ href: url }).run();
  })();
}

function insertTable() {
  editor.value
    ?.chain()
    .focus()
    .insertTable({ rows: 3, cols: 3, withHeaderRow: true })
    .run();
}

function onPickImage(event: Event) {
  const input = event.target as HTMLInputElement;
  if (input.files) handleFiles(input.files);
  input.value = "";
}

function onBeforeUnload(event: BeforeUnloadEvent) {
  if (dirty.value) {
    event.preventDefault();
    event.returnValue = "";
  }
}

onBeforeRouteLeave(async () => {
  await flush();
  return true;
});

onMounted(async () => {
  window.addEventListener("beforeunload", onBeforeUnload);
  try {
    aiEnabled.value = (await api.aiStatus()).enabled;
  } catch {
    aiEnabled.value = false;
  }
});
onBeforeUnmount(() => {
  window.clearTimeout(saveTimer);
  window.removeEventListener("beforeunload", onBeforeUnload);
});
</script>

<template>
  <div class="editor-pane">
    <div class="editor-toolbar">
      <button
        :class="{ on: isActive('heading', { level: 1 }) }"
        @click="editor?.chain().focus().toggleHeading({ level: 1 }).run()"
      >
        H1
      </button>
      <button
        :class="{ on: isActive('heading', { level: 2 }) }"
        @click="editor?.chain().focus().toggleHeading({ level: 2 }).run()"
      >
        H2
      </button>
      <button
        :class="{ on: isActive('heading', { level: 3 }) }"
        @click="editor?.chain().focus().toggleHeading({ level: 3 }).run()"
      >
        H3
      </button>
      <span class="sep"></span>
      <button
        :class="{ on: isActive('bold') }"
        @click="editor?.chain().focus().toggleBold().run()"
      >
        <b>B</b>
      </button>
      <button
        :class="{ on: isActive('italic') }"
        @click="editor?.chain().focus().toggleItalic().run()"
      >
        <i>I</i>
      </button>
      <button
        :class="{ on: isActive('strike') }"
        @click="editor?.chain().focus().toggleStrike().run()"
      >
        <s>S</s>
      </button>
      <button
        :class="{ on: isActive('code') }"
        @click="editor?.chain().focus().toggleCode().run()"
      >
        &lt;/&gt;
      </button>
      <span class="sep"></span>
      <button
        :class="{ on: isActive('bulletList') }"
        @click="editor?.chain().focus().toggleBulletList().run()"
      >
        {{ t("editor.listBullet") }}
      </button>
      <button
        :class="{ on: isActive('orderedList') }"
        @click="editor?.chain().focus().toggleOrderedList().run()"
      >
        {{ t("editor.listOrdered") }}
      </button>
      <button
        :class="{ on: isActive('blockquote') }"
        @click="editor?.chain().focus().toggleBlockquote().run()"
      >
        {{ t("editor.quote") }}
      </button>
      <button
        :class="{ on: isActive('codeBlock') }"
        @click="editor?.chain().focus().toggleCodeBlock().run()"
      >
        {{ t("editor.code") }}
      </button>
      <span class="sep"></span>
      <button :class="{ on: isActive('link') }" @click="setLink">
        {{ t("editor.link") }}
      </button>
      <button @click="fileInput?.click()">{{ t("editor.image") }}</button>
      <button :class="{ on: isActive('table') }" @click="insertTable">
        {{ t("editor.table") }}
      </button>
      <button
        :class="{ on: showToc }"
        :title="t('editor.toc')"
        @click="showToc = !showToc"
      >
        {{ t("editor.toc") }}
      </button>
      <button :class="{ on: isPublic }" :title="t('editor.share')" @click="togglePublic">
        {{ isPublic ? t("editor.shared") : t("editor.share") }}
      </button>
      <button
        :class="{ on: showHistory }"
        :title="t('history.title')"
        @click="showHistory = !showHistory"
      >
        {{ t("history.title") }}
      </button>
      <button
        v-if="aiEnabled"
        :class="{ on: showAI }"
        :title="t('ai.title')"
        @click="showAI = !showAI"
      >
        {{ t("ai.title") }}
      </button>
      <span class="sep"></span>
      <button @click="editor?.chain().focus().undo().run()">
        {{ t("editor.undo") }}
      </button>
      <button @click="editor?.chain().focus().redo().run()">
        {{ t("editor.redo") }}
      </button>
      <span class="status">
        {{
          status === "saving"
            ? t("editor.saving")
            : status === "saved"
              ? t("editor.saved")
              : dirty
                ? t("editor.unsaved")
                : ""
        }}
      </span>
    </div>
    <div v-if="isActive('table')" class="table-toolbar">
      <button @click="editor?.chain().focus().addRowAfter().run()">
        {{ t("editor.addRow") }}
      </button>
      <button @click="editor?.chain().focus().addColumnAfter().run()">
        {{ t("editor.addCol") }}
      </button>
      <button @click="editor?.chain().focus().deleteRow().run()">
        {{ t("editor.delRow") }}
      </button>
      <button @click="editor?.chain().focus().deleteColumn().run()">
        {{ t("editor.delCol") }}
      </button>
      <button @click="editor?.chain().focus().deleteTable().run()">
        {{ t("editor.delTable") }}
      </button>
    </div>

    <div class="editor-body">
      <TocPanel v-if="showToc" :items="toc" @navigate="navigateHeading" />
      <div ref="scrollEl" class="editor-scroll">
        <div class="editor-title">{{ store.note?.title || store.note?.path }}</div>
        <EditorContent :editor="editor" />
      </div>
      <AiPanel v-if="showAI" :content="aiContent" @insert="insertAI" />
      <HistoryPanel
        v-if="showHistory && store.note"
        :path="store.note.path"
        @restored="load(store.note!.path)"
      />
      <LinksPanel
        v-if="store.note"
        :path="store.note.path"
        @navigate="(p) => router.push({ name: 'note', params: { path: p } })"
        @create="openWiki"
      />
    </div>

    <SlashMenu ref="slashMenu" />
    <input
      ref="fileInput"
      type="file"
      accept="image/*"
      multiple
      hidden
      @change="onPickImage"
    />
  </div>
</template>
