# Pigeon Download Manager

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue.svg)](https://golang.org/)
[![Platform](https://img.shields.io/badge/Platform-Windows-lightgrey.svg)]()

一个基于 Go 语言开发的、模块化且可扩展的**多网盘命令行下载管家**。

## 核心特性

*   **纯 Go 驱动**：核心调度逻辑完全由 Go 编写，无需安装 Node.js 或 Python 环境。
*   **高扩展性架构**：内置 `Parser` 接口，目前支持蓝奏云、百度网盘
*   **无感登录**：利用 `chromedp` 协议，自动唤起本地 Chrome 浏览器扫码，静默提取 HttpOnly Cookie。
*   **双擎自动调度**：后台自动管理并静默拉起 `aria2c` 与 `BaiduPCS-Go` 引擎，退出程序时自动清理端口，即用即走。

## 快速开始

1. 前往本仓库的 **Releases** 页面，下载最新的 `Pigeon-v1.0.0.zip`。
2. 将压缩包解压到任意文件夹（**请确保所有文件都在同一目录，不要单独移动 exe**）。
3. 双击运行 `pigeon.exe`。
4. **注意**：首次运行如遇 Windows Defender 或杀毒软件拦截，请点击“更多信息” -> “仍要运行”（个人开发者无代码签名，属正常现象）。

## 使用指南

程序启动后，会自动在后台拉起下载引擎和解析服务，随后弹出主菜单：

### 1. 解析蓝奏云等链接
- 输入选项 `1`。
- 粘贴你的蓝奏云分享链接并回车（支持带密码的链接）。
- 程序会列出文件列表，输入对应的序号即可自动开始下载。
- 文件默认保存在 `pigeon.exe` 所在目录的 `downloads` 文件夹内。

### 2. 进入百度网盘交互模式
- 输入选项 `2`。
- **首次使用**：程序会弹出一个干净的浏览器，请**扫码登录**你的百度网盘。登录成功后浏览器会自动关闭，并生成 `baidu_cookie.txt` 保存凭证。下次使用无需再次扫码。
- 登录成功后，你将进入 `BaiduPCS-Go` 原生命令行模式，支持以下指令：
    - `ls`：查看当前目录下的文件列表。
    - `cd <目录名>`：进入指定文件夹。
    - `download <文件名>`：下载文件。
        - `-p <线程数>`: 多线程下载，最佳线程目测为4。
    - `exit`：退出百度网盘模式。

### 3. 退出程序
- 在主菜单输入 `q`。程序会自动清理并关闭后台的 Aria2 和 Java 解析服务，释放端口。

## 从源码编译

确保你的电脑已安装 Go 1.21 或更高版本。

    git clone https://github.com/pigeon5887-web/pigeon-download-manager.git
    cd pigeon-download-manager
    go build -o pigeon.exe

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