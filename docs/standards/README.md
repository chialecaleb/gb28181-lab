# GB28181 Lab 标准与协议依据

标准基线为 GB/T 28181-2022 与 GB 35114-2017；完整范围和适用角色见[需求](../requirements.md)。原始 PDF、OCR、图片和来源哈希集中维护在 [workspace 标准库](https://github.com/chialecaleb/gb28181-workspace/blob/main/docs/standards/README.md)，代码不通过该仓库提供运行时协议规则服务。

| 标准 | PDF 来源 | SHA-256 | 印刷页与 PDF 页 |
| --- | --- | --- | --- |
| GB/T 28181-2022 | [原文](https://github.com/chialecaleb/gb28181-workspace/blob/main/docs/standards/GBT28181-2022/GBT28181-2022.pdf) | `727dfbd2d1094780d2044800de7c59743550088d871a717c7bb4d36f33710cc6` | PDF 页 = 印刷页 + 7 |
| GB 35114-2017 | [原文](https://github.com/chialecaleb/gb28181-workspace/blob/main/docs/standards/GB35114-2017/GB35114-2017.pdf) | `56ac23dcf25b3287ff2c7cf63a7a21e4e19858b3fbf16080d41b85953045f9db` | PDF 页 = 印刷页 + 4 |

## 使用方式

工作区开发可读取父工作区的标准目录。独立 clone 时可单独获取共享标准仓库，或把合法取得且校验后的资料放入本仓库 `.local/standards/`；未来工具可通过 `GB28181_STANDARDS_ROOT` 定位，当前尚无应用资料加载器。不把父目录绝对路径写入代码，也不要求单元测试联网获取标准。

运行时原文浏览采用本地导入或许可允许的构建期打包，普通产品功能不依赖 GitHub 在线可用。不得因共享资料迁移取消需求中原文定位、条款解释和可追溯性目标；分发许可见[用户事项入口](../../USER-ACTIONS.md)。

## 本产品维护的内容

条款—需求—设计—用例映射、解析与序列化规则、适用角色、差异决定、合成测试向量均归本仓库。共享 OCR 只辅助检索；原文字节、签名算法、字段含义须核对 PDF。产品实现与另一个产品相同不等于标准符合。

已知需要逐项核对的差异包括附录 K 的资料性/规范性标注、Monitor-User-Identity 分隔符、keyversion 表述、C.4 摘要与正文编码。GB35114 的有日期引用不能直接替换为 2022 版业务规则。缺失引用规范和逐字段核对状态见共享来源说明；已有阅读记录不代表本次重新验证或完整验收。

实施每个功能时新增本仓库的可执行测试与证据，规则见[协议工程](../dev-rules/protocol-engineering.md)，追踪入口见[验收](../delivery/roadmap-and-acceptance.md)。
