import React, { useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  api,
  desktop,
  setToken,
  hexRows,
  status,
  type Config,
  type Snapshot,
  type Traces,
  type Operation,
} from "./api";
import "./style.css";

const names: Record<string, string> = {
  register: "注册 / 刷新",
  unregister: "注销",
  keepalive: "Keepalive 心跳",
  deviceinfo: "DeviceInfo 查询",
  catalog: "Catalog 查询",
};
function App() {
  const [connected, connect] = useState(!!desktop()),
    [credential, credentialSet] = useState(""),
    [snapshot, snapshotSet] = useState<Snapshot>(),
    [traces, tracesSet] = useState<Traces>(),
    [error, errorSet] = useState(""),
    [sending, sendingSet] = useState(false),
    [target, targetSet] = useState(""),
    [search, searchSet] = useState(""),
    [tab, tabSet] = useState("报文原文"),
    [selected, select] = useState<number>(),
    [opID, opSelect] = useState(""),
    [panel, panelSet] = useState("工作台"),
    [edit, editSet] = useState<Config>(),
    [showConfig, showConfigSet] = useState(false),
    [timeout, timeoutSet] = useState(10),
    [start, startSet] = useState(""),
    [end, endSet] = useState("");
  const polling = useRef(false),
    alive = useRef(true);
  async function refresh() {
    if (polling.current) return;
    polling.current = true;
    try {
      const [s, t] = await Promise.all([api.snapshot(), api.traces()]);
      if (alive.current) {
        snapshotSet(s);
        tracesSet(t);
        targetSet((v) => v || s.config.device.id);
      }
    } catch (e) {
      if (alive.current) errorSet(String(e));
    } finally {
      polling.current = false;
    }
  }
  useEffect(() => {
    alive.current = true;
    if (!connected) return;
    void refresh();
    const timer = setInterval(refresh, 1000);
    return () => {
      alive.current = false;
      clearInterval(timer);
    };
  }, [connected]);
  async function perform(kind: string) {
    if (!snapshot || sending) return;
    sendingSet(true);
    errorSet("");
    try {
      const op = await api.execute({
        kind,
        target:
          kind === "catalog" || kind === "deviceinfo"
            ? target
            : snapshot.config.device.id,
        idempotencyKey: crypto.randomUUID(),
        expectedRevision: snapshot.config.revision,
        timeoutSeconds: timeout,
        ...(kind === "catalog" ? { startTime: start, endTime: end } : {}),
      });
      opSelect(op.id);
      await refresh();
    } catch (e) {
      errorSet(String(e));
    } finally {
      sendingSet(false);
    }
  }
  async function demo() {
    if (!snapshot || sending) return;
    sendingSet(true);
    errorSet("");
    try {
      for (const kind of [
        "register",
        "keepalive",
        "deviceinfo",
        "catalog",
        "unregister",
      ]) {
        const result = await api.execute({
          kind,
          idempotencyKey: crypto.randomUUID(),
          expectedRevision: snapshot.config.revision,
          timeoutSeconds: 10,
        });
        opSelect(result.id);
        const deadline = Date.now() + 15000;
        let finished = false;
        while (Date.now() < deadline && alive.current) {
          const latest = await api.snapshot();
          snapshotSet(latest);
          const operation = latest.operations.find((o) => o.id === result.id);
          if (operation && operation.state !== "running") {
            if (operation.state !== "completed")
              throw new Error(operation.error?.message ?? "演示操作未完成");
            finished = true;
            break;
          }
          await new Promise((r) => setTimeout(r, 150));
        }
        if (!finished) throw new Error("演示等待超时");
      }
      await refresh();
    } catch (e) {
      errorSet(String(e));
    } finally {
      sendingSet(false);
    }
  }
  async function cancel(id: string) {
    try {
      await api.cancel(id);
      await refresh();
    } catch (e) {
      errorSet(String(e));
    }
  }
  async function save() {
    if (!edit) return;
    sendingSet(true);
    try {
      await api.save(edit);
      showConfigSet(false);
      await refresh();
    } catch (e) {
      errorSet(String(e));
    } finally {
      sendingSet(false);
    }
  }
  async function exportEvidence() {
    try {
      const data = await api.evidence();
      const url = URL.createObjectURL(
        new Blob([JSON.stringify(data, null, 2)], { type: "application/json" }),
      );
      const a = document.createElement("a");
      a.href = url;
      a.download = "gb28181-lab-evidence.json";
      a.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (e) {
      errorSet(String(e));
    }
  }
  const records = (traces?.records ?? []).filter(
      (r) =>
        (!opID || r.operationId === opID) &&
        (!search || r.text.toLowerCase().includes(search.toLowerCase())),
    ),
    record = records.find((r) => r.sequence === selected) ?? records.at(-1),
    op =
      snapshot?.operations.find((o) => o.id === opID) ??
      snapshot?.operations.at(-1),
    cfg = snapshot?.config,
    registered = snapshot?.device.registration === "registered";
  if (!connected)
    return (
      <main className="login">
        <div className="brand">
          GB <span>28181 LAB</span>
        </div>
        <h1>连接本地实验工作台</h1>
        <p>
          从 LAB_DATA_DIR 中的 access.token
          文件读取本机访问令牌。令牌只保留在本页内存中，关闭页面即清除。
        </p>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            setToken(credential);
            credentialSet("");
            connect(true);
          }}
        >
          <label>
            本机访问令牌
            <input
              type="password"
              required
              value={credential}
              onChange={(e) => credentialSet(e.target.value)}
              autoComplete="off"
            />
          </label>
          <button className="primary">连接引擎</button>
        </form>
      </main>
    );
  return (
    <div className="shell">
      <header>
        <div className="brand">
          GB <span>28181 LAB</span>
        </div>
        <span className="space">
          默认实验空间 <small> / M0</small>
        </span>
        <label className="search">
          <span>搜索</span>
          <input
            aria-label="搜索设备或报文"
            placeholder="设备、通道或信令内容"
            value={search}
            onChange={(e) => searchSet(e.target.value)}
          />
        </label>
        <button onClick={() => panelSet("标准与帮助")}>标准与帮助</button>
        <span className="local">本地模式</span>
      </header>
      <nav aria-label="主导航">
        {[
          "工作台",
          "设备与绑定",
          "操作历史",
          "标准与帮助",
          "媒体与录像",
          "安全实验",
          "容量诊断",
        ].map((n) => (
          <button
            key={n}
            className={panel === n ? "active" : ""}
            onClick={() => {
              panelSet(n);
              if (n === "设备与绑定" && cfg) {
                editSet(structuredClone(cfg));
                showConfigSet(true);
              }
            }}
          >
            {n}
          </button>
        ))}
        <div className="navnote">
          GB/T 28181—2022
          <br />
          一个设备 · 最多 32 通道
          <br />
          独立回环 SIP
        </div>
      </nav>
      <main className="workspace">
        <div className="pageheading">
          <div>
            <div className="eyebrow">SIGNALING WORKBENCH</div>
            <h1>{panel}</h1>
            <p>真实协议交互、独立业务关联与可追溯证据</p>
          </div>
          <button onClick={exportEvidence} disabled={!snapshot}>
            导出脱敏证据
          </button>
        </div>
        {error && (
          <div role="alert" className="error">
            {error}
            <button
              onClick={() => {
                errorSet("");
                void refresh();
              }}
            >
              重试连接
            </button>
            {!desktop() && (
              <button onClick={() => connect(false)}>重新认证</button>
            )}
          </div>
        )}
        {!snapshot ? (
          <div role="status" className="card">
            正在读取引擎状态…
          </div>
        ) : (
          <>
            <div className="context">
              <span>
                绑定{" "}
                <b>
                  {cfg?.bindingId === "builtin" ? "内置平台" : "外部测试平台"}
                </b>
              </span>
              <span>
                传输 <b>{cfg?.transport}</b>
              </span>
              <span>
                协议模式{" "}
                <b>
                  {cfg?.protocolMode === "compatibility"
                    ? "兼容模式"
                    : "标准模式"}
                </b>
              </span>
              <span>
                配置 <b>v{cfg?.revision}</b>
              </span>
              <span>
                版本标识 <b>{snapshot.device.peerVersion}</b>
              </span>
              <span>
                媒体 / GB35114 <b>未实现</b>
              </span>
            </div>
            {cfg?.protocolMode === "compatibility" && (
              <div className="warning" role="status">
                {cfg.bindingId === "external"
                  ? "兼容模式：使用 2016 版注册格式，并接受 GB2312 声明的基础查询（COMPAT-001 / 002）。兼容互通与 2022 标准符合性分别记录。"
                  : "兼容模式已选择；内置平台没有需要启用的兼容项，仍使用标准报文。外部绑定将启用 2016 版注册格式。"}
              </div>
            )}
            {["媒体与录像", "安全实验", "容量诊断"].includes(panel) ? (
              <section className="card empty">
                <h2>{panel}尚未实现</h2>
                <p>
                  M0 仅验证 REGISTER、Keepalive、DeviceInfo 与
                  Catalog。此处没有媒体会话、安全认证或容量验收结果。
                </p>
                <button onClick={() => panelSet("工作台")}>
                  返回信令工作台
                </button>
              </section>
            ) : panel === "标准与帮助" ? (
              <section className="card help">
                <h2>本阶段的依据与边界</h2>
                <p>
                  原文定位：
                  {[
                    [23, "注册"],
                    [37, "心跳"],
                    [88, "查询 XML"],
                    [98, "信息应答"],
                    [152, "目录分批"],
                  ].map(([page, label]) => (
                    <a
                      key={page}
                      href={
                        "https://github.com/chialecaleb/gb28181-workspace/blob/main/docs/standards/GBT28181-2022/GBT28181-2022.pdf#page=" +
                        page
                      }
                      target="_blank"
                      rel="noreferrer"
                    >
                      {label} · PDF {page}　
                    </a>
                  ))}
                </p>
                <p>
                  GB/T 28181—2022：9.1 注册、9.6 心跳、9.5 信息与目录查询；附录
                  A XML、附录 E 标识、附录 J 目录、附录 M
                  分批。原页审核范围和用例见仓库 docs/delivery/m0-protocol.md。
                </p>
                <p>
                  REGISTER 使用 RFC 3261 事务与 Digest；信息 / 目录在查询的空
                  SIP 200 后，由新的 MESSAGE 携带业务 Response。GB18030
                  按实际字节计算长度。
                </p>
                <p>
                  标准模式以 GB/T 28181—2022
                  为准。兼容模式仅显式启用已登记的差异；目前外部 REGISTER 可沿用
                  2016 版附录 J 的服务器编码格式，并接受 GB2312 声明的 ASCII
                  查询。两版依据、适用范围及执行记录见仓库
                  docs/compatibility.md。
                  每条操作和报文保留当时的模式与配置版本，切换前须先注销。
                </p>
                <p>
                  查询设备目录包含设备自身及其下级。零通道不代表零条目录；空目录可使用未来时间范围验证。超过传输阈值时使用
                  TCP。
                </p>
                <p>
                  “SIP
                  通过”只说明消息获得确认；业务、状态、版本标识分别判断。Digest
                  不代表 GB35114 通过。
                </p>
                <ol>
                  <li>确认设备、通道和绑定参数。</li>
                  <li>注册后发送心跳或查询，选择记录查看证据。</li>
                  <li>需要改配置时先注销；退出宿主将尝试注销并释放端口。</li>
                </ol>
                <p>
                  HTTP / CLI / 桌面共用单实例引擎，不能同时占用同一运行目录。
                </p>
              </section>
            ) : (
              <>
                <div className="workgrid">
                  <aside className="card tree">
                    <div className="sectionhead">
                      <h2>实验资源</h2>
                      <span>{cfg?.device.channels.length} 通道</span>
                    </div>
                    <div className="platform">
                      ◈{" "}
                      {cfg?.bindingId === "builtin"
                        ? "内置 SIP 平台"
                        : "外部测试绑定"}
                    </div>
                    {cfg &&
                      [cfg.device, ...cfg.device.channels]
                        .filter(
                          (d) => !search || (d.name + d.id).includes(search),
                        )
                        .map((d, i) => (
                          <button
                            className={
                              "node " +
                              (d.id === target ? "selected" : "") +
                              (i ? " channel" : "")
                            }
                            key={d.id}
                            onClick={() => targetSet(d.id)}
                          >
                            <span
                              className={"dot " + (registered ? "online" : "")}
                            />
                            <span>
                              {d.name}
                              <small>{d.id}</small>
                            </span>
                          </button>
                        ))}
                    <p className="hint">
                      圆点表示所属设备注册状态。通道独立状态尚未观测。
                    </p>
                    <button
                      onClick={() => {
                        editSet(structuredClone(snapshot.config));
                        showConfigSet(true);
                      }}
                    >
                      配置设备与绑定
                    </button>
                  </aside>
                  <section className="center">
                    <div className="card operation">
                      <div className="sectionhead">
                        <h2>信令操作</h2>
                        <span className="pill">
                          {status(snapshot.device.registration)}
                        </span>
                      </div>
                      <p className="muted">目标：{target}</p>
                      <div className="actions">
                        <button
                          disabled={
                            sending ||
                            !!snapshot.activeOperations ||
                            cfg?.bindingId !== "builtin"
                          }
                          onClick={() => void demo()}
                        >
                          完整信令演示
                        </button>
                        {Object.entries(names).map(([kind, name]) => (
                          <button
                            key={kind}
                            className={kind === "register" ? "primary" : ""}
                            disabled={
                              sending ||
                              snapshot.operations.some(
                                (o) => o.kind === kind && o.state === "running",
                              ) ||
                              (!["register", "unregister"].includes(kind) &&
                                !registered) ||
                              (["deviceinfo", "catalog"].includes(kind) &&
                                cfg?.bindingId !== "builtin")
                            }
                            onClick={() => void perform(kind)}
                          >
                            {name}
                          </button>
                        ))}
                      </div>
                      <div className="media">
                        <div className="mediaicon">▧</div>
                        <b>媒体尚未实现</b>
                        <span>
                          本阶段只展示真实信令；没有视频播放或媒体质量结果。
                        </span>
                      </div>
                    </div>
                    <div className="card state">
                      <h2>设备状态</h2>
                      <div className="metrics">
                        <div>
                          <small>注册</small>
                          <strong>
                            {status(snapshot.device.registration)}
                          </strong>
                        </div>
                        <div>
                          <small>心跳</small>
                          <strong>{status(snapshot.device.heartbeat)}</strong>
                        </div>
                        <div>
                          <small>心跳确认 / 已发送</small>
                          <strong>
                            {snapshot.device.heartbeatAcknowledged} /{" "}
                            {snapshot.device.heartbeatSent}
                          </strong>
                        </div>
                        <div>
                          <small>版本检查</small>
                          <strong>
                            {status(snapshot.device.versionCheck)}
                          </strong>
                        </div>
                      </div>
                      {snapshot.device.reason && (
                        <p className="error">{snapshot.device.reason}</p>
                      )}
                    </div>
                  </section>
                  <aside className="card parameters">
                    <h2>操作参数与结果</h2>
                    <label>
                      超时（秒）
                      <input
                        type="number"
                        min="1"
                        max="120"
                        value={timeout}
                        onChange={(e) => timeoutSet(Number(e.target.value))}
                      />
                    </label>
                    <label>
                      Catalog 起始时间
                      <input
                        placeholder="可选：2026-10-07T00:00:00"
                        value={start}
                        onChange={(e) => startSet(e.target.value)}
                      />
                    </label>
                    <label>
                      Catalog 结束时间
                      <input
                        placeholder="可选：2026-10-08T00:00:00"
                        value={end}
                        onChange={(e) => endSet(e.target.value)}
                      />
                    </label>
                    <p className="hint">
                      未填时间表示当前目录。设备按确认逐批发送，序号关联独立于
                      SIP 事务。
                    </p>
                    <h3>最近操作</h3>
                    {op ? (
                      <>
                        <b>
                          {names[op.kind]} · {status(op.state)}
                        </b>
                        <small className="mono">{op.id}</small>
                        <p className="hint">
                          {op.protocolMode === "compatibility"
                            ? "兼容模式"
                            : "标准模式"}
                          {op.compatibility.length
                            ? ` · ${op.compatibility.join("、")}`
                            : ""}
                        </p>
                        {op.error && (
                          <p className="error">{op.error.message}</p>
                        )}
                        <p className="hint">{op.cleanup}</p>
                        {op.state === "running" && op.kind !== "unregister" && (
                          <button onClick={() => void cancel(op.id)}>
                            取消操作
                          </button>
                        )}
                      </>
                    ) : (
                      <p>尚无操作</p>
                    )}
                  </aside>
                </div>
                <section className="card evidence">
                  <div className="sectionhead">
                    <h2>
                      协议追踪 <small>{records.length} 条</small>
                    </h2>
                    <select
                      aria-label="按操作筛选"
                      value={opID}
                      onChange={(e) => {
                        opSelect(e.target.value);
                        select(undefined);
                      }}
                    >
                      <option value="">全部操作</option>
                      {snapshot.operations.map((o) => (
                        <option key={o.id} value={o.id}>
                          {names[o.kind]} · {status(o.state)} ·{" "}
                          {o.id.slice(0, 6)}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div className="tabs" role="tablist">
                    {[
                      "报文原文",
                      "结构解析",
                      "十六进制",
                      "会话时序",
                      "分层验证",
                      "状态对比",
                    ].map((t) => (
                      <button
                        role="tab"
                        aria-selected={tab === t}
                        className={tab === t ? "active" : ""}
                        key={t}
                        onClick={() => tabSet(t)}
                      >
                        {t}
                      </button>
                    ))}
                  </div>
                  {traces && traces.dropped > 0 && (
                    <div className="warning">
                      追踪容量已达上限，已淘汰 {traces.dropped}{" "}
                      条；当前导出并非完整会话。
                    </div>
                  )}
                  <div className="tracegrid">
                    <div className="tracelist" aria-label="报文列表">
                      {records.length === 0 ? (
                        <p>没有匹配报文。发送操作后会在这里记录实际收发。</p>
                      ) : (
                        records.map((r) => (
                          <button
                            className={
                              record?.sequence === r.sequence ? "selected" : ""
                            }
                            key={r.sequence}
                            onClick={() => select(r.sequence)}
                          >
                            <small>
                              #{r.sequence}{" "}
                              {new Date(r.at).toLocaleTimeString()} · {r.role}{" "}
                              {r.direction === "out" ? "→" : "←"} {r.transport}
                            </small>
                            <span>{r.summary}</span>
                            <small>
                              {r.length} bytes {r.redacted ? "· 已脱敏" : ""}{" "}
                              {r.error ? "· 拒绝" : ""}
                            </small>
                          </button>
                        ))
                      )}
                    </div>
                    <div className="tracedetail" role="tabpanel">
                      {tab === "分层验证" ? (
                        <Checks op={op} />
                      ) : tab === "状态对比" ? (
                        <pre>
                          {op
                            ? JSON.stringify(
                                {
                                  before: op.before,
                                  after: op.after,
                                  result: op.result,
                                },
                                null,
                                2,
                              )
                            : "尚无操作"}
                        </pre>
                      ) : tab === "会话时序" ? (
                        <ol className="timeline">
                          {records.map((r) => (
                            <li key={r.sequence}>
                              <time>{new Date(r.at).toLocaleTimeString()}</time>{" "}
                              {r.role}{" "}
                              {r.direction === "out" ? "发送 →" : "← 接收"}{" "}
                              <code>{r.summary}</code>
                            </li>
                          ))}
                        </ol>
                      ) : record ? (
                        <>
                          <p className="tracehint">
                            {record.protocolMode === "compatibility"
                              ? "兼容模式"
                              : "标准模式"}{" "}
                            · 配置 v{record.revision}
                            {record.compatibility.length
                              ? ` · ${record.compatibility.join("、")}`
                              : ""}
                            <br />
                            {record.redacted
                              ? "敏感字段已等长掩码；hex 同样脱敏。"
                              : "未发现需掩码字段。"}
                            {record.truncated
                              ? " 原报文过大，仅保留前缀。"
                              : ""}
                            <br />
                            原始字节 SHA-256：<code>{record.sha256}</code>
                          </p>
                          <pre>
                            {tab === "报文原文"
                              ? record.text
                              : tab === "十六进制"
                                ? hexRows(record.hex)
                                : JSON.stringify(record.structure, null, 2)}
                          </pre>
                        </>
                      ) : (
                        <div className="empty">选择一条真实报文查看内容</div>
                      )}
                    </div>
                  </div>
                </section>
                {panel === "操作历史" && (
                  <section className="card history">
                    <h2>操作历史</h2>
                    {[...snapshot.operations].reverse().map((o) => (
                      <button key={o.id} onClick={() => opSelect(o.id)}>
                        {names[o.kind]} · {status(o.state)} ·{" "}
                        {o.protocolMode === "compatibility"
                          ? "兼容模式"
                          : "标准模式"}{" "}
                        · {new Date(o.startedAt).toLocaleString()}
                      </button>
                    ))}
                  </section>
                )}
              </>
            )}
          </>
        )}
      </main>
      <footer>
        <span className={"dot " + (snapshot && !error ? "online" : "")} />
        {snapshot && !error ? "引擎已连接" : "连接待确认"}
        <span>活动操作 {snapshot?.activeOperations ?? "—"}</span>
        <span>
          原文{" "}
          {cfg?.protocolMode === "compatibility"
            ? "GB18030 / 已登记兼容格式"
            : "GB18030"}{" "}
          · 展示 UTF-8
        </span>
        <span>
          更新时间 {snapshot ? new Date(snapshot.at).toLocaleTimeString() : "—"}
        </span>
      </footer>
      {showConfig && edit && (
        <div className="overlay">
          <section
            className="dialog"
            role="dialog"
            aria-modal="true"
            aria-label="设备与绑定配置"
            onKeyDown={(e) => {
              if (e.key === "Escape") {
                showConfigSet(false);
                return;
              }
              if (e.key === "Tab") {
                const nodes = Array.from(
                  e.currentTarget.querySelectorAll<HTMLElement>(
                    "button:not(:disabled),input:not(:disabled),select:not(:disabled)",
                  ),
                );
                const first = nodes[0],
                  last = nodes.at(-1);
                if (e.shiftKey && document.activeElement === first) {
                  e.preventDefault();
                  last?.focus();
                } else if (!e.shiftKey && document.activeElement === last) {
                  e.preventDefault();
                  first?.focus();
                }
              }
            }}
          >
            <div className="sectionhead">
              <h2>设备与绑定配置</h2>
              <button autoFocus onClick={() => showConfigSet(false)}>
                关闭
              </button>
            </div>
            <p>
              先注销并等待操作结束后保存。配置版本 v{edit.revision}，最多 32
              通道。
            </p>
            <label>
              设备名称
              <input
                value={edit.device.name}
                onChange={(e) =>
                  editSet({
                    ...edit,
                    device: { ...edit.device, name: e.target.value },
                  })
                }
              />
            </label>
            <label>
              设备 ID
              <input
                value={edit.device.id}
                onChange={(e) =>
                  editSet({
                    ...edit,
                    device: { ...edit.device, id: e.target.value },
                  })
                }
              />
            </label>
            <div className="formrow">
              <label>
                绑定
                <select
                  value={edit.bindingId}
                  onChange={(e) =>
                    editSet({ ...edit, bindingId: e.target.value })
                  }
                >
                  <option value="builtin">内置平台</option>
                  <option
                    disabled={!snapshot?.externalConfigured}
                    value="external"
                  >
                    外部平台{" "}
                    {snapshot?.externalConfigured
                      ? ""
                      : "（尚未配置已核实参数）"}
                  </option>
                </select>
              </label>
              <label>
                传输
                <select
                  value={edit.transport}
                  onChange={(e) =>
                    editSet({ ...edit, transport: e.target.value })
                  }
                >
                  <option>UDP</option>
                  <option>TCP</option>
                </select>
              </label>
              <label>
                每批目录数
                <input
                  type="number"
                  min="1"
                  max="32"
                  value={edit.catalogBatchSize}
                  onChange={(e) =>
                    editSet({
                      ...edit,
                      catalogBatchSize: Number(e.target.value),
                    })
                  }
                />
              </label>
            </div>
            <label>
              协议模式
              <select
                aria-label="协议模式"
                value={edit.protocolMode}
                onChange={(e) =>
                  editSet({ ...edit, protocolMode: e.target.value })
                }
              >
                <option value="standard">标准模式（GB/T 28181—2022）</option>
                <option value="compatibility">
                  兼容模式（显式启用已登记差异）
                </option>
              </select>
            </label>
            <p className="hint">
              外部绑定的兼容模式使用 2016 版 REGISTER 格式，接受 GB2312 声明的
              ASCII 查询；内置平台不启用这些差异。
              认证、正常心跳、业务应答与清理要求在两种模式下均保留。
            </p>
            <h3>通道</h3>
            {edit.device.channels.map((c, i) => (
              <div className="formrow" key={i}>
                <label>
                  名称
                  <input
                    value={c.name}
                    onChange={(e) =>
                      editSet({
                        ...edit,
                        device: {
                          ...edit.device,
                          channels: edit.device.channels.map((d, j) =>
                            j === i ? { ...d, name: e.target.value } : d,
                          ),
                        },
                      })
                    }
                  />
                </label>
                <label>
                  ID
                  <input
                    value={c.id}
                    onChange={(e) =>
                      editSet({
                        ...edit,
                        device: {
                          ...edit.device,
                          channels: edit.device.channels.map((d, j) =>
                            j === i ? { ...d, id: e.target.value } : d,
                          ),
                        },
                      })
                    }
                  />
                </label>
                <button
                  onClick={() =>
                    editSet({
                      ...edit,
                      device: {
                        ...edit.device,
                        channels: edit.device.channels.filter(
                          (_, j) => j !== i,
                        ),
                      },
                    })
                  }
                >
                  移除
                </button>
              </div>
            ))}
            <button
              disabled={edit.device.channels.length >= 32}
              onClick={() =>
                editSet({
                  ...edit,
                  device: {
                    ...edit.device,
                    channels: [
                      ...edit.device.channels,
                      {
                        id:
                          "3402000000131" +
                          String(edit.device.channels.length + 1).padStart(
                            7,
                            "0",
                          ),
                        name: "实验通道 " + (edit.device.channels.length + 1),
                        manufacturer: "GB28181 Lab",
                        model: "M0-Camera",
                        civilCode: "340200",
                        address: "合成实验点位",
                        addedAt: "",
                      },
                    ],
                  },
                })
              }
            >
              添加通道
            </button>
            <button
              className="primary"
              disabled={
                sending ||
                snapshot?.device.running ||
                !!snapshot?.activeOperations
              }
              onClick={() => void save()}
            >
              保存配置
            </button>
            {error && (
              <p role="alert" className="error">
                {error}
              </p>
            )}
          </section>
        </div>
      )}
    </div>
  );
}
function Checks({ op }: { op?: Operation }) {
  return (
    <div className="checks">
      {op ? (
        op.checks.map((c) => (
          <div key={c.layer}>
            <b>{c.layer}</b>
            <span className={"check " + c.status}>{status(c.status)}</span>
            <p>{c.detail}</p>
          </div>
        ))
      ) : (
        <p>运行操作后显示分层结果。未观测不等于通过。</p>
      )}
    </div>
  );
}
createRoot(document.getElementById("root")!).render(<App />);
