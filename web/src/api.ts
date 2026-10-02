const BASE = "/api/v1";

let unauthorizedHandler: (() => void) | null = null;

/** Registers a callback invoked when an authenticated request returns 401. */
export function setUnauthorizedHandler(handler: () => void) {
  unauthorizedHandler = handler;
}

export type NodeType = "folder" | "note";

export interface User {
  id: string;
  username: string;
  role: "admin" | "member";
  created: string;
  updated: string;
}

export interface AuthState {
  mode: "none" | "multi";
  needsSetup: boolean;
  authenticated: boolean;
  user?: User;
}

export interface TreeNode {
  name: string;
  path: string;
  type: NodeType;
  id?: string;
  title?: string;
  updated?: string;
  size?: number;
  children?: TreeNode[];
}

export interface Note {
  id: string;
  path: string;
  title: string;
  tags: string[] | null;
  icon?: string;
  created: string;
  updated: string;
  size: number;
  version: string;
  public: boolean;
  body: string;
}

export interface PublicNote {
  id: string;
  path: string;
  title: string;
  tags: string[] | null;
  created: string;
  updated: string;
  body: string;
}

export interface SearchHit {
  id: string;
  path: string;
  title: string;
  tags: string[] | null;
  updated: string;
  snippet: string;
}

export interface Asset {
  path: string;
  url: string;
  name: string;
  size: number;
  contentType: string;
}

export interface NoteMeta {
  id: string;
  path: string;
  title: string;
  tags: string[] | null;
  updated: string;
  size: number;
}

export interface WikiLink {
  raw: string;
  target?: NoteMeta;
}

export interface LinksResult {
  outgoing: WikiLink[];
  backlinks: NoteMeta[];
}

export interface Revision {
  id: string;
  path: string;
  savedAt: string;
  size: number;
  version: string;
}

