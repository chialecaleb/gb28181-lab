# M0 准备阶段证据

日期：2026-10-07。范围为本地环境准备、基础调研、实施计划与交接文本；用户明确将应用和四项信令实施交给后续 Agent A。自审，不是独立审批，也不是 M0 产品验收。

## 版本、范围与来源

- Lab：准备分支 codex/m0-preparation，基点 e72527a；最终准备提交通过该分支 git log 定位。
- workspace：准备分支 codex/lab-m0-preparation，基点 26baa40；只同步共同环境与用户事项，不锁产品组合提交。
- Hub 与指定外部代码仓库未修改；未推送、未创建 PR、未合并/发布、未启动 Agent A。
- 视觉来源为用户附件，只复制到忽略的本地 reference 索引。公开文档保存功能映射，不保存原图、私有路径或实际凭据。
- 技术调研使用 [技术栈页](../tech-stack.md)列出的官方索引、模块/包声明与许可证；标准原页及限制见 [协议调研](../delivery/m0-protocol.md)。未读取 LiveGBS 源码/API 文档，不以其作为开发参考。

## 已执行的准备检查

| 检查/实际命令 | 结果 | 解释 |
| --- | --- | --- |
| go version；node --version；pnpm --version；python --version | passed | 本机 Go1.27.0、Node24.19.0、pnpm12.8.1、Python3.12.14；未升级全局工具 |
| python scripts/local_env.py --probe | passed | 本地配置结构通过；Web 测试端口 TCP 可连接；4 类 Lab 端点共 6 项 UDP/TCP bind 通过后立即释放 |
| git -C 仓库路径 check-ignore（实际本地配置、索引、原图、运行目录与证据目录） | passed | 所有指定本地材料均被忽略；真实值未写入可提交文件 |
| Python unittest 的 test_local_env.py | passed | 9 项，覆盖优先级/空值、字面值、错误不泄漏、远程/畸形目标拒绝、端口/身份、认证不推断、默认不联网、探测失败与释放 |
| Lab 仓库 python scripts/verify.py | passed | 44 篇登记文档、161 个本地链接、12 项工具测试、仓库边界与工作树/暂存空白检查通过 |
| workspace 仓库 python scripts/verify.py | passed | 25 篇登记文档、2 份标准文档、119 个本地链接、121 个标准资源 SHA-256/大小、9 项工具测试和边界/空白检查通过 |
| 两仓库暂存新增文本与本地 TEST_* 对照扫描、git ls-files 检查 | passed | 用户密码、平台 ID、外部源码路径、测试 API URL 未进入新增文本；.env.local/reference 未跟踪。不是完整通用秘密扫描保证 |
| GB28181 PDF 页渲染与视觉核对 | passed | 只核对协议调研列出的 10 页；OCR 不作为原文，未改动标准资料 |

本次初始环境检查中 Get-NetTCPConnection/Get-NetUDPEndpoint 没有返回匹配项，不能据此判断服务不存在；随后独立 TCP connect 成功，结论以这一实测为准。渲染工具使用可用的 Poppler pdftoppm；pdftotext 不可用，改用 pypdf 定位后仍进行了原页视觉核对。工具不可用没有变成产品通过结论。

## 未执行的验收

| 内容 | 状态 | 后续责任 |
| --- | --- | --- |
| Lab Go/React/Wails 工程、双宿主启动/构建 | not-run | M0-P1—P5，尚无应用源码/依赖锁 |
| REGISTER/Keepalive/DeviceInfo/Catalog 实现与协议测试 | not-run | M0-P3/P4，先独立向量再行为 |
| LiveGBS Web 登录、API 查询、SIP 参数发现和互通 | not-run | M0-P6，仅测试阶段可读指定资料；TCP 可连接不能证明应用身份或登录成功 |
| SIP 认证方式、realm、用户名和密码 | not-run | 本地明确 unknown/空；不复用 Web 凭据、不擅自禁用认证 |
| 媒体、PTZ、GB35114、容量、实机安全产品、人工 UI 验收 | not-run | 按完整产品路线，不计入本轮准备通过 |

## 自审结论与交接

已修正准备工具对显式 HTTP 端口 0 的错误默认回退，并加入回归；端口 0 现在被拒绝。已区分 Wails 模块标签与 GitHub Release 差异、候选依赖与已验证组合。已在计划/规则/本地索引/prompt 中一致落实 LiveGBS 的测试专用边界。

执行 [M0 计划](../delivery/m0-plan.md)，使用 [Agent A prompt](../delivery/m0-agent-a-prompt.md)。用户事项 WS-ACT-001/002 已在工作区记录本次新增输入，当前没有需要用户立即处理的阻塞项；项目许可/标准分发的 WS-Q-001/002 不因本轮改变结论。
