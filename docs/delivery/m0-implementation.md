# M0 实现与运行合同

更新日期：2026-10-11。范围正本仍为 [M0 计划](m0-plan.md)；实际验证、UDP/TCP 兼容外测、历史失败与自审见[实施证据](../evidence/m0-implementation.md)。本轮 M0 范围已完成；这是已存在工程的运行说明，不代表完整 GB28181、GB35114 或容量验收。

## 构建与启动

本轮锁定 Go 1.27.0、Node 24.19.0、pnpm 12.8.1；依赖版本见根 go.mod/go.sum、web/package.json/pnpm-lock.yaml。Windows 需要 WebView2 Runtime；Linux HTTP/CLI 不依赖 Wails、GUI 或 DISPLAY。

在仓库根执行：

```powershell
pnpm --dir web install --frozen-lockfile
pnpm --dir web build
go build -o bin/gb28181-lab-server.exe ./cmd/gb28181-lab-server
go build -o bin/gb28181-lab.exe ./cmd/gb28181-lab
go build -tags production -ldflags "-H=windowsgui" -o bin/gb28181-lab-desktop.exe ./cmd/gb28181-lab-desktop
```

Linux 构建前两条 Go 命令去掉输出文件的 `.exe`。静态产物由 web/assets.go 嵌入，因此必须先构建 web；dist 为构建输出，不提交。Windows 桌面已实际启动并发送信令；尚未制作安装程序、签名或无开发环境离线发行包。

任选一个宿主：

```powershell
./bin/gb28181-lab-desktop.exe
./bin/gb28181-lab-server.exe
./bin/gb28181-lab.exe demo
```

HTTP 默认地址为 `http://127.0.0.1:18081`；在页面输入 LAB_DATA_DIR 下 access.token 的内容。程序不输出令牌；前端只在内存中保存。CLI 的 `snapshot` 输出配置/状态，`demo` 顺序执行注册、心跳、信息、目录、注销，输出 JSON 行，失败非零退出。正常关闭宿主时进行有截止时间的注销与资源回收。操作系统强制终止无法保证完成网络注销，重启保持未注册。

三个宿主不能同时拥有同一 LAB_DATA_DIR；使用其他目录也必须选择不冲突的监听端口。默认一个 NVR 和一个摄像机通道，目录包含自身及下级；无通道的 NVR 仍有一条目录记录。未来时间过滤可产生空目录。

## 配置和持久化

配置优先级：内置默认值 → `.env` → `.env.local` → 白名单进程环境变量。解析字面值，不执行 shell、不展开变量、不解释行内 `#`；引号去掉一层，重复/未知键和显式空必填值失败。TEST_* 可被配置文件校验识别，但不返回给应用运行时。详见[本地配置](../dev-rules/local-test-environment.md)。

产品的外部绑定只读 LAB_DATA_DIR 下可选的 `binding.external.json`，字段为 platformId、domain、target、username、password、authMode。target 必须是授权回环端点，authMode 目前只接受 `digest`；参数需另行核实，不从 Web 登录密码推导。文件不进入 Git、API 或前端，错误不回显内容。工作台只显示是否存在可选绑定，并在注销后切换。

SQLite schema 1 保存设备、通道、绑定选择、传输、时钟和 protocolMode 配置。protocolMode 只接受 standard / compatibility，新增和缺字段的旧配置均默认 standard；不增加表结构迁移。revision 比较交换拒绝过期修改；迁移前备份已有数据库，失败回滚，较新版本拒绝打开。OS 文件锁提供单实例所有权。运行目录在 Unix 为 0700，Windows 使用当前用户、SYSTEM、Administrators 的受保护 DACL。认证秘密单独放本地文件；在途 SIP、nonce、操作和在线状态不恢复。

在工作台“配置设备与绑定 → 协议模式”选择并保存。必须先注销、结束操作和响应；切换模式不重写旧证据。standard 为 GB/T 28181-2022 业务基线；compatibility 仅在外部绑定启用已登记的 COMPAT-001/002，内置平台保持标准行为。具体字段、原文对照、失败与回退方式见[兼容文档](../compatibility.md)。

## 应用与宿主边界

internal/app 是唯一状态和操作服务；internal/transport 仅转换 HTTP/Wails 输入输出；CLI 直接调用同一服务。设备和内置 registrar 使用独立真实 UDP/TCP socket，业务不得绕过 SIP 直接调用设备完成查询。

| HTTP 方法/路径 | 输入与结果 |
| --- | --- |
| GET `/api/v1/snapshot` | schemaVersion 1.0，配置版本、设备/平台状态、有界操作历史和资源计数 |
| GET `/api/v1/traces` | 最多 1000 条脱敏记录，原始长度/SHA-256、展示原文/hex、结构、淘汰数量、当时模式/配置版本和兼容项 |
| GET `/api/v1/evidence` | 脱敏快照、操作、规则版本、trace 和结论范围 |
| PUT `/api/v1/config` | 完整 Config DTO，携带当前 revision；已注册/有操作时拒绝 |
| POST `/api/v1/actions` | kind、target、idempotencyKey、expectedRevision、timeoutSeconds；Catalog 可带 startTime/endTime |
| POST `/api/v1/cancel` | id；取消在途操作，注销本身作为清理动作不再取消 |

