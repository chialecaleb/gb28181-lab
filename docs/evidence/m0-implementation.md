# M0 实施、自审与验证记录

首次实施日期：2026-10-07；实施收尾日期：2026-10-11；历史整理与集成日期：2026-10-13。原准备基线 codex/m0-preparation / 163d15a、原交付 039b013 及后继协作规范阶段的描述保留历史含义。按 A-019，准备、实施和协作规范成果统一归入本 M0 交付提交；版本身份以包含本报告的提交及各次外测源码指纹为准。2026-10-07/11 的本地交付记录不代表后来未获远端授权；本次集成范围与新验证见末节。

## 结论范围

**M0 P0—P7 按本轮用户确认范围完成，UDP/TCP 显式兼容外测 passed。** Go/React、Wails 薄宿主、Linux HTTP/CLI、SQLite、四信令工作台和两种模式均有实现。独立向量、回环网络、应用故障、跨入口访问、浏览器操作、Linux race 和受控第三方四信令检查通过。

默认 standard 以 GB/T 28181-2022 为准；compatibility 仅覆盖 COMPAT-001/002。第三方仍拒绝标准模式 REGISTER，缺失 X-GB-Ver 的版本检查仍 failed，兼容成功不等于该对端通过严格 2022 符合性。2026-10-07 的外部注册失败、2026-10-11 的两次兼容失败和全部清理均保留。本轮由同一 Agent 规划、实施、自检、收尾自审；独立 Agent 审查 not-run。不声明全标准、GB35114、安全媒体、容量或用户主观体验验收通过。

## 独立依据

原始 GB28181-2022 PDF SHA-256 为 `727dfbd2d1094780d2044800de7c59743550088d871a717c7bb4d36f33710cc6`。审核 PDF 页 18、19、22—24、35—38、63—69、87、88、92、97、98、120—123、145、146、152；印刷页为 PDF 页减 7。OCR 仅查找，编码/字段与附录 J/M 结合原页判断。

RFC 3261 的 8/10/17/18/20.14/22.4、RFC 2617 Digest、RFC 3428 MESSAGE、RFC 3581 rport 为辅助依据。Python 独立构造 XML/GB18030 字节和 Digest expected；Go 待测编码器不生成自己的 expected。含中文与 U+20000 四字节字符、空 Catalog、RFC 2617 Mufasa 摘要。规则资产、实现、抓取证据分别保存。

2026-10-11 先查两版 OCR，再视觉核对 2022 PDF 页 7、16—18、23—24 与 2016 PDF 页 120、122：2016 规范性 J.1/J.3 明确示范带服务器编码的注册/注销 URI，2022 前言删除旧附录 J。项目按 2022 4.3.2 的引用优先级、6.1.2 的 From/To 适用字段及 9.1/RFC3261 10.2 建立标准模式，旧版形式作为显式兼容项。2016 6.10“宜采用 GB2312”与 2022 6.10“应采用 GB18030”的 OCR 对照另记 COMPAT-002。原文事实、解释限制和资料哈希见[兼容文档](../compatibility.md)，不把旧版附录当成 2022 现行条款。

## 2026-10-07 基础检查记录

| 检查 | 结果与范围 |
| --- | --- |
| `python scripts/local_env.py` | 配置结构通过；不据此宣称 SIP 成功 |
| `pnpm --dir web install --frozen-lockfile` | 锁文件安装通过，未把 TEST_* 注入 Vite |
| `pnpm --dir web build` | TypeScript 与 Vite 生产构建通过 |
| `pnpm --dir web test` | 展示字节偏移与未知状态检查通过 |
| `go mod verify` | 下载模块校验通过 |
| `go test ./internal/... -timeout 180s` | 编码、摘要、状态、存储、网络、应用、适配器通过；外测用例默认跳过 |
| `go vet ./internal/... ./cmd/... ./web` | Windows 静态检查通过 |
| `go test -race ./internal/... -timeout 180s` | WSL/Linux Go 1.27.0、隔离 Zig 0.17.0 C 编译器；修复响应前置校验后复验 passed |
| Windows 三入口 `go build` | HTTP、CLI、production Wails 构建通过；桌面使用 WebView2 |
| Linux HTTP/CLI build 与 CLI demo | 无 DISPLAY、无 Wails 链接的宿主和实际信令演示通过 |
| `pnpm --dir web e2e` | Playwright 1280×800，真实注册/心跳/信息/目录/注销、中文配置、锁定、键盘提交、原文/hex、分层结果、导出、一键完整演示与 Escape 关闭通过 |
| `python scripts/verify.py` | 结构/文档/工具门禁在暂存最终文件后复验，结论不代替协议测试 |

