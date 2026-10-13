# 仓库地图

| 已有目录/文件 | 职责 |
| --- | --- |
| AGENTS.md、CLAUDE.md、CONTRIBUTING.md、REVIEW.md | Agent、开发与审查入口 |
| USER-ACTIONS.md | workspace 唯一用户事项清单的导航 |
| docs/requirements.md、docs/design.md、docs/tech-stack.md | 完整产品基线、架构和选型状态 |
| docs/dev-rules/、product-rules/、design-rules/ | 工程、产品与交互规范 |
| docs/standards/README.md | 共享原文入口与本产品维护边界 |
| docs/delivery/、decisions/、templates/、governance/ | 状态、验收、决定、模板与登记 |
| scripts/ | 文档/仓库验证、verify_app.py 应用门禁、工具回归及 local_env.py 本地环境准备检查 |
| .env.example | 合成配置合同；真实 .env.local 与 reference 索引被忽略 |
| .github/ | CI、PR/Issue 模板与保护期望配置 |
| cmd/ | HTTP、CLI、Windows Wails 宿主 |
| internal/app、config、storage、transport | 统一服务、配置、SQLite、薄适配 |
| internal/sip、gb2022、device、platform、evidence | 事务适配、编码、角色状态、证据 |
| web/ | React/TS 工作台、冻结依赖、Vitest/Playwright |
| protocol-assets/ | 审核规则、独立字节/摘要向量 |
| third_party/ | 实际依赖清单、许可证原文 |

媒体、安全、完整发行安装包仍未建立。已存在入口及构建前置见[运行合同](../delivery/m0-implementation.md)。

`.local/`、`artifacts/`、`bin/`、前端构建/测试输出和本地 `reference/` 都按已有忽略规则管理；用户配置、原图与运行数据不因所在目录被当作缓存删除。初版 UI 的文字依据见[交互规则](../design-rules/DESIGN.md#initial-ui-reference)，计划/报告和证据去向按[材料生命周期](../governance/documentation-policy.md#stage-documents)维护。
