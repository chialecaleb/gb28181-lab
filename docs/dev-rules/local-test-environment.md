# 仅用于本地的测试环境

本页说明环境准备工具、Go 运行时与受控外测的不同配置用途。执行计划见 [M0](../delivery/m0-plan.md)，历史准备结果见[准备报告](../evidence/m0-preparation.md)，当前结果见[实施报告](../evidence/m0-implementation.md)。

## 位置与加载约定

| 文件/变量 | 用途 |
| --- | --- |
| 仓库根 .env.example | 可提交的键名与合成示例，不自动载入为运行配置 |
| 仓库根 .env、.env.local | 仅本地配置，已被忽略；保留已有内容，仅按当前授权修改相应键 |
| 进程环境变量 | 最高优先级，可供一次测试覆盖；不修改用户/系统级环境变量 |
| LAB_* | 已由 Go 加载器实现的本地应用配置；准备工具另做独立检查 |
| TEST_LIVEGBS_* | 只供测试阶段适配工具使用；不得注入产品前端或形成产品私有依赖 |
| GB28181_STANDARDS_ROOT | 原始标准资料根，读取原文时由 Agent 使用 |
| GB28181_REFERENCE_ROOT | 本地索引根，默认 reference；索引为根下 gb28181-lab/README.md |
| .local/runtime、artifacts/m0 | 本地运行数据与测试证据，均忽略 |

优先级：进程环境变量 > .env.local > .env。显式空值也是覆盖，不用“空值回退”意外复用凭据。文件采用 UTF-8（允许 BOM）、单行 KEY=VALUE，可加一对外层单/双引号；仅整行 # 注释。没有 export、多行、插值、反斜杠转义或 shell 执行；行内 # 和 `$` 都是字面值。文件内重复键/未知键报错。Windows 路径可用正斜杠。

Python 准备工具与 Go 运行时分别实现上述约定并各有独立测试，不能 source/点执行 dotenv，不能自动把 TEST_* 升格为产品配置。启动产品只读取其所需白名单，不把整份环境交给 worker/前端。

## 本地端口与网络边界

本次规划使用回环 IPv4；该专用准备工具拒绝 DNS 名、非回环与 IPv6，避免扩大本地测试范围。这是本次工具的范围，不缩减最终产品的 IPv6/远程接入规划。

| 用途 | 默认端口 | 检查方式 |
| --- | --- | --- |
| Lab HTTP | 18081/TCP | 临时 bind 后关闭 |
| Lab 设备 SIP | 15061/UDP+TCP | 临时 bind 后关闭 |
| Lab 内置平台 SIP | 15062/UDP+TCP | 临时 bind 后关闭，避开第三方测试平台 |
| Vite 开发服务 | 5173/TCP | 临时 bind 后关闭；不是生产服务器 |
| 外部测试目标 | 由 TEST_LIVEGBS_HTTP_URL 与 SIP_HOST/PORT 指定 | 准备阶段只检查 Web 端口 TCP 可连接；不发 SIP |

不开放防火墙，不修改端口转发，不结束已有进程。暂时可 bind 只表示检查瞬间可用，应用启动仍必须检查占用。外部 SIP 的账号、密码、realm 和认证模式不能由 HTTP 地址或 Web 密码推导。

## 已可执行的命令

在 Lab 仓库根运行：

```powershell
python scripts/local_env.py
python scripts/local_env.py --probe
python scripts/verify.py
```

第一条仅解析、校验、报告是否配置以及路径是否存在，不读取参考代码内容。第二条额外对已验证回环的 Web 端口做一次最长 2 秒 TCP 连接和本地端口临时 bind，立即释放；不发送 HTTP/SIP/凭据，不登录、不抓取 API 文档、不修改平台。退出码 0 表示该检查通过，1 表示连接/bind 失败，2 表示配置无效。SIP_AUTH_MODE=unknown 明示待发现，配置结构通过并不意味外部 SIP 就绪。

新机器可手工复制 .env.example 为 .env.local 并填本地值，已有配置不要覆盖。远端/CI 使用合成临时输入的单元测试，不接入个人 LiveGBS。Lab 已有 HTTP/CLI/Wails 入口，实际构建和启动见[运行合同](../delivery/m0-implementation.md)。

## 测试阶段使用 LiveGBS

只有基础实现和独立检查完成后，才进入 [M0-P6](../delivery/m0-plan.md) 的外部测试。此时按本地索引读取指定的 API 文档，必要时只读查看指定前端中的相关请求，以获取测试所需信息。LiveGBS 不能用于开发参考；测试发现的不兼容要回到标准定位，不能直接照搬其实现。

测试参数保存在 TEST_LIVEGBS_HTTP_URL、USERNAME/PASSWORD、SIP_HOST/PORT/ID/REALM、SIP_AUTH_MODE/USERNAME/PASSWORD、SOURCE_ROOT、API_DOC_URL。unknown 不等于 none；外测前通过已授权只读来源核实实际 Digest 参数。显式 TEST_LIVEGBS_SIP_PASSWORD 优先，只有缺少该键时才可使用授权发现值；空值和掩码值不能回退到另一份凭据。发现的 ID、端口与已填写 realm 必须和授权目标一致，不从 Web 密码推导 SIP 密码。

外测预算为一个合成设备、一个通道、零媒体、单次不超过 10 分钟；不自动压测、不改全局设置、不操作原有设备。记录测试资源归属、正常时钟、版本与四信令证据，结束注销并仅清理本次产生的资源。不得将登录成功/目录管理 API 返回值替代真实 SIP 互通。

凭据不使用 VITE_ 前缀，不写公共文档/日志/截图/报告。Authorization、Cookie、令牌、Digest 原始敏感数据需脱敏；原始测试证据在本地受控保存。`.gitignore` 不保护已经跟踪的文件，提交前仍检查暂存清单和差异。

## M0 应用运行补充

TEST_* 仅由外测脚本读取，不进入运行时配置、Vite 或 API。产品可选外部绑定文件、访问令牌与实际命令见[M0 实现合同](../delivery/m0-implementation.md)。模式保存在 Config.protocolMode；外测由 `--mode standard` 或 `--mode compatibility` 显式选择，不把某台机器的兼容偏好写成应用默认值。

外测工具仅访问已验证回环目标，不跟随 HTTP 重定向或代理，限制响应体、等待与会话寿命。异常时协作停止自己启动的测试进程，确认本次设备离线后才移除，清理未知状态即失败。原始输入/SIP/API 证据在受保护的 artifacts 子目录；仅经过审查的逐事务摘要进入[兼容台账](../evidence/compatibility-runs.json)。结果和本轮实际模式差异见[实施证据](../evidence/m0-implementation.md)，不在公共文档保存本机平台配置。