实际截图在被忽略的 `artifacts/m0/ui/workbench-1280.png`；浏览器测试下载的脱敏证据在同目录。Windows 桌面通过本机 UI 实际点击观察；自动截图不等于用户主观验收。未运行无开发环境安装、签名、升级、24h 稳态、万设备或任何媒体测试。

## 2026-10-11 收尾检查

| 检查与命令 | 实际结果、源码范围与证据 |
| --- | --- |
| 本地配置与隐私 | 按用户授权仅更新本地设备接入密码，其他解析值保持不变；指定测试配置只读核实。真实值、路径、原始输入及抓包未纳入 Git；只输出状态和脱敏字段 |
| `python scripts/verify_app.py --e2e` | passed：冻结 pnpm、Prettier、Vitest、TypeScript/Vite、Go 模块校验、内部测试/vet、Windows HTTP/CLI/production Wails 构建、真实浏览器 E2E。日志 artifacts/interoperability-validation/app-final-check.log |
| `go test ./internal/sip ./internal/app ./internal/gb2022 ./internal/evidence -count=1 -timeout 120s` | passed：模式/路由/编码/证据以及 TCP 修复后定向复验；core-after-tcp-fix.log |
| `python -m unittest discover -s scripts -p test_interop_runner.py` | passed：14 个合成回归；密码优先级、发现值一致性、归属清理/异常、独立业务及原字节判定 |
| Linux `go test -race ./internal/... -timeout 180s`，vet、HTTP/CLI build、CLI demo | passed：隔离 WSL Go 1.27.0 / Zig 0.17.0，最终模式和 TCP 修复代码；linux-final-check.log。未改全局工具链，不写成 Windows race |
| P6 standard，UDP/TCP 分别运行 | failed（对端互通）：首 REGISTER 400，无回退；补偿仍拒绝，列表确认设备不存在。失败与版本差异保留，不阻塞用户已确认的显式兼容 P6 范围 |
| P6 compatibility，UDP/TCP 分别运行 | passed：COMPAT-RUN-003/005，应用退出 0、独立原始字节检查和清理均通过。各 183.7 秒、13 个独立事务；详情见下节及兼容台账 |
| Windows 原生桌面 | 本轮 production 构建 passed；真实 REGISTER/Catalog/关闭操作复用 2026-10-07、039b013 的既有证据。宿主入口/绑定未改，共享应用与 UI 的变化由上面门禁复验。当前原生自动化补测未形成操作通过：WebView2 阻止环境参数调试，UIAutomation 未找到控件；这些检查器尝试为 failed，未计入新通过数 |
| `python scripts/verify.py` | passed：最终暂存后的登记/链接、仓库边界、26 个工具回归、Python 语法及工作区/暂存空白检查；repository-final-check.log。不代替协议验收 |

本轮新增模式截图位于 artifacts/interoperability-validation/workbench-compatibility.png，完整工作台截图由本轮 E2E 更新。原生自动化私有记录在同目录的 native 子目录；停止的均为本次持有进程，没有操作已有窗口/设备。未改 Wails 宿主以开放调试或为自动化关闭产品约束。

