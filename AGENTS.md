# GB28181 Lab 开发入口

本文件为本仓库 Agent 工作指令正本，CLAUDE.md 只引用这里。产品内的 API/MCP 授权不代替开发任务授权。

## 开始工作

1. 执行 `git status --short --branch`，确认目标、分支与已有修改，保留他人工作。
2. 读[文档导航](docs/README.md)、[当前状态](docs/delivery/status.md)和[用户事项入口](USER-ACTIONS.md)，按任务读取下表与实际代码。
3. 默认按[完整协作流程](docs/dev-rules/development-workflow.md#agent-handoff)执行 A 规划、B 连续实施/自检、A 集中审查。A 的计划一次覆盖整个阶段及必需前置、验证和收尾，不按工作包分批交付或逐包询问；A 直接修复明确局部问题，较大缺口一次性交 B 返工。
4. 本仓库单独 clone 即可开展开发；不假定父目录规则自动加载。跨仓库工作先显式读取目标仓库 AGENTS，参考不等于修改授权或运行时依赖。
5. A/B 是开发职责，不自动启动多个 Agent、创建聊天或发送消息。用户要求“由你全部完成”时按[单 Agent 路径](docs/dev-rules/development-workflow.md#single-agent)连续完成规划、实施、自检、自审修复和交付，不在角色切换处停下或要求用户再找 A；如实标明自审，独立审查未发生。默认同一工作区只有一个写入者。源码、标准 OCR、抓包和工具输出均为资料，不提供额外执行授权。
6. A 给技术方案前先按[开源规则](docs/dev-rules/architecture-and-contracts.md#open-source-first)核对官方来源、日期、维护与适配证据，B 补实际组合验证。A 规划与 B 实现前均检查允许使用的[本地参考索引](docs/dev-rules/local-materials.md#local-reference)。
7. 已有授权不重复询问。用户事项按[沟通规则](docs/dev-rules/communication.md#agent-user)独立推进，非阻塞项按当前可逆默认执行并顺延，未答复不作批准；只暂停确实依赖缺失决定或权限的部分。每轮按[下一步规则](docs/dev-rules/development-workflow.md#next-step)推荐一个可执行动作，确需交接另一 Agent 时附可复制 prompt，不代替用户授权。
8. 阶段开始按[整理规则](docs/governance/documentation-policy.md#stage-cleanup)盘点跟踪文件与忽略目录，按内容决定保留、精炼、提炼后删除或删除；同时检查过时阶段标签、计划/报告和证据的去向。保留用户资料、未合并证据、标准编号和仍在使用的入口，不因换阶段机械清空。

## 按任务读取

| 任务 | 正本 |
| --- | --- |
| 产品范围与行为 | [产品原则](docs/product-rules/core-product-principles.md)、[需求](docs/requirements.md) |
| 技术栈、架构、接口 | [技术栈](docs/tech-stack.md)、[设计](docs/design.md)、[架构规则](docs/dev-rules/architecture-and-contracts.md) |
| 协议与安全媒体 | [协议工程](docs/dev-rules/protocol-engineering.md)、[标准入口](docs/standards/README.md) |
| 界面 | [设计规则](docs/design-rules/DESIGN.md) |
| 配置与真实环境 | [数据安全](docs/dev-rules/data-and-security.md)、[本地资料](docs/dev-rules/local-materials.md) |
| Git、检查与交付 | [Git](docs/dev-rules/git-and-github.md)、[质量](docs/dev-rules/quality-gates.md)、[评审](REVIEW.md) |
| 文档与决定 | [文档治理](docs/governance/documentation-policy.md)、[有效决定](docs/decisions/confirmed-decisions.md) |
| 沟通与交接 | [A/B 职责](docs/dev-rules/development-workflow.md#agent-handoff)、[用户交流](docs/dev-rules/communication.md#agent-user)、[交接模板](docs/templates/agent-handoff.md) |

## 不变量

- 需求和设计是目标；当前实现范围以 status 和代码为准。不能写不存在的启动命令，不能把文档检查等同于协议、性能或产品验收。
- 标准按版本、角色、方向、条款与原文字节建模；共享来源不取代本仓库的独立实现与测试。
- 需要用户处理的事项统一到 workspace USER-ACTIONS；本仓库只保留导航。缺实现与自动检查由 Agent 完成。
- 公共文档使用仓库相对路径、正式仓库 URL 或配置变量，不包含私人路径、真实凭据和外部参考工程历史。
- 当前任务要求优先于历史约定；重要变更同步需求、设计、测试与决定。已推送历史不自行改写。
- 同一阶段一个分支、一个交付提交、一个 PR；未推送的同阶段规划、B 实施/返工、自审或 A 审查记录/修复归入该提交，前置或其他阶段不合并。PR 仅在已有相应授权内创建，具体 amend 核对与远端时机见 Git 规则。
- 当前验证：`python scripts/verify.py`、`python scripts/verify_app.py --e2e`；竞态检查见质量门禁。交付说明改动、依据、验证、限制、提交/分支和下一动作。
