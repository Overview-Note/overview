<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { onBeforeRouteLeave, useRouter } from "vue-router";
import { BubbleMenu, EditorContent, useEditor } from "@tiptap/vue-3";
import { TableOfContents } from "@tiptap/extension-table-of-contents";
import type { SuggestionKeyDownProps, SuggestionProps } from "@tiptap/suggestion";
import { api, ApiError, type AIStatus, type Asset } from "../api";
import { baseExtensions } from "../editor/extensions";
import { SlashCommand, slashItems, type SlashItem } from "../editor/slash";
import { compressImage } from "../media/compress";
import type { TocItem } from "../editor/toc";
import { t } from "../i18n";
import { renderMermaid } from "../markdown/diagrams";
import { createTurndown, htmlToMarkdown, mdToHtml } from "../markdown/pipeline";
import { useDialogStore } from "../stores/dialog";
import { useSettingsStore } from "../stores/settings";
import { useWorkspaceStore } from "../stores/workspace";
import AgentPanel from "./AgentPanel.vue";
import AiPanel from "./AiPanel.vue";
import HistoryPanel from "./HistoryPanel.vue";
import LinksPanel from "./LinksPanel.vue";
import SlashMenu from "./SlashMenu.vue";
import TocPanel from "./TocPanel.vue";

const props = defineProps<{ path: string }>();

const store = useWorkspaceStore();
const dialogs = useDialogStore();
const settings = useSettingsStore();
const router = useRouter();

const status = ref<"idle" | "saving" | "saved">("idle");
const dirty = ref(false);
const fileInput = ref<HTMLInputElement | null>(null);
const attachmentInput = ref<HTMLInputElement | null>(null);
const slashMenu = ref<InstanceType<typeof SlashMenu> | null>(null);
const scrollEl = ref<HTMLElement | null>(null);
const toc = ref<TocItem[]>([]);
const showToc = ref(true);
const isPublic = ref(false);
const showAI = ref(false);
const aiStatus = ref<AIStatus | null>(null);
const aiTab = ref<"assistant" | "agent">("assistant");
const aiSelection = ref("");
const aiEnabled = computed(() => aiStatus.value?.enabled ?? false);
const showHistory = ref(false);
const visibilityMenuOpen = ref(false);
const visSelect = ref<HTMLElement | null>(null);
const isMobile = ref(false);
const sideOpen = ref(false);
const sideTab = ref<"toc" | "links" | "history" | "ai">("toc");
const aiContent = ref("");
const flashMessage = ref("");
let flashTimer: number | undefined;
let suppress = false;
let loading = false;
let saveTimer: number | undefined;
let mermaidTimer: number | undefined;
let loadSeq = 0;
let noteAbort: AbortController | undefined;
let currentPath = "";
let baseVersion = "";

const turndown = createTurndown();

const tocVisible = computed(() =>
  isMobile.value ? sideOpen.value && sideTab.value === "toc" : showToc.value,
);
const aiVisible = computed(() =>
  isMobile.value ? sideOpen.value && sideTab.value === "ai" : showAI.value,
);
const historyVisible = computed(() =>
  isMobile.value ? sideOpen.value && sideTab.value === "history" : showHistory.value,
);
const linksVisible = computed(() =>
  isMobile.value ? sideOpen.value && sideTab.value === "links" : !!store.note,
);

let mobileQuery: MediaQueryList | undefined;

function onMobileChange(event: MediaQueryListEvent) {
  isMobile.value = event.matches;
  if (!event.matches) sideOpen.value = false;
}

function openSide(tab: "toc" | "links" | "history" | "ai") {
  if (!isMobile.value) return;
  sideTab.value = tab;
  sideOpen.value = true;
}

function onTocClick() {
  if (isMobile.value) {
    openSide("toc");
    return;
  }
  showToc.value = !showToc.value;
}

function onHistoryClick() {
  if (isMobile.value) {
    if (store.note) openSide("history");
    return;
  }
  showHistory.value = !showHistory.value;
}

function onAiClick() {
  if (isMobile.value) {
    openSide("ai");
    if (!aiEnabled.value) {
      void dialogs.askConfirm(t("ai.title"), t("ai.notConfiguredHint"), false);
    }
    return;
  }
  toggleAI();
}

function onMobileEscape(event: KeyboardEvent) {
  if (event.key === "Escape" && sideOpen.value) {
    sideOpen.value = false;
  }
}

