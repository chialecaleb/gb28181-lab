# Git 与 GitHub 协作

## 仓库与版本

主分支 `main` 为经检查、合并的最新实现。workspace、hub、lab 独立提交和维护远程，无 submodule、gitlink 或组合版本锁。产品的依赖仍需锁文件、BOM 或明确版本，标准仍按适用版本引用；“最新实现”不等于每次安装浮动依赖。

初次建仓只提交空 README；既有远端历史不重建。本轮文档、脚本和 CI 的 bootstrap 可在保护启用前集中提交并推送 main，验证 CI 后启用保护；这个例外到初始化结束即失效，不作为未来直推授权。

## 本地工作

默认一个明确阶段一个任务分支和一个交付提交，例如 `docs/foundation`、`feat/register`。未推送的同阶段修复可 amend，前提是确认归属；已推送修复追加提交，不自动强推或改写他人历史。不同产品分别提交，跨仓库任务在报告记录各自分支、提交和依赖顺序。

开始执行 `git status --short --branch`；Git 命令在正确仓库执行，跨仓库使用 `git -C <repo>`。暂存前检查差异和新文件归属，提交前检查 `git diff --cached --stat`、`git diff --cached --check` 与实际内容。不要把不相关或私有文件一起提交。

提交消息用 `feat`、`fix`、`docs`、`refactor`、`test`、`ci`、`build`、`chore` 描述最终结果；不强加未经决定的 DCO 或密码签名。工作树与暂存内容不一致时，工作树通过不能冒充提交版本通过。阶段内检查点按恢复需要建立，远端交付应具备完整证据。

## PR 与主分支保护

[main.json](../../.github/rulesets/main.json)为期望规则；远端实际设置须核验，文件存在不等于生效。

| 设置 | 规则 |
| --- | --- |
| main 更新 | PR 合并；初始化结束后禁止日常直推 |
| 删除与强推 | 禁止 |
| 历史 | 线性，PR 只允许 squash |
| 强制审批人数 | 0，适合单维护者；自审不称独立审批 |
| Review 讨论 | 必须解决 |
| 必需检查 | `repository-quality`，绑定 GitHub Actions App |
| 更新到最新 main | 不强制每次 rebase；有相关冲突或依赖变化时仍须更新验证 |
| 绕过者 | 无常驻 bypass，包括管理员 |

推送、创建 PR、合并、发布依照当前任务已有授权，已授权不重复询问；不得把文档规则当作永久外部操作授权。需要新增授权时先完成可评审成果，写入用户事项，继续无依赖工作。禁止自动撤销保护绕过失败，不自动发布版本。

## CI 与低频操作

[CI](../../.github/workflows/ci.yml)只对面向 main 的 PR 和手动触发运行，无路径过滤，固定 job `repository-quality`。初始化通过手动运行产生真实状态后启用 Ruleset。一个 Ubuntu runner、15 分钟上限、只读 contents、checkout 不保留凭据、Action 固定完整 SHA；同一 PR 新运行取消旧运行。合并后的 main push 不重复跑同一批检查。

本地先验证、修复并集中推送；不通过反复 push 调试，不自动评论或安装 bot。远端请求有界、串行处理变更，写请求间至少一秒；遇到权限、限流或风控错误先诊断并停止相关写操作，不切换账户规避。检查名变更先让新检查成功产出，再更新保护。

本地只设置仓库级 `core.autocrlf=false`、`pull.ff=only`、`fetch.prune=true`，不改全局身份。仓库默认 squash 并清理合并后的分支；具体期望设置见[repository-settings.json](../../.github/repository-settings.json)。安全报告见[SECURITY](../../SECURITY.md)。
