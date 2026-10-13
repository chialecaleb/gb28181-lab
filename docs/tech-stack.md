# GB28181 Lab 技术栈与选型状态

查询日期：2026-10-07。本页汇总 [设计正本](design.md)与 [M0 计划](delivery/m0-plan.md)。本轮通过官方发行/模块索引、包元数据、README、go.mod 和许可证进行基础调研；下表保留初始候选调研；实际锁定与构建结果以本页末尾实施记录为准。

## M0 推荐组合与事实依据

| 层次 | M0 推荐/已查到版本 | 维护、许可、兼容与验证边界 |
| --- | --- | --- |
| Go | Go 1.27.1 候选；本机 1.27.0 | [官方下载元数据](https://go.dev/dl/?mode=json)列出 1.27.1、1.26.8 稳定版本；采用受支持稳定线，BSD 风格许可；构建前记录 go.mod/toolchain，尚未升级本机 |
| Node | Node 24 LTS，24.21.0 候选；本机 24.19.0 | [官方版本索引](https://nodejs.org/dist/index.json)列出 24.21.0/Krypton；不因 26.x 为最新就换到非 LTS；Node 含第三方许可，发布前列清单 |
| 包管理 | pnpm 12.9.1 候选；本机 12.8.1 | [npm 元数据](https://registry.npmjs.org/pnpm/latest)，MIT；落地固定 packageManager 与 lockfile，CI 冻结安装 |
| 前端 | React 19.3.0、TypeScript 7.0.2、Vite 8.3.3 | [React](https://registry.npmjs.org/react/latest)、[TypeScript](https://registry.npmjs.org/typescript/latest)、[Vite](https://registry.npmjs.org/vite/latest)；分别 MIT/Apache-2.0/MIT；Vite engines 为 Node ^20.19.0 或 >=22.12.0，候选 Node 满足声明，不等于构建已测 |
| React 构建插件 | @vitejs/plugin-react 6.1.2 候选 | [包元数据](https://registry.npmjs.org/@vitejs/plugin-react/latest)，MIT、Vite ^8.0.0；OXC/Babel/React compiler 的 peer 和可选依赖在冻结安装时核实，避免混用旧模板 |
| Windows 宿主 | Wails v2.15.0 候选、WebView2 | [模块索引](https://proxy.golang.org/github.com/wailsapp/wails/v2/@v/v2.15.0.info)、[go.mod](https://proxy.golang.org/github.com/wailsapp/wails/v2/@v/v2.15.0.mod)、[MIT 许可](https://github.com/wailsapp/wails/blob/v2.15.0/LICENSE)；模块 Go 最低 1.25.0，Windows 10/11、WebView2，实际 doctor/build 未运行 |
| Linux 宿主 | Go net/http + 同一 React 静态产物 | Go 标准库优先；独立 headless 入口，不 import Wails、不需要 DISPLAY；Linux 构建/运行待工程验证 |
| SIP | sipgo v1.6.0，封装在 internal/sip | [release](https://github.com/emiago/sipgo/releases/tag/v1.6.0)发布于 2026-09-20；[模块](https://github.com/emiago/sipgo/blob/v1.6.0/go.mod)为 Go 1.23.0/toolchain 1.24.2；[BSD-2-Clause](https://github.com/emiago/sipgo/blob/v1.6.0/LICENSE)；自带 UDP/TCP/事务，GB 规则自建 |
| GB 字符集 | golang.org/x/text v0.42.0 候选 | [索引](https://proxy.golang.org/golang.org/x/text/@v/v0.42.0.info)、[模块](https://proxy.golang.org/golang.org/x/text/@v/v0.42.0.mod)；Go >=1.26.0；使用 simplifiedchinese.GB18030 方向，许可与确切编码向量在引入前复核 |
| 持久化 | database/sql + modernc.org/sqlite v1.60.1 候选 | [驱动说明](https://pkg.go.dev/modernc.org/sqlite@v1.60.1)、[模块](https://proxy.golang.org/modernc.org/sqlite/@v/v1.60.1.mod)；2026-09-29，Go >=1.26、BSD-3-Clause、Windows/Linux amd64，避免 CGO 安装门槛；驱动要求与模块完全一致的 libc 依赖，不能独立浮动升级 |
| UI 验证 | Vitest 5.0.3、Testing Library React 16.3.3、Playwright Test 1.63.0 候选 | [Vitest](https://registry.npmjs.org/vitest/latest)、[Testing Library](https://registry.npmjs.org/@testing-library/react/latest)、[Playwright](https://registry.npmjs.org/@playwright/test/latest)；MIT/MIT/Apache-2.0，Node24/React19/Vite8 满足声明范围；DOM/浏览器依赖及实际交互待安装后验证 |
| MCP | 官方 Go SDK v1.8.0，M0 只研究/预留适配 | [release](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.8.0)为 2026-09-14；[README 矩阵](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/README.md)说明 v1.7+ 默认支持 2026-07-28；Go >=1.25.0；[许可](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/LICENSE)处于 MIT→Apache-2.0 迁移，保留适用原许可与 NOTICE，不能简写为全部已转 Apache |
| 媒体与密码 | FFmpeg/libav、独立密码提供器接口；M0 无媒体/安全实现 | [FFmpeg 许可说明](https://ffmpeg.org/legal.html)为 LGPL-2.1-or-later，启用 GPL 组件会改变许可；原生 worker 隔离，具体构建、SVAC、密码产品与介质留 M1+ 门禁 |

准备时确认 Python 3.12.14、Git 和以上 Go/Node/pnpm 命令可执行，当时只验证工具版本与环境准备脚本，未运行应用构建。后续 Go、React 与宿主的真实构建/运行结果见本页末尾[实际锁定与验证](#m0-实际锁定与验证)；doctor 未运行。准备任务未修改全局工具链。

## Wails 版本来源差异

[安装文档](https://wails.io/docs/gettingstarted/installation/)显示 v2.15.0；Go 模块索引确认 v2.15.0（2026-08-17，源提交 713dc89694c789ee0a45355c2886cef1986e9a01），可读到对应 go.mod 与 MIT LICENSE。与此同时 [GitHub latest release API](https://api.github.com/repos/wailsapp/wails/releases/latest)返回 v2.14.0（2026-08-10），v2.15.0 release API 返回 404。“有模块标签”与“有 GitHub Release”分别核对，不能据其中一个否定另一个。

准备时将 v2.15.0 列为首选候选，之后的模块校验、编译、WebView2 与宿主隔离结果见本页实际锁定记录。后续 A 先查证候选，B 执行组合验证，再由 A 核对。安装文档仍泛写 Go 1.21+，具体 v2.15.0 模块要求 Go 1.25.0，实际模块声明优先。若候选验证失败，记录失败后选已验证的稳定版本，不默默降级或使用浮动 latest。

## SIP 候选比较与自研边界

| 方案 | 查证事实 | M0 选择与退出成本 |
| --- | --- | --- |
| sipgo v1.6.0 | 明确版本 release、UDP/TCP、事务/对话、注册认证示例；README 提醒部分严格校验不在解析器中，并提供 ValidateRequest/Response | 首选适配；必须验证真实原字节捕获、GB18030、Content-Length、CSeq 自动递增和生命周期；通过适配接口可替换库 |
| ghettovoice/gosip | [仓库](https://github.com/ghettovoice/gosip)API 查得未归档、push 日期 2026-09-19、BSD-2-Clause；[go.mod](https://github.com/ghettovoice/gosip/blob/master/go.mod)仍为 Go1.13 和较早依赖；latest release API 404 | 作为备选；不能仅凭一次 push 宣称测试/维护质量更好，也不能声称已停更。进入备选时须核对 commit 与字节/事务测试 |
| 完全自研 SIP 栈 | 需承担 RFC 分帧、解析、重传、事务、对话、认证和异常输入的完整维护 | M0 不选；已有库无法满足的具体缺口才单独论证，禁止为四个方法重写完整 SIP 栈 |

Lab 自研 GB 2022/35114 字段规则、设备/平台状态、操作关联、配置、独立向量与证据。sipgo README 表明新事务可隐式递增已有 CSeq，必须在适配层明确选项；用 transport 实际字节观察，不能重新序列化“近似报文”充当已发送原文。外部 LiveGBS 不参与上述选型/实现依据。

## 其他取舍和实施门禁

- 持久化优先 SQLite WAL、单写/有界连接和关键配置迁移；SQLite 本体与 Go 驱动的许可分开核对。MySQL/PostgreSQL 后端继续保留最终承诺，不在本切片先安装三个数据库。
- UI 首轮用 React 组件、语义表单和 CSS 变量搭工作台，保持 HostAdapter 边界；有实际需要再引入 UI/状态/路由库并核对许可，不为截图增加整套无关依赖。
- dotenv 准备工具只依赖 Python 标准库。已比较 [godotenv v1.5.1](https://github.com/joho/godotenv/releases/tag/v1.5.1)（2023-02-05 发布），不因成熟库存在就改变本地文件的字面值语义；Go 侧需按 [配置合同](dev-rules/local-test-environment.md)实现小型白名单加载器并用共享输入复验，而不是调用 Python 或全局 os.Setenv。
- 不以依赖文档中的性能宣传关闭 Lab 容量门禁。M0 工程锁定前，至少完成 Go/平台组合、React/插件组合、SQLite 与 libc、SIP 字节/事务、MCP 将来客户端兼容性对应的记录；未测保持 not-run。
- 引入依赖时保存 go.mod/go.sum、package.json/pnpm-lock.yaml、版本化构建工具配置、第三方许可清单；M0 准备阶段不制造没有工程消费者的锁文件。新增/升级每项是可审查变更。

准备时可运行的是 `python scripts/local_env.py`、可选 `--probe` 与 `python scripts/verify.py`；应用建立后新增 `python scripts/verify_app.py --e2e`。完整计划与当前实际证据分别见 [M0](delivery/m0-plan.md)和[实施报告](evidence/m0-implementation.md)，初始准备报告保留历史范围。

## M0 实际锁定与验证

2026-10-07 采用本机 Go 1.27.0 / Node 24.19.0 / pnpm 12.8.1，不升级全局环境。React 19.3.0、TypeScript 7.0.2、Vite 8.3.3、plugin-react 6.1.2、Vitest 5.0.3、Playwright 1.63.0 已冻结安装、类型/生产构建和真实 E2E 验证。没有引入 Testing Library，实际浏览器承担关键交互测试。

sipgo 1.6.0、icholy/digest 1.1.0、x/text 0.42.0、SQLite 1.60.1 / libc 1.77.1 / memory 1.12.1 / x/sys 0.48.0 已集成并通过内部网络、存储和竞态测试。独立 GB 编码、关联、状态和证据由本仓库实现，sipgo 负责 SIP 事务/网络，Digest 使用成熟库并加参数与重放约束。

Wails 2.15.0 模块及校验和有效，latest Release 仍为 2.14.0，2.15.0 Release API 404。已实测 production Windows 构建、WebView2 启动、REGISTER/Catalog 与关闭，因此按模块版本锁定；没有宣称通过未运行的 doctor、安装器或离线发布验证。Linux HTTP/CLI 无 Wails 依赖，已在 WSL 运行，race 使用隔离 Zig 0.17.0 C 编译器。

实际许可和必要 NOTICE 见[许可清单](../THIRD-PARTY-NOTICES.md)。MCP SDK 1.8.0 与 2026-07-28 规范仅调研，不进入模块依赖；未来 stdio/Streamable HTTP 适配复用统一服务令牌、幂等、取消和资源预算。完整安全引用与角色叶节点仍有明确缺项，见[能力资产](delivery/capability-leaves.json)。

前端源码格式化使用已查询 npm 官方元数据并锁定的 Prettier 3.9.9（MIT、Node >=14），仅开发工具；许可原文纳入清单，门禁执行格式检查。

2026-10-11 收尾未新增或升级依赖。重新查询 sipgo 官方 release，最新仍为 [v1.6.0](https://github.com/emiago/sipgo/releases/tag/v1.6.0)（2026-09-20）；锁定模块的[事务层源码](https://github.com/emiago/sipgo/blob/v1.6.0/sip/transaction_layer.go)将 WithTransactionLayerTerminateOnConnClose 标为 Experimental。独立 TCP 夹具复现最终响应后立即断开时的丢失，适配层改用库的默认事务生命周期，保留调用截止时间/关闭/断链恢复并通过复验。没有采纳未合并的上游补丁或另写 SIP 事务栈；具体发现见 F-14。
