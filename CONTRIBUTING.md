# 开发与贡献

先读 [AGENTS](AGENTS.md)、[文档导航](docs/README.md)与[状态](docs/delivery/status.md)。当前交付仅为文档、治理和检查工具，应用尚未实现。

1. 明确需求 ID 和受影响模块，阅读适用规则与设计。
2. 按[流程](docs/dev-rules/development-workflow.md)完整实现正常与失败路径，公共接口和文档同步。
3. 运行 `python scripts/verify.py`（Python 3.10+、Git，仅 Python 标准库）；新增文件先按路径暂存，门禁按 Git 跟踪清单检查。
4. 按 [REVIEW](REVIEW.md)自审；按 [Git 规则](docs/dev-rules/git-and-github.md)保存阶段提交，在有效授权内推送、创建 PR、合并。

产品依赖和构建命令在工程落地时登记[技术栈](docs/tech-stack.md)与[质量门禁](docs/dev-rules/quality-gates.md)，不凭计划宣称已经可运行。实际协议验收使用独立对端和可复现证据。

所有用户事项由[统一入口](USER-ACTIONS.md)管理。许可证尚未决定，不从参考项目继承许可或 DCO 声明；第三方组成须核对来源及许可证。
