# 开发与贡献

先读 [AGENTS](AGENTS.md)、[文档导航](docs/README.md)、[当前状态](docs/delivery/status.md)和[用户事项入口](USER-ACTIONS.md)。Lab 已有 Go/React、桌面/HTTP/CLI 和信令切片；实际运行范围与外部失败以状态和证据为准，不代表完整产品验收。

## 完成一次变更

1. 明确需求或治理目标，核对代码、数据、协议与相关正本；跨模块/公共契约用[提案](docs/templates/change-proposal.md)的必要部分，模板本身不是审批节点。
2. 默认按 [A 规划 → B 执行 → A 审查](docs/dev-rules/development-workflow.md#agent-handoff)协作。A 给覆盖整个阶段的可执行计划、前期技术查证和整理清单；B 连续实施、自检、修复；A 直接处理局部问题，较大缺口集中返工。用户要求单 Agent 全部完成时，由当前 Agent 连续完成这些职责和收尾自审。
3. 同步受影响需求、设计、接口、迁移、规则、测试与证据；新用户指示回写后继续，不逐工作包等待“继续”。
4. 运行与风险匹配的[质量门禁](docs/dev-rules/quality-gates.md)，按 [REVIEW](REVIEW.md)核对最终成果，区分自检/自审与独立 A 审查。
5. 按 [Git 规则](docs/dev-rules/git-and-github.md#stage-commits)保存一个阶段分支和交付提交。未推送同阶段返工、审查记录与修复 amend；其他阶段不归并，已推送默认追加。获准远端交付后使用同一个阶段 PR，核对实际待合入范围与前置门槛。
6. 提供版本、报告、限制及一个首选下一步；确需交另一 Agent 时用[交接模板](docs/templates/agent-handoff.md)附可复制 prompt。单 Agent 任务完成自审后直接交付整体结果，不留默认 A/B 交接给用户。没有自动创建聊天、启动 Agent 或发送消息的授权。

## 实际入口

| 变更 | 命令/依据 |
| --- | --- |
| 文档、治理 | `python scripts/verify.py`；新增文件先按路径暂存，检查 Git 跟踪清单 |
| 应用、接口、依赖、UI | `python scripts/verify_app.py --e2e`，定向检查和 Linux race 按质量门禁 |
| 外部平台、媒体、安全、容量 | 仅在对应任务授权、真实前置和预算满足时执行，未运行明确记录 |

构建/启动见[运行合同](docs/delivery/m0-implementation.md)，技术选择见[技术栈](docs/tech-stack.md)。文档变更不要求无关 SIP、设备或性能复测；命令不存在时补齐必要实现或说明限制，不用静默跳过当通过。

## 用户参与与交付

用户的全部决定、人工检查、额外授权和配置只在 workspace 唯一清单维护，产品[入口](USER-ACTIONS.md)只导航。非阻塞项按当前默认推进、跨阶段顺延；人工步骤写到可直接照做。Agent 开发缺口留在计划/状态，不转嫁给用户。授权 push、PR、合并、发布各自按实际范围处理。

本轮流程规则不引入新的开源许可、DCO 或签名声明；第三方组成仍须核对来源与许可证。项目许可证和其他长期取舍依照本仓已有决定及集中事项，不能继承外部工程的历史授权。