async function uploadAndInsert(original: File) {
  const isImage = original.type.startsWith("image/");
  try {
    let file = original;
    if (isImage && settings.compressImages) {
      file = await compressImage(original);
    }
    const asset = await api.uploadAsset(file);
    if (isImage) {
      editor.value?.chain().focus().setImage({ src: asset.url, alt: file.name }).run();
      if (file !== original && original.size > 0) {
        const saved = Math.round((1 - file.size / original.size) * 100);
        if (saved >= 10) {
          flashStatus(t("editor.compressed", { percent: saved }));
        }
      }
      return;
    }
    insertAttachment(asset);
  } catch (e) {
    await dialogs.askConfirm(t("editor.uploadFailed"), (e as Error).message, false);
  }
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function insertAttachment(asset: Asset) {
  // Keep the served "/assets/..." href so Tiptap's Link mark accepts it; the
  // save pipeline (toVaultMarkdown) rewrites it back to a vault-relative path.
  const href = asset.url;
  const title = `${asset.name} (${formatSize(asset.size)})`;
  editor.value
    ?.chain()
    .focus()
    .insertContent({
      type: "text",
      text: asset.name,
      marks: [{ type: "link", attrs: { href, title } }],
    })
    .run();
  flashStatus(t("editor.attachmentInserted", { name: asset.name }));
}

function flashStatus(message: string) {
  flashMessage.value = message;
  window.clearTimeout(flashTimer);
  flashTimer = window.setTimeout(() => {
    flashMessage.value = "";
  }, 4000);
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

function syncAiContent() {
  aiContent.value = editor.value?.getText() ?? "";
}

const editor = useEditor({
  content: "",
  extensions: [
    ...baseExtensions(),
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
    // Ignore programmatic content changes while a note is loading so opening a
    // note never marks it dirty or triggers a save.
    if (suppress || loading) return;
    dirty.value = true;
    syncAiContent();
    scheduleSave();
  },
  onSelectionUpdate: () => {
    aiSelection.value = selectedText();
  },
});

function selectedText(): string {
  const e = editor.value;
  if (!e) return "";
  const { from, to } = e.state.selection;
  if (from === to) return "";
  return e.state.doc.textBetween(from, to, "\n");
}

function insertAI(payload: { text: string; replace: boolean }) {
  if (!editor.value) return;
  if (payload.replace) {
    editor.value.commands.setContent(mdToHtml(payload.text));
  } else {
    editor.value.chain().focus("end").insertContent(`\n${payload.text}`).run();
  }
  syncAiContent();
  dirty.value = true;
  scheduleSave();
}

async function onAgentChanged(payload: { wrote: boolean; paths: string[] }) {
  if (!payload.wrote) return;
  await flush();
  await store.refreshTree();
  // A failed flush (e.g. a version conflict) leaves the editor dirty; don't
  // drop the user's unsaved edits by reloading over them.
  if (dirty.value) return;
  const current = store.note?.path ?? currentPath;
  if (current && (payload.paths.length === 0 || payload.paths.includes(current))) {
    await load(current);
  }
}

async function load(path: string) {
  if (!editor.value) return;
  const seq = ++loadSeq;
  window.clearTimeout(mermaidTimer);
  // Abort the previous note request so a slow load can't land after the switch.
  noteAbort?.abort();
  const controller = new AbortController();
  noteAbort = controller;
  try {
    const loaded = await store.openNote(path, controller.signal);
    // A newer note was opened while this request was in flight — drop it.
    if (seq !== loadSeq || !editor.value) return;
    baseVersion = loaded.version;
    isPublic.value = loaded.public;
    window.clearTimeout(saveTimer);
    loading = true;
    suppress = true;
    // Drop the outgoing note's images before replacing the DOM so their
    // in-flight fetches/decodes do not compete with the incoming note.
    cancelPendingImages();
    editor.value.commands.setContent(mdToHtml(loaded.body), false);
    suppress = false;
    // setContent emits an update that onUpdate swallows while suppressing, so
    // sync here too: otherwise the AI panel sees an empty note until the user
    // edits it once.
    syncAiContent();
    dirty.value = false;
    status.value = "idle";
    await nextTick();
    loading = false;
    if (seq === loadSeq) scheduleMermaid();
  } catch {
    if (controller.signal.aborted) return;
    if (seq === loadSeq) loading = false;
    // store.error already set
  }
}

// Cancels image loads that are still in flight (e.g. from a note the user just
// navigated away from) to avoid main-thread decode jank.
function cancelPendingImages() {
  const root = scrollEl.value;
  if (!root) return;
  root.querySelectorAll("img").forEach((img) => {
    if (!img.complete) img.removeAttribute("src");
  });
}

// Mermaid rendering is expensive; debounce it and skip diagrams for notes the
// user has already navigated away from while flicking through the tree.
function scheduleMermaid() {
  window.clearTimeout(mermaidTimer);
  const seq = loadSeq;
  mermaidTimer = window.setTimeout(() => {
    if (seq !== loadSeq) return;
    void renderMermaid(scrollEl.value);
  }, 160);
}

watch(
  [() => props.path, editor],
  async ([path], [oldPath]) => {
    if (!editor.value || !path) return;
    if (oldPath && oldPath !== path) {
      // Persist the previous note in the background so switching stays snappy.
      void flush();
    }
    await load(path);
    currentPath = path;
    if (isMobile.value) sideOpen.value = false;
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
  const md = htmlToMarkdown(turndown, editor.value.getHTML());
  const previousTitle = store.note?.title;
  dirty.value = false;
  status.value = "saving";
  try {
    const saved = await api.saveNote(path, md, version, isPublic.value);
    store.applySaved(saved);
    // If the user switched notes mid-save, don't touch the new note's state.
    if (currentPath === path) {
      baseVersion = saved.version;
      status.value = "saved";
    }
    if (saved.title !== previousTitle) {
      await store.refreshTree();
    }
  } catch (e) {
    if (currentPath === path) {
      dirty.value = true;
      status.value = "idle";
    }
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

function publicURL(): string {
  return `${location.origin}/public/${currentPath.split("/").map(encodeURIComponent).join("/")}`;
}

function onDocumentClick(event: MouseEvent) {
  if (!visibilityMenuOpen.value) return;
  const el = visSelect.value;
  if (el && !el.contains(event.target as Node)) {
    visibilityMenuOpen.value = false;
  }
}

function setVisibility(next: boolean) {
  visibilityMenuOpen.value = false;
  if (isPublic.value === next) return;
  isPublic.value = next;
  dirty.value = true;
  scheduleSave();
  if (next) {
    void navigator.clipboard?.writeText(publicURL()).catch(() => undefined);
    flashStatus(t("visibility.copied"));
  }
}

function openPublicPage() {
  if (isPublic.value && currentPath) {
    window.open(publicURL(), "_blank", "noopener");
  }
}

async function gotoPublicPage() {
  if (!currentPath) return;
  if (!isPublic.value) {
    const ok = await dialogs.askConfirm(
      t("editor.publishTitle"),
      t("editor.publishMessage"),
      false,
    );
    if (!ok) return;
    setVisibility(true);
  }
  if (dirty.value) await flush();
  openPublicPage();
}

function toggleAI() {
  showAI.value = !showAI.value;
  if (showAI.value && !aiEnabled.value) {
    void dialogs.askConfirm(t("ai.title"), t("ai.notConfiguredHint"), false);
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
  if (isMobile.value) sideOpen.value = false;
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

function insertInlineMath() {
  void (async () => {
    const tex = await dialogs.ask(t("editor.mathPrompt"), "");
    if (!tex) return;
    editor.value?.chain().focus().insertInlineMath(tex).run();
  })();
}

function insertBlockMath() {
  void (async () => {
    const tex = await dialogs.ask(t("editor.mathPrompt"), "");
    if (!tex) return;
    editor.value?.chain().focus().insertBlockMath(tex).run();
  })();
}

function insertMermaid() {
  void (async () => {
    const source = await dialogs.ask(
      t("editor.mermaidPrompt"),
      "graph TD\n  A[开始] --> B[结束]",
    );
    if (!source) return;
    editor.value
      ?.chain()
      .focus()
      .insertContent({ type: "mermaidBlock", attrs: { source } })
      .run();
    void nextTick(() => renderMermaid(scrollEl.value));
  })();
}

function onPickImage(event: Event) {
  const input = event.target as HTMLInputElement;
  if (input.files) handleFiles(input.files);
  input.value = "";
}

function onPickAttachment(event: Event) {
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
  window.addEventListener("click", onDocumentClick);
  window.addEventListener("keydown", onMobileEscape);
  mobileQuery = window.matchMedia?.("(max-width: 768px)");
  if (mobileQuery) {
    isMobile.value = mobileQuery.matches;
    mobileQuery.addEventListener("change", onMobileChange);
  }
  try {
    aiStatus.value = await api.aiStatus();
  } catch {
    aiStatus.value = null;
  }
});
onBeforeUnmount(() => {
  window.clearTimeout(saveTimer);
  window.clearTimeout(flashTimer);
  window.clearTimeout(mermaidTimer);
  noteAbort?.abort();
  window.removeEventListener("beforeunload", onBeforeUnload);
  window.removeEventListener("click", onDocumentClick);
  window.removeEventListener("keydown", onMobileEscape);
  mobileQuery?.removeEventListener("change", onMobileChange);
});
</script>

<template>
  <div class="editor-pane">
    <div class="note-bar">
      <div class="note-bar-title">{{ store.note?.title || store.note?.path }}</div>
      <div class="note-bar-actions">
        <button
          class="note-bar-public"
          :title="t('editor.publicPage')"
          @click="gotoPublicPage"
        >
          {{ t("editor.publicPage") }}
        </button>
        <button
          :class="{ on: isMobile ? sideOpen && sideTab === 'toc' : showToc }"
          :title="t('editor.toc')"
          @click="onTocClick"
        >
          {{ t("editor.toc") }}
        </button>
        <button
          :class="{ on: isMobile ? sideOpen && sideTab === 'history' : showHistory }"
          :title="t('history.title')"
          @click="onHistoryClick"
        >
          {{ t("history.title") }}
        </button>
        <button
          :class="{ on: isMobile ? sideOpen && sideTab === 'ai' : showAI }"
          :title="aiEnabled ? t('ai.title') : t('ai.notConfigured')"
          @click="onAiClick"
        >
          {{ t("ai.title") }}
        </button>
        <button
          v-if="isMobile"
          :class="{ on: sideOpen && sideTab === 'links' }"
          :title="t('links.backlinks')"
          @click="openSide('links')"
        >
          {{ t("links.backlinks") }}
        </button>
        <div ref="visSelect" class="vis-select">
          <button
            class="vis-trigger"
            :class="{ on: isPublic }"
            :title="t('visibility.title')"
            @click="visibilityMenuOpen = !visibilityMenuOpen"
          >
            <svg
              v-if="isPublic"
              class="vis-icon"
              viewBox="0 0 24 24"
              width="15"
              height="15"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <circle cx="12" cy="12" r="9" />
              <path d="M3 12h18M12 3c2.5 2.5 2.5 15 0 18M12 3c-2.5 2.5-2.5 15 0 18" />
            </svg>
            <svg
              v-else
              class="vis-icon"
              viewBox="0 0 24 24"
              width="15"
              height="15"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <rect x="4" y="10" width="16" height="11" rx="2" />
              <path d="M8 10V7a4 4 0 0 1 8 0v3" />
            </svg>
            <span>{{ isPublic ? t("visibility.public") : t("visibility.private") }}</span>
          </button>
          <div v-if="visibilityMenuOpen" class="vis-menu">
            <button :class="{ on: !isPublic }" @click="setVisibility(false)">
              <span class="vis-icon">
                <svg
                  viewBox="0 0 24 24"
                  width="15"
                  height="15"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <rect x="4" y="10" width="16" height="11" rx="2" />
                  <path d="M8 10V7a4 4 0 0 1 8 0v3" />
                </svg>
              </span>
              <span class="vis-text">
                <span class="vis-name">{{ t("visibility.private") }}</span>
                <small>{{ t("visibility.privateHint") }}</small>
              </span>
            </button>
            <button :class="{ on: isPublic }" @click="setVisibility(true)">
              <span class="vis-icon">
                <svg
                  viewBox="0 0 24 24"
                  width="15"
                  height="15"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <circle cx="12" cy="12" r="9" />
                  <path d="M3 12h18M12 3c2.5 2.5 2.5 15 0 18M12 3c-2.5 2.5-2.5 15 0 18" />
                </svg>
              </span>
              <span class="vis-text">
                <span class="vis-name">{{ t("visibility.public") }}</span>
                <small>{{ t("visibility.publicHint") }}</small>
              </span>
            </button>
            <button v-if="isPublic" class="vis-open" @click="openPublicPage()">
              <span class="vis-icon">
                <svg
                  viewBox="0 0 24 24"
                  width="15"
                  height="15"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <path
                    d="M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5"
                  />
                </svg>
              </span>
              <span class="vis-text">
                <span class="vis-name">{{ t("visibility.open") }}</span>
              </span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="editor-body">
      <div class="editor-main">
        <div class="editor-toolbar">
          <div class="toolbar-group">
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
            <button
              :class="{ on: isActive('taskList') }"
              @click="editor?.chain().focus().toggleTaskList().run()"
            >
              {{ t("editor.taskList") }}
            </button>
            <span class="sep"></span>
            <button :class="{ on: isActive('link') }" @click="setLink">
              {{ t("editor.link") }}
            </button>
            <button @click="fileInput?.click()">{{ t("editor.image") }}</button>
            <button @click="attachmentInput?.click()">{{ t("editor.attachment") }}</button>
            <button :class="{ on: isActive('table') }" @click="insertTable">
              {{ t("editor.table") }}
            </button>
            <button @click="insertInlineMath">{{ t("editor.mathInline") }}</button>
            <button @click="insertBlockMath">{{ t("editor.mathBlock") }}</button>
            <button @click="insertMermaid">{{ t("editor.mermaid") }}</button>
            <span class="sep"></span>
            <button @click="editor?.chain().focus().undo().run()">
              {{ t("editor.undo") }}
            </button>
            <button @click="editor?.chain().focus().redo().run()">
              {{ t("editor.redo") }}
            </button>
          </div>
          <span class="status">
            {{
              flashMessage ||
              (status === "saving"
                ? t("editor.saving")
                : status === "saved"
                  ? t("editor.saved")
                  : dirty
                    ? t("editor.unsaved")
                    : "")
            }}
          </span>
        </div>
        <div ref="scrollEl" class="editor-scroll">
          <div class="editor-crumb">{{ store.note?.path }}</div>
          <EditorContent :editor="editor" />
        </div>
        <BubbleMenu
          v-if="editor"
          :editor="editor"
          :should-show="() => editor!.isActive('table')"
          :tippy-options="{ duration: 100, placement: 'top' }"
        >
          <div class="table-bubble">
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
            <button
              class="danger-text"
              @click="editor?.chain().focus().deleteTable().run()"
            >
              {{ t("editor.delTable") }}
            </button>
          </div>
        </BubbleMenu>
      </div>

      <div
        v-if="isMobile && sideOpen"
        class="sheet-backdrop"
        @click="sideOpen = false"
      ></div>

      <div
        class="editor-side"
        :class="{ 'mobile-open': isMobile && sideOpen }"
        v-if="showToc || showAI || showHistory || store.note"
      >
        <div v-if="isMobile" class="sheet-head">
          <div class="sheet-tabs">
            <button
              :class="{ on: sideTab === 'toc' }"
              @click="sideTab = 'toc'"
            >
              {{ t("toc.title") }}
            </button>
            <button
              v-if="store.note"
              :class="{ on: sideTab === 'links' }"
              @click="sideTab = 'links'"
            >
              {{ t("links.backlinks") }}
            </button>
            <button
              v-if="store.note"
              :class="{ on: sideTab === 'history' }"
              @click="sideTab = 'history'"
            >
              {{ t("history.title") }}
            </button>
            <button
              :class="{ on: sideTab === 'ai' }"
              @click="sideTab = 'ai'"
            >
              {{ t("ai.title") }}
            </button>
          </div>
          <button
            class="sheet-close"
            :aria-label="t('tokens.close')"
            @click="sideOpen = false"
          >
            ✕
          </button>
        </div>
        <TocPanel v-if="tocVisible" :items="toc" @navigate="navigateHeading" />
        <div v-if="aiVisible" class="ai-section">
          <div class="segmented ai-tabs">
            <button
              :class="{ on: aiTab === 'assistant' }"
              @click="aiTab = 'assistant'"
            >
              {{ t("ai.tabAssistant") }}
            </button>
            <button :class="{ on: aiTab === 'agent' }" @click="aiTab = 'agent'">
              {{ t("ai.tabAgent") }}
            </button>
          </div>
          <AiPanel
            v-if="aiTab === 'assistant'"
            :content="aiContent"
            :enabled="aiEnabled"
            @insert="insertAI"
          />
          <AgentPanel
            v-else
            :path="store.note?.path ?? ''"
            :selection="aiSelection"
            :status="aiStatus"
            @insert="insertAI"
            @changed="onAgentChanged"
          />
        </div>
        <HistoryPanel
          v-if="historyVisible && store.note"
          :path="store.note.path"
          @restored="load(store.note!.path)"
        />
        <LinksPanel
          v-if="linksVisible && store.note"
          :path="store.note.path"
          @navigate="(p) => router.push({ name: 'note', params: { path: p } })"
          @create="openWiki"
        />
      </div>
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
    <input
      ref="attachmentInput"
      type="file"
      multiple
      hidden
      @change="onPickAttachment"
    />
  </div>
</template>