kind 为 register/unregister/keepalive/deviceinfo/catalog。操作先返回 operationId，再从 snapshot 查询状态；固定 protocolMode、revision 及兼容项，SIP 状态、独立业务应答、状态变化、媒体、安全分层。超时 1—120 秒；幂等键 1—128 字符；相同键不同输入返回 conflict。HTTP 错误为 `error: {code,message,fieldPath?,retryable}`，非法 JSON/未知字段/尾随值失败，正文限制 128 KiB。Wails 仅绑定窄 Desktop 适配，绝不暴露 Service.LocalToken、TraceLocal 或原始凭据。

HTTP 只接受回环来源、精确 Host、同源 Origin、Bearer 令牌；跨站请求拒绝，设置 CSP、nosniff、no-store。M0 没有远程 HTTPS、多用户、SSO、服务账号和 MCP 接口。未来 MCP 使用已研究的 Go SDK 1.8.0 / 2026-07-28 协议，通过同一权限/任务契约接入 stdio 或 Streamable HTTP，不另建成功状态。

## 协议和资源策略

规则版本为 gb2022-m0-2，资产为 protocol-assets/m0-contract.json，独立 Python 向量为 protocol-assets/m0-vectors.json。标准模式使用 GB18030、最终 Content-Length、无 userinfo 的 REGISTER 路由 URI；Digest、X-GB-Ver、Keepalive MESSAGE Notify、查询/独立 Response 均有实现和测试。GB 身份域、认证 realm 与实际路由主机分离。兼容模式保留特定旧版字段形式，SIP 成功与对应 RFC/版本差异分别展示。

| 资源 | M0 上限/清理 |
| --- | --- |
| 设备/通道 | 一个设备，最多 32 通道；内置/外部择一绑定 |
| 操作 | 最多 16 在途且每种动作至多一个；历史 128，幂等映射随历史淘汰 |
| SIP | 每端点 16 客户事务、128 入站事务预算；UDP 重传由 sipgo 处理 |
| TCP | 每端点最多 8 接入连接/8 分帧缓存；空闲读取 185 秒，连接/写入有截止时间 |
| XML/SIP | 64 KiB 头、16 MiB 帧；XML 深度 12、元素 300000、单文本 64 KiB；属于产品预算 |
| Catalog | 每条标准上限 10000；隔离夹具总数 20000；实际设备批次 1—32，逐批等待确认，超 1300 字节切 TCP |
| trace | 1000 条且 8 MiB 原始前缀；单条最多存 256 KiB，完整原文长度/SHA 单列，淘汰/截断明示 |

SIP 响应进入 sipgo 前校验当前 branch、CSeq、方法、Call-ID、身份与来源，避免错误响应提前结束事务。取消/注销递增代次，迟到业务不能复活旧状态。失败注册尝试补偿注销；对端不确认时明确记录，不能声称远端已清理。默认注册 86400 秒、心跳 60 秒、三次超时、失败重试不短于 60 秒；单元测试可控时钟不改变外测时钟。

TCP 最终响应到达后立即关闭的场景采用 sipgo 默认事务生命周期，避免实验性断链终止丢失已收到的最终响应。无响应仍由调用截止时间和宿主关闭回收，设备状态机独立处理断链离线/重注册。此修复适用于两种模式；具体回归见 F-14。

M0 仅实现已审核 XML 子集；未知 CmdType/过滤字段、折叠头、重复单值头、未支持的可选目录字段拒绝，不宣称完整标准解析器。媒体、PTZ、Date/Note、GB35114、级联、安全角色与容量均未实现。完整后续功能叶节点见[能力清单](capability-leaves.json)，不因切片交付删除。

## 验证与交接

```powershell
python scripts/verify.py
python scripts/verify_app.py --e2e
```

首次 E2E 需 `pnpm --dir web exec playwright install chromium`。Linux 有 C 编译器时运行 `python scripts/verify_app.py --race --e2e`；本机 Windows 无 C 编译器，已用独立 WSL Go/Zig 工具链执行 `go test -race ./internal/...`，不声称 Windows race 已运行。CI 执行同一应用门禁，2026-10-13 集成的远端结果以 M0 PR 对应提交的 repository-quality 检查为准；本地复验见[实施证据](../evidence/m0-implementation.md)。

P6 工具为 `python scripts/test_livegbs.py --run --transport UDP --mode standard`，TCP 单独运行；显式兼容测试使用 `--mode compatibility`，省略 mode 仍为 standard。仅在独立门禁通过且有外测授权后运行。工具优先使用 TEST_LIVEGBS_SIP_PASSWORD，缺少该键时才允许读取已授权发现值，显式空值或掩码值直接失败；发现的目标必须与授权配置一致。

工具先证明合成 ID 不存在，运行认证注册/刷新、四信令与三个正常心跳周期，再注销并只移除本次已确认离线设备。Python 独立检查原始 SIP 长度/哈希、事务、SN/身份、业务结果及心跳间隔，HTTP 成功不能单独通过；异常同样执行归属核查和清理。返回 0 为完整测试通过、1 为互通或应用验证失败、2 为环境/参数受阻。原始外测资料只在 artifacts，公开摘要见兼容台账；不能把该工具接入产品启动或 CI。

本轮 UDP/TCP 的兼容外测完成，标准模式对照仍保留 400，不能称为严格 2022 对端验证。后续在获授权后进入 M1；完整安全叶节点及未运行的发行/性能验收继续保留，当前结果见[状态](status.md)。
