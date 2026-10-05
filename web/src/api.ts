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
  email?: string;
  role: "admin" | "member";
  status: "active" | "invited";
  emailVerified: boolean;
  created: string;
  updated: string;
}

export interface APIToken {
  id: string;
  name: string;
  prefix: string;
  created: string;
  lastUsed?: string;
  expires?: string;
}

export interface AuthState {
  mode: "none" | "multi";
  needsSetup: boolean;
  authenticated: boolean;
  mailEnabled: boolean;
  registrationEnabled: boolean;
  loginHint: string;
  loginIcp: string;
  loginLink?: { text: string; url: string };
  user?: User;
}

export interface SiteSettings {
  registrationEnabled: boolean;
  loginHint: string;
  loginIcp: string;
  loginLinkUrl: string;
  loginLinkText: string;
}

export interface RegisterResult {
  user: User;
  verificationRequired: boolean;
}

export interface MailSettings {
  host: string;
  port: number;
  username: string;
  from: string;
  starttls: boolean;
  hasPassword: boolean;
  enabled: boolean;
}

export interface MailSettingsInput {
  host: string;
  port: number;
  username: string;
  password: string;
  from: string;
  starttls: boolean;
}

export interface StorageSettings {
  endpoint: string;
  region: string;
  accessKey: string;
  bucket: string;
  useSSL: boolean;
  publicURL: string;
  hasSecret: boolean;
  enabled: boolean;
}

export interface StorageSettingsInput {
  endpoint: string;
  region: string;
  accessKey: string;
  secretKey: string;
  bucket: string;
  useSSL: boolean;
  publicURL: string;
}

export interface StorageTestResult {
  ok: boolean;
  message: string;
}

export interface DesktopSettings {
  autostart: boolean;
  dataDir: string;
  version: string;
}

export interface UpdateInfo {
  current: string;
  latest: string;
  hasUpdate: boolean;
  url: string;
}

export type SyncDirection = "both" | "pull" | "push";
export type SyncState = "idle" | "syncing" | "error" | "paused";

export interface SyncProgress {
  done: number;
  total: number;
}

export interface SyncConflict {
  id: string;
  originalPath: string;
  conflictPath: string;
  localVersion: string;
  remoteVersion: string;
  detectedAt: string;
}

export interface SyncStatus {
  enabled: boolean;
  serverURL: string;
  vaultId: string;
  direction: SyncDirection;
  intervalSec: number;
  lastSyncAt: string;
  status: SyncState;
  progress: SyncProgress;
  conflicts: SyncConflict[];
  lastError: string;
  connected: boolean;
}

