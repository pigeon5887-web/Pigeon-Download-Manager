# Pigeon Download Manager V1.1.5

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue.svg)](https://golang.org/)
[![Platform](https://img.shields.io/badge/Platform-Windows-lightgrey.svg)]()

一个基于 Go 语言开发的、模块化且可扩展的**多网盘命令行下载管家**。

## 核心特性

*   **纯 Go 驱动**：核心调度逻辑完全由 Go 编写，无需安装 Node.js 或 Python 环境。
*   **无感登录与安全加密**：支持多浏览器（Edge / Chrome / Brave）唤起扫码，静默提取 HttpOnly Cookie。凭证采用 **AES-256-GCM** 高强度加密存储，保护账号隐私。
*   **序列号增强下载**：独家定制的增强命令行，支持 `cd <序号>` 进目录、`download <序号1,序号2>` 多选下载，告别手敲长文件名。
*   **双擎自动调度**：后台自动管理并静默拉起 `aria2c` 与 `BaiduPCS-Go` 引擎，退出程序时自动清理端口，即用即走。
*   **高扩展性架构**：内置 `Parser` 接口，目前支持蓝奏云、百度网盘，可随时接入更多底层解析引擎。

## 快速开始

1. 前往本仓库的 **Releases** 页面，下载最新的 `Pigeon-v1.1.5-Setup.exe` 安装包。
2. 双击运行安装程序，按照指引完成安装（建议使用默认路径）。
3. 安装完成后，双击桌面快捷方式或开始菜单中的 `Pigeon Download Manager` 运行。
4. **注意**：首次运行如遇 Windows Defender 或杀毒软件拦截，请点击“更多信息” -> “仍要运行”（个人开发者无代码签名，属正常现象）。

## 使用指南

程序启动后，会自动在后台拉起下载引擎和解析服务，随后弹出主菜单：

### 1. 解析蓝奏云等链接
- 输入选项 `1`。
- 粘贴你的蓝奏云分享链接并回车（支持带密码的链接）。
- 程序会列出文件列表，输入对应的序号即可自动开始下载。
- 文件默认保存在 `pigeon_disk_download.exe` 所在目录的 `downloads` 文件夹内。

### 2. 进入百度网盘交互模式
- 输入选项 `2`。
- **首次使用**：程序会弹出一个干净的浏览器（可选择 Edge/Chrome/Brave），请**扫码登录**你的百度网盘。登录成功后浏览器会自动关闭，并将凭证**加密保存**为 `baidu_cookie.pigeon`。下次使用无需再次扫码。
- 登录成功后，你将进入 `百度网盘命令行`，支持以下指令：
    - `ls`：查看当前目录下的文件列表。
    - `cd <序号>` 或 `cd "目录名"`：按序号或真实名称进入文件夹。
    - `download <序号1,序号2>`：按序号多选下载文件。
    - `download <序号> -p <线程数>`：多线程下载，最佳线程目测为 4，如 `download 1,2 -p 4`。
    - `quit` 或 `exit`：退出百度网盘模式。

### 3. 退出程序
- 在主菜单输入 `q`。程序会自动静默清理并关闭后台的 Aria2 和 Java 解析服务，释放端口。

## 从源码编译

确保你的电脑已安装 Go 1.21 或更高版本。

    git clone https://github.com/pigeon5887-web/pigeon-download-manager.git
    cd pigeon-download-manager
    go build -o pigeon_disk_download.exe

## 第三方组件声明

本项目的诞生离不开以下优秀开源项目的支持，在此向原作者致敬。完整的许可证文本和版权声明详见 [THIRD_PARTY_LICENSES](./THIRD_PARTY_LICENSES) 目录：

*   [chromedp](https://github.com/chromedp/chromedp) (MIT License) - 浏览器自动化控制
*   [aria2](https://github.com/aria2/aria2) (GPL-2.0 License) - 多线程下载引擎
*   [BaiduPCS-Go](https://github.com/qjfoidnh/BaiduPCS-Go) (Apache-2.0 License) - 百度网盘命令行客户端
*   [netdisk-fast-download](https://github.com/qaiu/netdisk-fast-download) (MIT License) - 多网盘直链解析服务

## 免责声明

本工具仅供个人学习、交流及技术研究使用，请勿用于任何商业用途。使用本工具下载的文件请遵守相关网盘的服务条款及版权规定。因使用本工具产生的任何纠纷与作者无关。

## License
本项目采用 [MIT](LICENSE) 协议开源。