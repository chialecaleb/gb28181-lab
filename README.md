# GB28181 Lab · 国标视频协议模拟实验室

工程 / 目录 / 仓库名称：`gb28181-lab`。

GB28181 Lab 面向 GB/T 28181-2022 与 GB 35114-2017 的完整适用功能，提供独立设备 / 平台 / 安全终端模拟和第三方协议实验。核心采用 Go，Windows 桌面采用 Wails，Linux 提供无 GUI 的 Go Web 服务，前端采用 React + TypeScript。

产品需求覆盖 A / B / C 的真实认证 / 签名 / 加密、完整平台接入与级联、六协议输出、分屏轮巡、平台实录、用户 / SSO、三数据库，以及证书素材、安全故障、多域拓扑、CLI / CI 和证据比较；软件实验与真实密码产品的验收结论分别记录。代码和文档随 main 持续更新，不锁定与其他产品的组合版本。

- [需求文档](docs/requirements.md)：协议角色、全部行为目标、可编辑配置、自操作实验、媒体、MCP 和验收条件。
- [设计文档](docs/design.md)：设备状态机、批量生成、网络调度、虚拟平台、全景媒体、宿主、API 和任务设计。
- [标准资料与核对说明](docs/standards/README.md)：共享原始资料、定位、已知差异与本产品规则职责。

本仓库自行维护实现、文档、规则、场景、测试、配置和发布包。设备可以直接接入第三方平台，也可以注册到软件内置的虚拟平台完成实验。当前 M0 工程与四信令切片已按本轮范围完成，UDP/TCP 显式兼容外测通过；默认采用 GB/T 28181-2022 标准模式，容量和完整协议符合性仍未验证。

## 开发与文档入口

- [文档导航](docs/README.md)：需求、设计、治理、标准与模板。
- [开发指令](AGENTS.md)与[参与开发](CONTRIBUTING.md)：连续执行、沟通、Git 与质量门禁。
- [A/B 开发协作](docs/dev-rules/development-workflow.md#agent-handoff)：A 完整规划、B 连续实施、A 集中审查；明确一人包办时按[单 Agent 路径](docs/dev-rules/development-workflow.md#single-agent)完成自审交付。[用户交流](docs/dev-rules/communication.md#agent-user)独立推进，事项只看 workspace 唯一入口。
- [初版工作台 UI 参考](docs/design-rules/DESIGN.md#initial-ui-reference)：蓝白浅色工作台、设备树、操作区、参数区与信令证据布局；示意图与实际实现/验收分开。
- [当前状态](docs/delivery/status.md)：实际交付和后续工程工作。
- [用户事项入口](USER-ACTIONS.md)：导航到 workspace 的唯一清单。
- [技术栈](docs/tech-stack.md)：技术方向、选型状态与工程前核对要求。

当前检查为 `python scripts/verify.py` 与 `python scripts/verify_app.py --e2e`；Linux 有 C 编译器时追加 `--race`。本地配置检查仍为 `python scripts/local_env.py`，详见[配置说明](docs/dev-rules/local-test-environment.md)。

实际构建、启动和 API 合同见 [M0 运行说明](docs/delivery/m0-implementation.md)。先执行 `pnpm --dir web install --frozen-lockfile`、`pnpm --dir web build`，再 `go run ./cmd/gb28181-lab-server` 启动 HTTP，或 `go run ./cmd/gb28181-lab demo` 运行实际四信令演示。Windows Wails 桌面构建命令也已验证，详见运行说明。

[M0 计划](docs/delivery/m0-plan.md)、[协议依据](docs/delivery/m0-protocol.md)、[实施证据与自审](docs/evidence/m0-implementation.md)、[第三方许可](THIRD-PARTY-NOTICES.md)分别记录范围、规则、检查和依赖。[兼容文档](docs/compatibility.md)记录两种模式、2022/2016 原文差异、全部尝试与清理。LiveGBS 仅在独立实现/测试完成后用于 P6；标准模式仍返回 REGISTER 400，兼容成功不宣称严格 2022 符合性。标准可单独获取，产品构建和运行不依赖其他产品仓库。
