# 第三方依赖与许可

M0 锁定依赖见 go.mod/go.sum 和 web/package.json/pnpm-lock.yaml。实际链接的 Go 模块及本机安装的前端构建/测试依赖列表位于 [inventory.json](third_party/inventory.json)，许可证与 NOTICE 原文按 SHA-256 存于 third_party/licenses。清单保留上游文字，不把项目自己的许可选择套用到依赖。

sipgo 使用 BSD-2-Clause，Wails、icholy/digest、React、Vite 使用 MIT，TypeScript 与 Playwright 使用 Apache-2.0；Go 扩展库、SQLite 驱动及其 libc 包含各自的 BSD 风格和第三方声明，以清单内实际原文为准。Wails 模块 2.15.0 与 latest Release 2.14.0 的差异及构建结果见[技术栈](docs/tech-stack.md)。

清单覆盖本轮 Windows 链接模块和安装的 npm 包；Go 完整模块图中未链接的上游 CLI/代码生成工具不因此视为本产品分发内容。Linux 新增依赖、安装包中的 WebView2/Go Runtime、原生媒体/密码提供器必须在发布时重新核对。MCP SDK 目前仅调研，未加入产品依赖。没有打包 FFmpeg、标准 PDF、用户原图、第三方产品代码或真实测试资料。

本轮没有发布产品或安装包，也没有替用户选择本仓库源码的开源许可证。构建工具版本、兼容结果和不支持项见[运行合同](docs/delivery/m0-implementation.md)。
