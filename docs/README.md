# GB28181 Lab 文档导航

当前已完成本轮 M0 工程与真实四信令工作台，UDP/TCP 显式兼容外测通过。默认标准模式、兼容差异与完整符合性分别记录；代码、标准、性能与人工验收状态以[当前状态](delivery/status.md)和实际证据为准。

| 入口 | 内容 |
| --- | --- |
| [AGENTS](../AGENTS.md)、[贡献](../CONTRIBUTING.md)、[评审](../REVIEW.md) | 开发、验证、交付 |
| [用户事项](../USER-ACTIONS.md) | 导航到三仓库集中正本 |
| [需求](requirements.md)、[产品原则](product-rules/core-product-principles.md) | 角色、范围、功能、不变量 |
| [设计](design.md)、[技术栈](tech-stack.md) | 架构、边界、已有选型与待核对版本 |
| [仓库地图](dev-rules/repo-map.md) | 当前资产与规划目录区别 |
| [A/B 职责](dev-rules/development-workflow.md#agent-handoff)、[单 Agent](dev-rules/development-workflow.md#single-agent)、[用户沟通](dev-rules/communication.md#agent-user)、[交接模板](templates/agent-handoff.md) | 默认分角色协作；一人包办时连续完成自审交付；用户事项独立推进 |
| [流程同步计划](delivery/collaboration-process-plan.md)、[逐文件核对](evidence/collaboration-process.md) | 开发规范的逐项适配、真实验证与自审记录，包含本轮 UI 参考登记 |
| [Git](dev-rules/git-and-github.md)、[质量](dev-rules/quality-gates.md) | 提交、PR、分支保护、真实门禁 |
| [架构规则](dev-rules/architecture-and-contracts.md)、[协议工程](dev-rules/protocol-engineering.md) | 依赖、接口与标准映射 |
| [交互](design-rules/DESIGN.md)、[初版 UI 参考](design-rules/DESIGN.md#initial-ui-reference)、[安全](dev-rules/data-and-security.md)、[本地资料](dev-rules/local-materials.md) | UI 布局与视觉依据、配置、真实数据与参考 |
| [验收](delivery/roadmap-and-acceptance.md)、[需求追踪](delivery/traceability.md) | 开发路线、用例与证据对应 |
| [运行合同](delivery/m0-implementation.md)、[实施证据](evidence/m0-implementation.md)、[能力叶节点](delivery/capability-leaves.json)、[许可](../THIRD-PARTY-NOTICES.md) | 已有入口、真实验证、自审与后续完整范围 |
| [M0 计划](delivery/m0-plan.md)、[协议调研](delivery/m0-protocol.md)、[单 Agent 历史输入](delivery/m0-agent-a-prompt.md) | 当前阶段唯一实施计划、标准依据与执行交接 |
| [本地测试环境](dev-rules/local-test-environment.md)、[准备证据](evidence/m0-preparation.md) | 已有工具、真实检查、测试资料用途与未运行项 |
| [标准](standards/README.md) | 共享原文与产品解释边界 |
| [标准/兼容模式](compatibility.md) | 2022/2016 原文核对、COMPAT 项、全部兼容尝试与清理 |
| [文档治理](governance/documentation-policy.md)、[阶段整理](governance/documentation-policy.md#stage-cleanup)、[登记表](governance/document-registry.json) | 正本、状态、文件去留、材料生命周期与链接规则 |
| [有效决定](decisions/confirmed-decisions.md)、[组织决定](decisions/0002-workspace-foundation.md) | 当前决定与理由 |
| [模板](templates/README.md) | 计划、模块、审查和交接 |

保留 requirements.md 与 design.md 中已有细粒度需求和接口，避免重复另建一份缩略需求作为正本。开发时按任务阅读相关部分，不要求每次加载全部文档。
