import { defineStore } from "pinia";
import { ref } from "vue";
import { api, type Note, type SearchHit, type TreeNode } from "../api";

function parentOf(path: string): string {
  return path.includes("/") ? path.slice(0, path.lastIndexOf("/")) : "";
}

export const useWorkspaceStore = defineStore("workspace", () => {
  const tree = ref<TreeNode[]>([]);
  const note = ref<Note | null>(null);
  const loadingTree = ref(false);
  const loadingNote = ref(false);
  const error = ref<string | null>(null);

  async function refreshTree() {
    loadingTree.value = true;
    try {
      tree.value = await api.tree();
    } finally {
      loadingTree.value = false;
    }
  }

  async function openNote(path: string): Promise<Note> {
    loadingNote.value = true;
    error.value = null;
    try {
      const loaded = await api.note(path);
      note.value = loaded;
      return loaded;
    } catch (e) {
      note.value = null;
      error.value = (e as Error).message;
      throw e;
    } finally {
      loadingNote.value = false;
    }
  }

  function applySaved(saved: Note) {
    if (note.value && note.value.path === saved.path) {
      note.value = saved;
    }
  }

  function clearNote() {
    note.value = null;
  }

  async function search(query: string): Promise<SearchHit[]> {
    return api.search(query);
  }

  async function createNote(parent: string, name: string): Promise<Note> {
    const path = `${parent ? parent + "/" : ""}${name}.md`;
    const created = await api.saveNote(path, `# ${name}\n`, "*");
    await refreshTree();
    return created;
  }

  async function createFolder(parent: string, name: string): Promise<string> {
    const path = `${parent ? parent + "/" : ""}${name}`;
    await api.createFolder(path);
    await refreshTree();
    return path;
  }

  async function renameNode(node: TreeNode, name: string): Promise<string> {
    const dir = parentOf(node.path);
    const to = `${dir ? dir + "/" : ""}${name}`;
    await api.rename(node.path, to);
    await refreshTree();
    return to;
  }

  async function moveNode(node: TreeNode, targetFolder: string): Promise<string> {
    const dir = targetFolder;
    const name = node.path.includes("/")
      ? node.path.slice(node.path.lastIndexOf("/") + 1)
      : node.path;
    const to = `${dir ? dir + "/" : ""}${name}`;
    if (to === node.path) return to;
    await api.rename(node.path, to);
    await refreshTree();
    return to;
  }

  async function removeNode(node: TreeNode): Promise<void> {
    await api.deleteNode(node.path);
    await refreshTree();
  }

  return {
    tree,
    note,
    loadingTree,
    loadingNote,
    error,
    refreshTree,
    openNote,
    applySaved,
    clearNote,
    search,
    createNote,
    createFolder,
    renameNode,
    moveNode,
    removeNode,
  };
});
