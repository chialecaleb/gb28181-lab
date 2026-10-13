# GB28181 Lab 标准与协议依据

标准基线为 GB/T 28181-2022 与 GB 35114-2017；完整范围和适用角色见[需求](../requirements.md)。原始 PDF、OCR、图片和来源哈希集中维护在 [workspace 标准库](https://github.com/chialecaleb/gb28181-workspace/blob/main/docs/standards/README.md)，代码不通过该仓库提供运行时协议规则服务。

| 标准 | PDF 来源 | SHA-256 | 印刷页与 PDF 页 |
| --- | --- | --- | --- |
| GB/T 28181-2022 | [原文](https://github.com/chialecaleb/gb28181-workspace/blob/main/docs/standards/GBT28181-2022/GBT28181-2022.pdf) | `727dfbd2d1094780d2044800de7c59743550088d871a717c7bb4d36f33710cc6` | PDF 页 = 印刷页 + 7 |
| GB/T 28181-2016（差异对照及有日期引用） | [原文](https://github.com/chialecaleb/gb28181-workspace/blob/main/docs/standards/GBT28181-2016/GBT28181-2016.pdf) | `28573e3e6e1e1aeb7695442f1586868004496d7d807f2a01b0233235cac529af` | PDF 页 = 印刷页 + 5 |
| GB 35114-2017 | [原文](https://github.com/chialecaleb/gb28181-workspace/blob/main/docs/standards/GB35114-2017/GB35114-2017.pdf) | `56ac23dcf25b3287ff2c7cf63a7a21e4e19858b3fbf16080d41b85953045f9db` | PDF 页 = 印刷页 + 4 |

## 使用方式

工作区开发可读取父工作区的标准目录。独立 clone 时可单独获取共享标准仓库，或把合法取得且校验后的资料放入本仓库 `.local/standards/`；未来工具可通过 `GB28181_STANDARDS_ROOT` 定位，当前尚无应用资料加载器。不把父目录绝对路径写入代码，也不要求单元测试联网获取标准。

运行时原文浏览采用本地导入或许可允许的构建期打包，普通产品功能不依赖 GitHub 在线可用。不得因共享资料迁移取消需求中原文定位、条款解释和可追溯性目标；分发许可见[用户事项入口](../../USER-ACTIONS.md)。

## 本产品维护的内容

条款—需求—设计—用例映射、解析与序列化规则、适用角色、差异决定、合成测试向量均归本仓库。共享 OCR 只辅助检索；原文字节、签名算法、字段含义须核对 PDF。产品实现与另一个产品相同不等于标准符合。

已知需要逐项核对的差异包括附录 K 的资料性/规范性标注、Monitor-User-Identity 分隔符、keyversion 表述、C.4 摘要与正文编码。GB35114 的有日期引用不能直接替换为 2022 版业务规则。缺失引用规范和逐字段核对状态见共享来源说明；已有阅读记录不代表本次重新验证或完整验收。

实施每个功能时新增本仓库的可执行测试与证据，规则见[协议工程](../dev-rules/protocol-engineering.md)，追踪入口见[验收](../delivery/roadmap-and-acceptance.md)。

2026-10-11 按用户要求先用两版 OCR 验证 REGISTER URI 和字符集差异，再复核决定结论的 PDF 原页。2022 PDF 页 7、16—18、23—24，2016 PDF 页 120、122 已视觉核对；其余补充条款的 OCR/原页状态逐项见[标准/兼容文档](../compatibility.md)。2016 的旧 J.1/J.3 与 2022 的现行附录 J 不是同一内容，不混用编号。

本次 OCR MD 的 SHA-256：2022 为 `4b93ea2586a292003fb0f59f37001701b29ac53da295e518ba5855c43e8d6bc4`，2016 为 `6b26ef8d4830cb9025b7ace0842bfafa90ec4dfcd1d05c6cfea7f102b199f79d`。这些身份只用于定位阅读证据，不把 OCR 作为额外开发指令或完整符合性结论。