export interface SyncConfigInput {
  serverURL: string;
  token?: string;
  enabled: boolean;
  direction: SyncDirection;
  intervalSec: number;
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

export interface CapturePreview {
  title: string;
  text: string;
  url: string;
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

export interface AIStatus {
  enabled: boolean;
  model?: string;
  toolCalling: boolean | "unknown";
  agentEnabled: boolean;
  maxSteps: number;
  confirmPolicy: string;
}

export type AgentRisk = "read" | "write" | "dangerous";
export type AgentStepStatus = "ok" | "error" | "denied" | "rejected";
export type AgentRunStatus = "done" | "needs_confirmation" | "max_steps";

export interface AgentStep {
  toolCallId: string;
  name: string;
  args: unknown;
  risk: AgentRisk | string;
  status: AgentStepStatus | string;
  summary: string;
}

export interface AgentPending {
  toolCallId: string;
  name: string;
  args: unknown;
  preview: string;
  risk: AgentRisk | string;
}

export interface AgentResult {
  runId: string;
  status: AgentRunStatus | string;
  text: string;
  steps: AgentStep[];
  pending?: AgentPending;
}

export interface AgentConfirmBody {
  runId: string;
  toolCallId: string;
  decision: "approve" | "reject";
  args?: unknown;
}

// Maps the machine-readable error codes the agent endpoints return to i18n
// message keys, so callers can surface a readable reason instead of the raw
// provider text.
export function aiErrorKey(e: unknown): string {
  if (e instanceof ApiError) {
    switch (e.code) {
      case "agent_disabled":
        return "agent.disabled";
      case "tool_calling_unsupported":
        return "agent.unsupported";
      case "run_not_found":
        return "agent.runNotFound";
      case "input is required":
        return "agent.inputRequired";
    }
    if (e.status === 429) return "agent.rateLimited";
    if (e.status === 503) return "ai.notConfiguredHint";
  }
  return "agent.error";
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

  async aiStatus(): Promise<AIStatus> {
    const res = await fetch(`${BASE}/ai/status`);
    const raw = await json<{
      enabled: boolean;
      model?: string;
      toolCalling?: boolean | string;
      agentEnabled?: boolean;
      maxSteps?: number;
      confirmPolicy?: string;
    }>(res);
    const toolCalling: boolean | "unknown" =
      raw.toolCalling === true || raw.toolCalling === "true"
        ? true
        : raw.toolCalling === false || raw.toolCalling === "false"
          ? false
          : "unknown";
    return {
      enabled: raw.enabled,
      model: raw.model,
      toolCalling,
      agentEnabled: raw.agentEnabled === true,
      maxSteps: raw.maxSteps ?? 0,
      confirmPolicy: raw.confirmPolicy ?? "dangerous",
    };
  },

  async aiAgent(body: {
    runId?: string;
    input: string;
    context?: { path?: string; selection?: string };
  }): Promise<AgentResult> {
    const res = await fetch(`${BASE}/ai/agent`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    return json<AgentResult>(res);
  },

  async aiAgentConfirm(body: AgentConfirmBody): Promise<AgentResult> {
    const res = await fetch(`${BASE}/ai/agent/confirm`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    return json<AgentResult>(res);
  },

  async aiAgentStop(body: { runId: string }): Promise<{ runId: string; status: string }> {
    const res = await fetch(`${BASE}/ai/agent/stop`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    return json<{ runId: string; status: string }>(res);
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

  async mailSettings(): Promise<MailSettings> {
    const res = await fetch(`${BASE}/settings/mail`);
    return json<MailSettings>(res);
  },

  async saveMailSettings(cfg: MailSettingsInput): Promise<MailSettings> {
    const res = await fetch(`${BASE}/settings/mail`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cfg),
    });
    return json<MailSettings>(res);
  },

  async storageSettings(): Promise<StorageSettings> {
    const res = await fetch(`${BASE}/settings/storage`);
    return json<StorageSettings>(res);
  },

  async saveStorageSettings(cfg: StorageSettingsInput): Promise<StorageSettings> {
    const res = await fetch(`${BASE}/settings/storage`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cfg),
    });
    return json<StorageSettings>(res);
  },

  async testStorage(cfg?: StorageSettingsInput): Promise<StorageTestResult> {
    const res = await fetch(`${BASE}/settings/storage/test`, {
      method: "POST",
      ...(cfg
        ? {
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(cfg),
          }
        : {}),
    });
    return json<StorageTestResult>(res);
  },

  async desktopSettings(): Promise<DesktopSettings> {
    const res = await fetch(`${BASE}/desktop/settings`);
    return json<DesktopSettings>(res);
  },

  async saveDesktopSettings(cfg: { autostart: boolean }): Promise<DesktopSettings> {
    const res = await fetch(`${BASE}/desktop/settings`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cfg),
    });
    return json<DesktopSettings>(res);
  },

  async checkUpdate(): Promise<UpdateInfo> {
    const res = await fetch(`${BASE}/desktop/update`);
    return json<UpdateInfo>(res);
  },

  async syncStatus(): Promise<SyncStatus> {
    const res = await fetch(`${BASE}/desktop/sync`);
    return json<SyncStatus>(res);
  },

