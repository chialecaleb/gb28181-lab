# 需求到证据追踪

需求正本为[requirements.md](../requirements.md)，设计见[design.md](../design.md)。本页只声明实际 M0 子集；完整 LAB-S/E/X 拆分为[449 个功能叶节点](capability-leaves.json)，保留依据、角色待细化点、阶段与缺项。功能拆分不等于全标准条款审核完成。

| 需求子集 | 规则/方向与实现 | 测试和状态 |
| --- | --- | --- |
| LAB-R-001—005 宿主/工程子集 | cmd 三入口、internal/transport、web，共用 internal/app | Windows 桌面实际运行；Linux headless 与 CLI；M0-T-11 passed |
| LAB-C-001 UDP/TCP | internal/sip，RFC 3261/3428/3581 | 重传、分帧、断链、来源/事务校验，M0-T-03/04 passed |
| LAB-C-002/003/005 字节/版本/身份 | internal/gb2022、protocol-assets；6.1.2/6.10/附录 E/I | 独立 GB18030 字节、长度、版本分层，M0-T-09/10 passed |
| LAB-C-004/006 与 LAB-F-001 | 设备→registrar REGISTER/Digest、刷新、注销；internal/device/platform/app | 9.1；M0-T-01/02 passed（内部），外部 UDP/TCP 兼容互通 passed |
| LAB-F-008 / LAB-C-004 | 设备→平台 Keepalive MESSAGE Notify；9.6/A.2.5 | 正常确认、三次发送/接收超时、恢复；M0-T-05 passed（内部），外部 UDP/TCP 各三个正常周期 passed |
| LAB-F-007 / LAB-C-010/011 | 平台 Query→空200→设备新 MESSAGE Response→空200；9.5/A.2.4/A.2.6/J/M | M0-T-06/07/08 passed（内部），外部单设备/单通道查询与独立响应 passed |
| LAB-C-013 两种模式 | internal/app/sip/gb2022/evidence、web；2022 4.3.2/6.1.2/6.10/9.1 与 2016 J.1/J.3 | M0-T-13 passed：默认/显式切换、旧配置、历史/导出、COMPAT-001/002 正反例；标准模式外部对照仍 failed |
| LAB-U-001—008 本工作台子集 | 设备树、操作/参数、报文/hex/结构/时序、分层状态、取消/导出 | web/e2e 真实点击与1280×800检查 passed；主观体验 not-run |
| LAB-E-003/004/009/013/015/016 子集 | 本地状态、六个 API、设备树、SQLite、资源计数、配置恢复 | internal/app/storage/transport；M0-T-11 passed；完整叶节点仍 not-run |
| LAB-X-001/005 子集 | CLI demo、CI 门禁、脱敏 JSON/SHA 原字节关联 | CLI 与导出 passed；完整场景、JUnit、语义重放/版本比较 not-run |
| LAB-S-001—018、其余 LAB-E/X | 完整安全/媒体/用户/级联与实验能力 | 叶节点均保留；M1—M5 实现/审核/验收，M0 不声明支持 |

当前交付版本为包含本报告的 M0 提交（集成分支 codex/m0）。历史准备基线 163d15a、修复前基点 039b013 及各次外测源码指纹保留于[兼容执行记录](../evidence/compatibility-runs.json)与原报告；2026-10-13 的提交归并未改变被测产品代码。M0-T-12 的 UDP/TCP 兼容外测通过，满足本轮确认的退出范围。完整命令、输入边界、历史失败与清理见[实施证据](../evidence/m0-implementation.md)。内部互测、独立向量、第三方兼容互通、严格符合性和密码产品验收分别记录。
