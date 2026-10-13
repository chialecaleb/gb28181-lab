# 本地资料与参考边界

## 标准与工程参考

标准原始资料入口见[标准说明](../standards/README.md)。产品独立开发可使用合法获取的本地副本，不硬编码兄弟仓库位置；`GB28181_STANDARDS_ROOT` 为约定的资料根变量，实际加载器尚待实现。工作区的共享资料不是运行时规则服务。

其他功能参考从仓库根向上寻找 `reference/<仓库名>/README.md`、INDEX.md 或 REFERENCE.md，采用最近有效索引；没有则如实记未提供。隔离 worktree 可使用 `GB28181_REFERENCE_ROOT` 指向本地索引根。参考仅为只读资料，不复制其凭据、历史授权或无关产品决定；复用代码需核对许可证及来源。

## 文件位置

| 内容 | 位置与 Git 处理 |
| --- | --- |
| 本机资料、草稿、迁移备份 | .local/，忽略 |
| 真实配置 | .env.local、secrets/，忽略 |
| 抓包、录像、日志、构建证据 | captures/、recordings/、artifacts/，默认忽略 |
| 合成用例、脱敏证据 | tests/ 或 docs/evidence/，审查后跟踪 |
| 标准原始文件 | workspace 集中维护与校验；产品副本在 .local/standards/ |

`.gitignore` 不保护已跟踪文件；单机额外规则可用 .git/info/exclude。不用 assume-unchanged 隐藏秘密。发现凭据泄漏先停止传播并处理撤销，不能只补忽略规则。

清理前检查 `git status --short --ignored`、`git ls-files` 与忽略原因；不打印真实配置内容。移动前核对来源/目标绝对路径和文件归属，重要原始资料校验哈希；不自动执行 git clean -fdx，不删除用户样本。