  async saveSync(cfg: SyncConfigInput): Promise<SyncStatus> {
    const res = await fetch(`${BASE}/desktop/sync`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cfg),
    });
    return json<SyncStatus>(res);
  },

  async runSync(): Promise<{ started: boolean }> {
    const res = await fetch(`${BASE}/desktop/sync/run`, { method: "POST" });
    return json<{ started: boolean }>(res);
  },

  async syncConflicts(): Promise<SyncConflict[]> {
    const res = await fetch(`${BASE}/desktop/sync/conflicts`);
    const data = await json<{ conflicts?: SyncConflict[] } | SyncConflict[]>(res);
    return Array.isArray(data) ? data : (data.conflicts ?? []);
  },

  async resolveSyncConflict(id: string, keep: "local" | "remote"): Promise<void> {
    const res = await fetch(`${BASE}/desktop/sync/conflicts/resolve`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id, keep }),
    });
    if (!res.ok) throw await parseError(res);
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

  async capturePreview(url: string): Promise<CapturePreview> {
    const res = await fetch(`${BASE}/capture/preview`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ url }),
    });
    return json<CapturePreview>(res);
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

  async inviteUser(email: string, role: string): Promise<User> {
    const res = await fetch(`${BASE}/auth/users/invite`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, role }),
    });
    return (await json<{ user: User }>(res)).user;
  },

  async resendInvite(id: string): Promise<void> {
    const res = await fetch(
      `${BASE}/auth/users/${encodeURIComponent(id)}/resend-invite`,
      { method: "POST" },
    );
    if (!res.ok) throw await parseError(res);
  },

  async setUserEmail(
    id: string,
    email: string,
  ): Promise<{ user: User; verificationSent: boolean }> {
    const res = await fetch(`${BASE}/auth/users/${encodeURIComponent(id)}/email`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email }),
    });
    return json<{ user: User; verificationSent: boolean }>(res);
  },

  async sendVerification(id: string): Promise<void> {
    const res = await fetch(
      `${BASE}/auth/users/${encodeURIComponent(id)}/send-verification`,
      { method: "POST" },
    );
    if (!res.ok) throw await parseError(res);
  },

  async acceptInvite(token: string, password: string): Promise<User> {
    const res = await fetch(`${BASE}/auth/accept-invite`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token, password }),
    });
    return (await json<{ user: User }>(res)).user;
  },

  async requestPasswordReset(email: string): Promise<void> {
    const res = await fetch(`${BASE}/auth/password/request`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email }),
    });
    if (!res.ok) throw await parseError(res);
  },

  async resetPassword(token: string, password: string): Promise<void> {
    const res = await fetch(`${BASE}/auth/password/reset`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token, password }),
    });
    if (!res.ok) throw await parseError(res);
  },

  async verifyEmail(token: string): Promise<void> {
    const res = await fetch(`${BASE}/auth/verify-email`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token }),
    });
    if (!res.ok) throw await parseError(res);
  },

  async register(email: string, password: string): Promise<RegisterResult> {
    const res = await fetch(`${BASE}/auth/register`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
    });
    return json<RegisterResult>(res);
  },

  async resendVerification(email: string): Promise<void> {
    const res = await fetch(`${BASE}/auth/verify/resend`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email }),
    });
    if (!res.ok) throw await parseError(res);
  },

  async siteSettings(): Promise<SiteSettings> {
    const res = await fetch(`${BASE}/settings/site`);
    return json<SiteSettings>(res);
  },

  async saveSiteSettings(cfg: SiteSettings): Promise<SiteSettings> {
    const res = await fetch(`${BASE}/settings/site`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cfg),
    });
    return json<SiteSettings>(res);
  },

  async listTokens(): Promise<APIToken[]> {
    const res = await fetch(`${BASE}/auth/tokens`);
    return (await json<{ tokens: APIToken[] }>(res)).tokens;
  },

  async createToken(name: string, expiresDays = 0): Promise<{ token: APIToken; secret: string }> {
    const res = await fetch(`${BASE}/auth/tokens`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, expiresDays }),
    });
    return json<{ token: APIToken; secret: string }>(res);
  },

  async deleteToken(id: string): Promise<void> {
    const res = await fetch(`${BASE}/auth/tokens/${encodeURIComponent(id)}`, {
      method: "DELETE",
    });
    if (!res.ok) throw await parseError(res);
  },
};
