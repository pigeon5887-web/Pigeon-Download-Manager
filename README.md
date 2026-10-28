# Pigeon Download Manager

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue.svg)](https://golang.org/)
[![Platform](https://img.shields.io/badge/Platform-Windows-lightgrey.svg)]()

一个基于 Go 语言开发的、模块化且可扩展的**多网盘命令行下载管家**。

## 核心特性

*   **纯 Go 驱动**：核心调度逻辑完全由 Go 编写，无需安装 Node.js 或 Python 环境。
*   **高扩展性架构**：内置 `Parser` 接口，目前支持蓝奏云、百度网盘，并可随时接入 `payload_jumper` 等更多底层解析引擎。
*   **无感登录**：利用 `chromedp` 协议，自动唤起本地 Chrome/Edge/Brave 浏览器扫码，静默提取 HttpOnly Cookie。
*   **安全凭证存储**：采用 `AES-256-GCM` 加密本地凭证，彻底抛弃明文 Cookie，保护账号隐私。
*   **序列号多选下载**：独家定制的增强命令行，支持 `download 1,2,3` 按序号批量下载，告别手敲冗长文件名。
*   **双擎自动调度**：后台自动管理并静默拉起 `aria2c` 与 `BaiduPCS-Go` 引擎，退出程序时自动清理端口，即用即走。

## 快速开始

1. 前往本仓库的 **Releases** 页面，下载最新的 `Pigeon-v1.0.0.zip`。
2. 将压缩包解压到任意文件夹。
3. 双击运行 `pigeon.exe`（如遇 Windows Defender 拦截，请点击“更多信息” -> “仍要运行”）。
4. 根据终端菜单提示操作即可（首次使用百度网盘会自动弹出干净的浏览器供扫码）。

## 从源码编译

确保你的电脑已安装 Go 1.21 或更高版本。

    git clone https://github.com/pigeon5887-web/pigeon-download-manager.git
    cd pigeon-download-manager
    go build -o pigeon.exe

## 第三方组件声明

本项目的诞生离不开以下优秀开源项目的支持，在此向原作者致敬：

*   [chromedp](https://github.com/chromedp/chromedp) (MIT License) - 浏览器自动化控制
*   [aria2](https://github.com/aria2/aria2) (GPL-2.0 License) - 多线程下载引擎
*   [BaiduPCS-Go](https://github.com/qjfoidnh/BaiduPCS-Go) (Apache-2.0 License) - 百度网盘命令行客户端
*   [netdisk-fast-download](https://github.com/qaiu/netdisk-fast-download) (MIT License) - 多网盘直链解析服务

## 免责声明

本工具仅供个人学习、交流及技术研究使用，请勿用于任何商业用途。使用本工具下载的文件请遵守相关网盘的服务条款及版权规定。因使用本工具产生的任何纠纷与作者无关。

## License
本项目采用 [MIT](LICENSE) 协议开源。