export interface TrashEntry {
  id: string;
  path: string;
  isDir: boolean;
  deletedAt: string;
  size: number;
  original: string;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function parseError(res: Response): Promise<ApiError> {
  let code = "internal";
  let message = res.statusText;
  try {
    const data = (await res.json()) as {
      error?: { code?: string; message?: string };
    };
    if (data.error) {
      code = data.error.code ?? code;
      message = data.error.message ?? message;
    }
  } catch {
    /* ignore */
  }
  return new ApiError(res.status, code, message);
}

async function json<T>(res: Response): Promise<T> {
  if (!res.ok) {
    if (res.status === 401 && !res.url.includes("/auth/")) {
      unauthorizedHandler?.();
    }
    throw await parseError(res);
  }
  return (await res.json()) as T;
}

export const api = {
  async tree(): Promise<TreeNode[]> {
    const res = await fetch(`${BASE}/tree`);
    const data = await json<{ nodes: TreeNode[] }>(res);
    return data.nodes;
  },

  async note(path: string, signal?: AbortSignal): Promise<Note> {
    const res = await fetch(`${BASE}/note?path=${encodeURIComponent(path)}`, { signal });
    return json<Note>(res);
  },

  async saveNote(
    path: string,
    body: string,
    baseVersion: string,
    isPublic = false,
  ): Promise<Note> {
    const res = await fetch(`${BASE}/note`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path, body, baseVersion, public: isPublic }),
    });
    return json<Note>(res);
  },

  async revisions(path: string): Promise<Revision[]> {
    const res = await fetch(`${BASE}/history?path=${encodeURIComponent(path)}`);
    return (await json<{ revisions: Revision[] }>(res)).revisions;
  },

  async revisionContent(path: string, id: string): Promise<string> {
    const res = await fetch(
      `${BASE}/history/revision?path=${encodeURIComponent(path)}&id=${encodeURIComponent(id)}`,
    );
    return (await json<{ content: string }>(res)).content;
  },

  async restoreRevision(path: string, id: string): Promise<Note> {
    const res = await fetch(`${BASE}/history/restore`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path, id }),
    });
    return json<Note>(res);
  },

  async trash(): Promise<TrashEntry[]> {
    const res = await fetch(`${BASE}/trash`);
    return (await json<{ entries: TrashEntry[] }>(res)).entries;
  },

  async restoreTrash(id: string): Promise<void> {
    const res = await fetch(`${BASE}/trash/restore`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id }),
    });
    if (!res.ok) throw await parseError(res);
  },

  async purgeTrash(id: string): Promise<void> {
    const res = await fetch(`${BASE}/trash?id=${encodeURIComponent(id)}`, {
      method: "DELETE",
    });
    if (!res.ok) throw await parseError(res);
  },

  async aiStatus(): Promise<{ enabled: boolean; model?: string }> {
    const res = await fetch(`${BASE}/ai/status`);
    return json<{ enabled: boolean; model?: string }>(res);
  },

  async aiSettings(): Promise<{ baseUrl: string; model: string; hasKey: boolean }> {
    const res = await fetch(`${BASE}/settings/ai`);
    return json<{ baseUrl: string; model: string; hasKey: boolean }>(res);
  },

  async saveAISettings(cfg: {
    baseUrl: string;
    apiKey: string;
    model: string;
  }): Promise<{ baseUrl: string; model: string; hasKey: boolean }> {
    const res = await fetch(`${BASE}/settings/ai`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cfg),
    });
    return json<{ baseUrl: string; model: string; hasKey: boolean }>(res);
  },

  async aiChat(
    mode: "chat" | "organize" | "complete",
    payload: { messages?: { role: string; content: string }[]; content?: string },
  ): Promise<string> {
    const res = await fetch(`${BASE}/ai/chat`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ mode, ...payload }),
    });
    return (await json<{ reply: string }>(res)).reply;
  },

  async publicNotes(): Promise<NoteMeta[]> {
    const res = await fetch(`${BASE}/public/notes`);
    return (await json<{ notes: NoteMeta[] }>(res)).notes;
  },

  async publicNote(path: string): Promise<PublicNote> {
    const res = await fetch(`${BASE}/public/note?path=${encodeURIComponent(path)}`);
    return json<PublicNote>(res);
  },

  async deleteNode(path: string): Promise<void> {
    const res = await fetch(`${BASE}/note?path=${encodeURIComponent(path)}`, {
      method: "DELETE",
    });
    if (!res.ok) throw await parseError(res);
  },

  async createFolder(path: string): Promise<void> {
    const res = await fetch(`${BASE}/folder`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path }),
    });
    if (!res.ok) throw await parseError(res);
  },

  async rename(from: string, to: string): Promise<void> {
    const res = await fetch(`${BASE}/rename`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ from, to }),
    });
    if (!res.ok) throw await parseError(res);
  },

  async importArchive(
    file: File,
  ): Promise<{ notes: number; assets: number; skipped: number }> {
    const res = await fetch(`${BASE}/import`, {
      method: "POST",
      headers: { "Content-Type": "application/zip" },
      body: file,
    });
    return json<{ notes: number; assets: number; skipped: number }>(res);
  },

  exportArchiveURL(): string {
    return `${BASE}/export`;
  },

  async search(q: string): Promise<SearchHit[]> {
    const res = await fetch(`${BASE}/search?q=${encodeURIComponent(q)}`);
    const data = await json<{ results: SearchHit[] }>(res);
    return data.results;
  },

  async links(path: string): Promise<LinksResult> {
    const res = await fetch(`${BASE}/links?path=${encodeURIComponent(path)}`);
    return json<LinksResult>(res);
  },

  async resolve(target: string): Promise<NoteMeta> {
    const res = await fetch(`${BASE}/resolve?target=${encodeURIComponent(target)}`);
    return json<NoteMeta>(res);
  },

  async uploadAsset(file: File): Promise<Asset> {
    const form = new FormData();
    form.append("file", file);
    const res = await fetch(`${BASE}/assets`, { method: "POST", body: form });
    return json<Asset>(res);
  },

  async authState(): Promise<AuthState> {
    const res = await fetch(`${BASE}/auth/state`);
    return json<AuthState>(res);
  },

  async setup(username: string, password: string): Promise<User> {
    const res = await fetch(`${BASE}/auth/setup`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, password }),
    });
    return (await json<{ user: User }>(res)).user;
  },

  async login(username: string, password: string): Promise<User> {
    const res = await fetch(`${BASE}/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, password }),
    });
    return (await json<{ user: User }>(res)).user;
  },

  async logout(): Promise<void> {
    const res = await fetch(`${BASE}/auth/logout`, { method: "POST" });
    if (!res.ok) throw await parseError(res);
  },

  async listUsers(): Promise<User[]> {
    const res = await fetch(`${BASE}/auth/users`);
    return (await json<{ users: User[] }>(res)).users;
  },

  async createUser(username: string, password: string, role: string): Promise<User> {
    const res = await fetch(`${BASE}/auth/users`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, password, role }),
    });
    return json<User>(res);
  },

  async deleteUser(id: string): Promise<void> {
    const res = await fetch(`${BASE}/auth/users/${encodeURIComponent(id)}`, {
      method: "DELETE",
    });
    if (!res.ok) throw await parseError(res);
  },
};
