# 文档治理

## 信息归属

当前用户有效要求优先于旧约定。需求规定目标，设计规定实现边界，代码代表实际实现，证据说明某次执行观察；冲突必须定位并修正文档或实现，不能以日期新旧机械判断。历史材料和模板不是新的用户授权。

| 内容 | 正本 |
| --- | --- |
| 开始工作与任务导航 | AGENTS.md、docs/README.md |
| 产品行为、范围、需求编号 | docs/requirements.md |
| 模块、接口、数据与技术取舍 | docs/design.md；技术栈页只汇总与标注选型状态 |
| 开发过程与长期约束 | docs/dev-rules/ |
| 实际完成、开发后续 | docs/delivery/status.md |
| 验收计划与执行报告 | docs/delivery/、docs/evidence/ |
| 长期决定及依据 | docs/decisions/ |
| 用户决定、外部配置、授权、人工检查 | workspace USER-ACTIONS.md；产品入口仅导航 |
| 标准原始资料和哈希 | workspace docs/standards/ |

产品文档独立可读，不通过父目录相对链接形成必须存在的目录依赖。正式共享入口使用 GitHub URL；本机位置和真实配置只放忽略目录。需求编号保持稳定，标准引用记录版本和条款；“随 main 更新”只描述代码集成策略。

## 登记与状态

[document-registry.json](document-registry.json)登记本仓库自有 Markdown，字段 path、kind、status、owner、scope、reviewed、basis。reviewed 仅表示本次结构/内容核对日期，不代表用户批准或产品通过。active 为当前规则，draft 为候选，reference 为来源/证据，superseded 为已替代记录。

workspace 中标准 PDF、OCR 和图片使用来源清单管理；OCR Markdown 不作为开发指令登记，但图片链接和文件哈希仍须检查。新增、移动和删除自有文档同步索引、登记与入链；旧稳定入口可保留短导航，不保存重复正文。

## 变更与证据

保留已有 LAB/HUB 需求编号；长期取舍使用 ADR，用户有效决定使用 A-*。用户事项使用 WS/HUB/LAB-Q/ACT/H-*，开发后续使用 LOOP-*；编号不复用。小修直接改，不为每个错字创建计划和 ADR。

证据状态仅使用 passed、failed、not-run、blocked；记录执行版本、命令、环境、输入、预期、实际、日志位置及限制。无应用时只报告文档/工具验证，不写启动成功、性能达标或国密验收通过。自审与独立审查分别标明。

阶段结束提炼持久结论，过期过程材料可由 Git 历史保留。清理忽略文件前核对内容与绝对路径，用户配置、样本和密钥不自动删除。公开报告仅使用合成身份与脱敏结果，第三方许可证及必要来源声明保留。

## 检查

运行 `python scripts/verify.py`。新增文件先暂存以纳入 Git 跟踪清单；检查器涵盖登记、链接/锚点、越界链接、本机路径和脚本回归，不能代替语义审查、外部链接可用性、安全扫描或产品验收。
