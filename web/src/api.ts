export interface Channel {
  id: string;
  name: string;
  manufacturer: string;
  model: string;
  civilCode: string;
  address: string;
  addedAt: string;
}
export interface Device extends Channel {
  firmware: string;
  channels: Channel[];
}
export interface Config {
  schemaVersion: string;
  revision: number;
  protocolMode: string;
  device: Device;
  bindingId: string;
  transport: string;
  expires: number;
  heartbeatSeconds: number;
  retrySeconds: number;
  catalogBatchSize: number;
}
export interface Check {
  layer: string;
  status: string;
  detail: string;
}
export interface State {
  registration: string;
  heartbeat: string;
  reason?: string;
  running: boolean;
  generation: number;
  versionCheck: string;
  peerVersion: string;
  heartbeatSent: number;
  heartbeatAcknowledged: number;
  heartbeatMisses: number;
  lastHeartbeat: string;
}
export interface Operation {
  id: string;
  kind: string;
  target: string;
  state: string;
  protocolMode: string;
  compatibility: string[];
  startedAt: string;
  finishedAt: string;
  sipStatus: number;
  business: string;
  sn?: string;
  error?: { code: string; message: string };
  checks: Check[];
  before: State;
  after: State;
  cleanup: string;
  result?: unknown;
}
export interface Snapshot {
  schemaVersion: string;
  at: string;
  config: Config;
  device: State;
  platform: { registered: boolean; online: boolean };
  externalConfigured: boolean;
  operations: Operation[];
  activeOperations: number;
  resources: Record<string, number>;
  capabilities: Check[];
}
export interface Trace {
  sequence: number;
  at: string;
  role: string;
  direction: string;
  transport: string;
  protocolMode: string;
  revision: number;
  compatibility: string[];
  summary: string;
  operationId?: string;
  length: number;
  sha256: string;
  error?: string;
  text: string;
  hex: string;
  redacted: boolean;
  truncated: boolean;
  structure: Record<string, string>;
}
export interface Traces {
  records: Trace[];
  dropped: number;
  lastSequence: number;
  bytes: number;
  limit: number;
  byteLimit: number;
}
export interface Action {
  kind: string;
  target?: string;
  idempotencyKey: string;
  expectedRevision: number;
  timeoutSeconds: number;
  startTime?: string;
  endTime?: string;
}
type Desktop = {
  Snapshot: () => Promise<Snapshot>;
  Execute: (a: Action) => Promise<Operation>;
  SaveConfig: (c: Config) => Promise<Config>;
  Cancel: (id: string) => Promise<Operation>;
  Traces: () => Promise<Traces>;
  Evidence: () => Promise<unknown>;
};
declare global {
  interface Window {
    go?: { transport?: { Desktop?: Desktop } };
  }
}
export const desktop = () => window.go?.transport?.Desktop;
// Memory only: access credentials never enter localStorage, URLs or logs.
let token = "";
export const setToken = (value: string) => {
  token = value;
};
async function request<T>(
  path: string,
  method = "GET",
  body?: unknown,
): Promise<T> {
  const res = await fetch("/api/v1/" + path, {
    method,
    headers: {
      Authorization: "Bearer " + token,
      ...(body ? { "Content-Type": "application/json" } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error?.message ?? "请求失败");
  return data;
}
export const api = {
  snapshot: () => desktop()?.Snapshot() ?? request<Snapshot>("snapshot"),
  traces: () => desktop()?.Traces() ?? request<Traces>("traces"),
  execute: (a: Action) =>
    desktop()?.Execute(a) ?? request<Operation>("actions", "POST", a),
  save: (c: Config) =>
    desktop()?.SaveConfig(c) ?? request<Config>("config", "PUT", c),
  cancel: (id: string) =>
    desktop()?.Cancel(id) ?? request<Operation>("cancel", "POST", { id }),
  evidence: () => desktop()?.Evidence() ?? request<unknown>("evidence"),
};
export function hexRows(hex: string): string {
  return (hex.match(/.{1,32}/g) ?? [])
    .map(
      (row, i) =>
        `${(i * 16).toString(16).padStart(6, "0")}  ${(row.match(/../g) ?? []).join(" ")}`,
    )
    .join("\n");
}
export function status(value: string): string {
  return (
    (
      {
        registered: "已注册",
        unregistered: "未注册",
        registering: "注册中",
        offline: "离线",
        unknown: "未观测",
        waiting: "等待心跳",
        acknowledged: "已确认",
        missed: "未确认",
        completed: "已完成",
        running: "进行中",
        failed: "失败",
        cancelled: "已取消",
        passed: "通过",
        "not-run": "未验证",
        "not-applicable": "不适用",
      } as Record<string, string>
    )[value] ?? value
  );
}