外测源码指纹按 internal/**/*.go、scripts/*.py、go.mod/go.sum 的相对路径与字节计算。UDP COMPAT-RUN-003 在最终 TCP 修复前通过；其后仅新增 TCP 最终响应/关闭修复及无交换证据的显示修正，独立原字节和既有 UDP/混合传输断言均保持有效，相关核心及完整门禁已复验。TCP COMPAT-RUN-005 使用指纹 `94bf29f9888a8c7da8707f7f47b0f033e7b91e6b95183879d9df2055f8d1f3d3`。文档整理与提交重写不冒充重新外测。

## M0 用例执行映射

| ID | 已执行范围与结果 | 实现/测试 |
| --- | --- | --- |
| M0-T-01 | passed：真实 401/认证注册，独立 MD5/qop/legacy、错误密码/realm/URI、重放与过期 nonce | internal/sip/digest_test.go；internal/app/service_test.go |
| M0-T-02 | passed：真实刷新/注销、403/3xx 拒绝、取消补偿；可控时钟到期及旧代次拒绝 | internal/app/service_test.go；internal/device/state_test.go |
| M0-T-03 | passed：首包无确认后重传字节不变、重复请求一次业务副作用、错误/乱序 CSeq 响应不结束正确事务 | internal/sip/endpoint_test.go |
| M0-T-04 | passed：所有字节分割点、真实分片/粘包、非法长度/重复长度/截断连接、断链离线与重注册、最终响应后立即断开不丢失结果、无响应有界失败、端口回收 | internal/sip/frame_test.go、endpoint_test.go；internal/app/service_test.go |
| M0-T-05 | passed：内部真实心跳、连续三次响应超时及重注册恢复、虚拟时钟三接收周期离线；外部 UDP/TCP 各三个正常周期确认 | internal/app/service_test.go、external_test.go；internal/platform/registrar_test.go；scripts/interop_evidence.py |
| M0-T-06 | passed：中文/四字节名称、新 MESSAGE 应答、SIP 200 无业务应答超时，错 SN/目标和未绑定来源拒绝 | internal/app/correlation_test.go、service_test.go；web/e2e/workbench.spec.ts |
| M0-T-07 | passed：零通道仍有自身、时间过滤空结果、单/多条、强制分批、Num 错误、重复/不完整聚合、查询取消/超时 | internal/gb2022/xml_test.go；internal/device/state_test.go；internal/app/service_test.go |
| M0-T-08 | passed：隔离 10000/10001 条分批，真实大目录响应改用 TCP；未知字段拒绝 | internal/gb2022/xml_test.go；internal/app/service_test.go |
| M0-T-09 | passed：独立编码、错误声明/非法编码/实体/根/重复字段、CRLF/长度负例，有界 trace/脱敏副本 | internal/gb2022、internal/sip、internal/evidence 测试 |
| M0-T-10 | passed：内置双方 X-GB-Ver、外部夹具缺失版本仍保留 failed 检查，与 SIP 成功分离 | internal/app/service_test.go；internal/device/state_test.go |
| M0-T-11 | passed（M0子集）：HTTP/Desktop 快照合同、真实 CLI/桌面/浏览器动作、权限、幂等冲突、忙/取消、存储重启/版本冲突/锁、资源边界 | internal/transport/http_test.go；internal/app；internal/storage；web/e2e |
| M0-T-12 | passed（本轮确认的兼容范围）：UDP/TCP 认证注册/刷新、三周期心跳、独立信息/目录、认证注销与清理。标准对照 failed，旧失败不覆盖 | scripts/test_livegbs.py、interop_evidence.py；internal/app/external_test.go；COMPAT-RUN-001—005 |
| M0-T-13 | passed：默认/旧配置、未知枚举、显式切换/运行中拒绝、内置边界、历史/导出不可重标、两兼容项正常/负例 | internal/app/mode_test.go；internal/sip/endpoint_test.go；internal/gb2022/xml_test.go；internal/evidence/trace_test.go；web/e2e |

以上为明确列出的合成场景，不是穷尽所有恶意网络排列或完整标准字段。Catalog 大批边界只在隔离内存夹具执行，未向外部平台发送大规模目录。

## 外部测试与差异

### 2026-10-07 历史失败

开发和独立测试阶段未读取 LiveGBS 源码/API 文档；进入 P6 后才读取本地指定 API 文档。文档未列基本 SIP 配置接口，因此只读指定来源中测试需要的配置读取调用。Web 登录成功，通过只读基本配置接口取得平台 ID、realm、端口和独立 SIP 密码；未复用 Web 密码、猜测密码、关闭鉴权或修改平台全局设置。

两次独立运行分别使用 UDP/TCP、一个预先证明不存在的随机合成 NVR ID、一个合成通道，零媒体。均在约一秒内收到首 REGISTER 的 SIP 400，未进入 401 挑战。响应 User-Agent 为 LiveGBS v260930，未携 X-GB-Ver，也没有说明拒绝字段。失败后尝试 Expires=0 补偿，同样被拒绝；只读列表确认本次 ID 不存在，没有执行删除已有设备的操作。登录会话已登出。原始 SIP、私有输入、接口观察及摘要在忽略的 artifacts/m0/external。

当时只确认 RFC3261 10.2 和 GB 6.1.2 的 From/To 适用范围，未证明对端 400 的字段原因，因此保留失败。2026-10-11 补审纠正了“带服务器编码形式未经标准支持”的过宽表述：2016 规范性附录有明确示范，2022 删除该旧附录后按本项目记录的引用层级解释实现。首次实现还把数字身份域用于路由 URI，已作为 F-11 独立修复，不能用该历史首包证明现行标准模式已互通。

### 2026-10-11 受控诊断与完整兼容验证

仅在独立实现与检查之后只读指定测试配置和必要 API，未读取第三方实现来替代产品协议设计。工具核对授权目标，显式本地 SIP 密码优先，不复用 Web 密码。每次一个事前确认不存在的合成设备/一个通道、零媒体、正常时钟、最长十分钟；各次结束核查归属和离线状态后，仅移除自己的记录并登出。

[逐事务台账](compatibility-runs.json)记录 COMPAT-RUN-001—005：001 为未认证 URI 诊断；002 因 GB2312 查询被拒绝而 failed；003 UDP passed；004 虽线上的认证注销收到 200，应用因 TCP 关闭竞态 failed；005 修复后 TCP passed。每次保留原先结果，未将 HTTP 成功或原始报文成功替代应用失败。标准模式 UDP/TCP 对照仍为 400，无自动回退且设备确认不存在。

成功的两次流程都包括初始 401/认证 200、刷新 401/认证 200、Catalog 和 DeviceInfo 各自 Query/空 200/独立 Response/空 200、三个 Keepalive 确认，以及 Expires=0 的 401/认证 200。DeviceInfo Result=OK、Channel=1；Catalog 设备自身加一个通道共两项，SN、目标、总数和 Num 一致。UDP 配置下较大目录响应走 TCP 并获确认，TCP 运行全部走 TCP。

UDP 三个实际间隔为 60.998、60.999、61.000 秒；TCP 为 60.997、61.000、61.000 秒。Go 夹具关闭后核查操作与资源计数为零，并重新绑定原 UDP/TCP 端口确认释放。Python 同时核对原始文件长度/SHA-256、独立事务关联、业务应答和正常间隔；平台只读观察为在线，注销后移除本次离线设备，最终确认不存在。原始 SIP/输入/API 仅保存于受保护 artifacts/interoperability，公开材料不含真实平台配置。

## 自审与修复

本轮为单 Agent 自审，不称独立审批。F-01—F-10 为原实施修复，本轮保留并经相关门禁覆盖；F-11—F-15 为本次实施和收尾自审发现，均由当前 Agent 修复复验。

| 编号 | 发现、修复与验证 |
| --- | --- |
| F-01 | TCP 硬关闭后额外减少引用造成警告；改为触发读循环关闭并等待池移除，验证反复注销与端口重用 |
| F-02 | REGISTER 取消/丢失确认可能留下绑定；串行注册生命周期并增加有截止时间的补偿注销，未确认明确保留风险 |
| F-03 | 心跳 deadline 未计为 miss；改为只有显式取消不计，实际三次超时驱动离线并验证恢复 |
| F-04 | 目录 responder 在注销后仍可能等待；记录有界取消句柄，注销取消，最终 context/事务释放 |
| F-05 | 注册剩余秒数向下取整使最小 3600 秒变 3599；改为向上取整，保留 Contact 优先规则 |
| F-06 | XML 全文本查找 encoding 可能误伤名称；只检查 XML 声明处理指令；配置入参深复制，避免保留调用者可变切片 |
| F-07 | TCP 临时地址 trace 路由可能积累；按实际活动流清理别名。敏感串按长度优先掩码，避免较短前缀破坏完整匹配 |
| F-08 | Windows 0600 不提供 ACL 保护；运行目录设置仅当前用户/系统/管理员，Unix 0700，存储及宿主复验 |
| F-09 | 错 CSeq 响应先进入 sipgo 会提前结束事务；在入站过滤器匹配活跃请求期望，真实错序响应和 Linux race 复验 |
| F-10 | 随机 UDP 端口不一定同时可供 TCP 绑定；仅隔离端口 0 模式有界重选，显式配置端口仍直接报告冲突 |
| F-11（P1，closed） | 数字 GB 身份域曾同时充当 REGISTER/MESSAGE 的 RFC 路由主机，内部对端也未拒绝；分离路由地址、From/To 域与 realm，内置 registrar 校验接收路由/AoR。独立向量重新生成，路由/注入正反例、内部 UDP/TCP 与标准外测对照复验 |
| F-12（P1，closed） | 外测工具缺少显式密码优先级、完整独立线包判定及异常归属清理；改为有界直接持有测试进程、严格参数和 API 形状校验、原始证据独立验证、所有退出路径确认本次资源。14 个合成回归、真实失败/成功/移除路径复验 |
| F-13（P1，closed） | 原外测缺少用户要求的显式兼容合同，旧 GB2312 查询不能进入业务；增加 COMPAT-001/002，限定模式/外部绑定/ASCII 基础查询，保留 GB18030 响应及全部原约束。结构解析改从原字节读取再统一脱敏；模式/XML/证据正反例与 UDP/TCP 外测复验 |
| F-14（P1，closed） | TCP 对端发送最终响应后立即关闭，sipgo 实验性断链终止可能抢先丢弃已读响应，应用错误保留上一个 401；独立 48 次端点夹具在旧选项下复现丢失 200，移除该实验选项、使用默认事务生命周期。完整 200/401/403 后关闭、无响应截止、断链重注册、Linux race、COMPAT-RUN-005 复验 |
| F-15（P2，closed） | 模式切换可能误标历史，且无交换证据的 URI 检查不能预报通过；操作和报文固定当时模式/revision，兼容项深复制，旧存储默认标准，运行中拒绝切换，SIPStatus=0 的 URI 检查为 not-run。持久化、历史/导出、调用者修改、取消前无交换和 Web E2E 复验 |

2026-10-11 收尾自审核对标准解释、默认/显式模式、认证 URI、不可重标证据、配置/存储兼容、来源/事务/SN、工具异常清理与最终差异。暂存仅包含源码、合成用例、规则、公开文档和脱敏台账；本地凭据/资料路径在新增行检查中无匹配，忽略目录无跟踪文件。当前 M0 范围内无未关闭核心问题；严格 2022 第三方符合性、媒体/安全/容量/完整发行与用户主观验收仍属明确后续范围。独立 Agent 审查 not-run。当日仅保存本地阶段提交；运行与下一步见[实现合同](../delivery/m0-implementation.md)，完整 LAB-S/E/X 后续叶节点见[能力资产](../delivery/capability-leaves.json)。

## 2026-10-13 历史整理、复验与集成

依据 A-019，由同一 Agent 连续完成本次整理、验证和自审，独立 Agent 审查 not-run。开始时工作区干净，旧 main 为 024ecbf，最终成果为 3e5a512；远端对应引用已核对。193 个跟踪文件全部保留，源码、合成夹具、协议资产、锁文件和许可没有改动。历史计划/报告仍承担版本与验证说明，不在合并前删除；只精炼当前状态、授权和失效的远端说明。

| 项目 | 实际依据与范围 |
| --- | --- |
| 恢复备份 | 全部 13 个原引用保存至忽略的 .local/integration-20261013/before.bundle，git bundle verify passed；SHA-256 为 06b32023cd01ba738407c6d5dc66bbaa53ee9c5ac9f504d0b57a48099ac0bde8 |
| 前两个提交 | 保留初始化 2556c00；新文档基线 c73b0b2 的树与 024ecbf 完全一致，已通过显式旧 SHA 的 force-with-lease 推送 main |
| M0 内容 | 准备 eb57314、实施 03d319d、协作规范 3e5a512 合为一个阶段提交；初始归并树与 3e5a512 完全一致，随后仅同步本次交付文档 |
| 保护恢复 | 基线改写的临时管理员绕过已移除；main-protection 保持 active、bypass_actors 为空，五项规则与操作前逐项一致 |
| python scripts/verify.py | passed：文档同步后的最终暂存版本为 50 份登记文档、359 个本地链接、26 个工具回归，Python 语法及工作区/暂存空白检查全部通过 |
| python scripts/verify_app.py --e2e | passed：冻结安装、Prettier、2 个 Vitest 用例、TypeScript/Vite、模块校验、内部 Go 测试/vet、Windows 三入口构建、1 个真实浏览器 E2E；Go 复用了有效缓存 |
| 远端集成依据 | 本 M0 PR 的 repository-quality 在 Ubuntu 执行仓库门禁及 verify_app.py --race --e2e；检查成功后才允许正常 squash 合并，实际结果由该 PR 的检查和合并记录定位 |

本次自审确认归并没有丢失文件、产品树和既有 F-01—F-15 修复，历史失败及第三方符合性限制仍在。没有重复访问外部平台，也没有重新执行原生桌面操作或人工体验验收；旧外测依据按未变的源码、依赖、样本和配置假设复用。

忽略的用户配置、.local、reference、artifacts、bin、Python 缓存及前端依赖/产物整体保留、未递归计数；没有删除未跟踪用户资料。旧引用的恢复由已校验 bundle 承担，分支清理仅在 M0 合并并核对内容后进行。M0 交付后等待用户启动 M1。
