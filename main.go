package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// 启动 Aria2
func startAria2Daemon() {
	conn, err := net.Dial("tcp", "127.0.0.1:6800")
	if err == nil {
		conn.Close()
		fmt.Println(ColorGreen + "[ok] 主人~检测到 Aria2 已在后台运行了喵" + ColorReset)
		return
	}

	fmt.Println("[+] 主人~未检测到 Aria2，正在后台静默启动喵...")
	cmd := exec.Command("./aria2c.exe",
		"--enable-rpc", "--rpc-listen-all=false", "--rpc-listen-port=6800",
		"--rpc-secret=mytoken", "--rpc-allow-origin-all", "--file-allocation=none",
	)
	cmd.SysProcAttr = getSysProcAttr()

	if err := cmd.Start(); err != nil {
		fmt.Printf("%s[error] 启动 Aria2 失败喵: %v%s\n", ColorRed, err, ColorReset)
		return
	}
	time.Sleep(1 * time.Second)
	fmt.Println(ColorGreen + "[ok] Aria2 已在后台启动喵~！" + ColorReset)
}

// 启动 NFD 解析服务
func startNfdDaemon() {
	conn, err := net.Dial("tcp", "127.0.0.1:6400")
	if err == nil {
		conn.Close()
		fmt.Println(ColorGreen + "[ok] 主人~检测到 netdisk-fast-download 已在后台运行了喵~" + ColorReset)
		return
	}

	fmt.Println("[+] 未检测到蓝奏云解析服务，正在后台静默启动喵~...")
	cmd := exec.Command("cmd", "/c", "start", "/b", "run.bat")
	cmd.Dir = "netdisk-fast-download"
	cmd.SysProcAttr = getSysProcAttr()

	if err := cmd.Start(); err != nil {
		fmt.Printf("%s[error] 启动 NFD 服务失败喵: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	fmt.Println("[+] 请好好等待解析服务加载哦！喵~...")
	time.Sleep(3 * time.Second)

	conn, err = net.Dial("tcp", "127.0.0.1:6400")
	if err == nil {
		conn.Close()
		fmt.Println(ColorGreen + "[ok] 蓝奏云解析服务已启动喵~！" + ColorReset)
	} else {
		fmt.Println(ColorRed + "[error] 主人，服务启动似乎有些慢呢，请检查是否有 Java 环境喵" + ColorReset)
	}
}

func main() {
	// 使用 defer 保证退出时静默清理，但不会打断正常运行
	defer cleanupServices()

	// 加载下载路径等本地配置
	loadConfig()

	startAria2Daemon()
	startNfdDaemon()

	// ★ 核心修改：吞掉 Ctrl+C 信号，防止父进程被直接杀死
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range c {
			// 收到信号，什么都不做。让子进程去处理（比如打断下载）。
		}
	}()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\n主人~欢迎喏！请选择你要的网盘，开始今天的工作！")
		fmt.Println("========================================")
		fmt.Println("1. 解析蓝奏云等链接")
		fmt.Println("2. 进入百度网盘交互模式")
		fmt.Println("3. 解析123云盘链接")
		fmt.Println("4. 解析夸克/UC链接")
		fmt.Println("5. 设置下载路径")
		fmt.Println("q. 退出这里喵")
		fmt.Println("========================================")
		fmt.Print("请选择功能 (1/2/3/4/5/q): ")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(strings.ToLower(choice))

		if choice == "q" {
			fmt.Println("[+] 正在离开喵...")
			break
		}

		switch choice {
		case "1":
			runNfdMode(reader, "蓝奏云等链接")
		case "2":
			runBaiduInteractive(reader)
		case "3":
			runNfdMode(reader, "123云盘")
		case "4":
			runNfdMode(reader, "夸克/UC")
		case "5":
			runDownloadPathMenu(reader)
		default:
			fmt.Println(ColorRed + "[error] 输入无效，主人，请重新选择喵" + ColorReset)
		}
	}
}

// 浏览器提取 Cookie
func getBaiduCookieWithChromedp() (string, error) {
	fmt.Println("[+] 主人~准备借用浏览器去偷曲奇啦喵...")

	fmt.Println("主人，要借用哪个浏览器扫码呢喵？:")
	fmt.Println("1. Microsoft Edge (推荐喵~)")
	fmt.Println("2. Google Chrome")
	fmt.Println("3. Brave Browser")
	fmt.Print("请选择 (默认 1 喵): ")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)
	if choice == "" {
		choice = "1"
	}

	var execPath string
	if choice == "1" {
		execPath = `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`
		if _, err := os.Stat(execPath); os.IsNotExist(err) {
			execPath = `C:\Program Files\Microsoft\Edge\Application\msedge.exe`
		}
	} else if choice == "2" {
		execPath = `C:\Program Files\Google\Chrome\Application\chrome.exe`
		if _, err := os.Stat(execPath); os.IsNotExist(err) {
			execPath = `C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`
		}
	} else if choice == "3" {
		execPath = `C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe`
		if _, err := os.Stat(execPath); os.IsNotExist(err) {
			execPath = `C:\Program Files (x86)\BraveSoftware\Brave-Browser\Application\brave.exe`
		}
	}

	if execPath != "" {
		if _, err := os.Stat(execPath); os.IsNotExist(err) {
			fmt.Printf("%s[error] 呜呜...没找到主人选的那个浏览器呢，是不是没安装喵？%s\n", ColorRed, ColorReset)
			execPath = ""
		}
	}

	currentDir, _ := os.Getwd()
	userDataDir := filepath.Join(currentDir, "user-data-baidu-chromedp")

	opts := chromedp.DefaultExecAllocatorOptions[:]
	if execPath != "" {
		opts = append(opts, chromedp.ExecPath(execPath))
	}
	opts = append(opts,
		chromedp.UserDataDir(userDataDir),
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()

	ctx, cancelCtx := chromedp.NewContext(allocCtx,
		chromedp.WithErrorf(func(format string, args ...interface{}) {
			msg := fmt.Sprintf(format, args...)
			if strings.Contains(msg, "could not unmarshal event") || strings.Contains(msg, "unknown ClientNavigationReason") {
				return
			}
		}),
	)
	defer cancelCtx()

	fmt.Println("\n========================================================")
	fmt.Println("[!] 主人！快在弹出的浏览器里扫码登录喵~")
	fmt.Println("========================================================")

	var cookieStr string
	err := chromedp.Run(ctx,
		network.Enable(),
		chromedp.Navigate("https://pan.baidu.com/disk/main"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			for i := 0; i < 300; i++ {
				cookies, err := network.GetCookies().Do(ctx)
				if err != nil {
					return err
				}

				var bduss, stoken string
				for _, c := range cookies {
					if strings.Contains(c.Domain, "baidu.com") {
						if c.Name == "BDUSS" {
							bduss = c.Value
						}
						if c.Name == "STOKEN" {
							stoken = c.Value
						}
					}
				}

				if bduss != "" {
					cookieStr = fmt.Sprintf("BDUSS=%s; STOKEN=%s", bduss, stoken)
					return nil
				}
				time.Sleep(1 * time.Second)
			}
			return fmt.Errorf("等主人扫码等到睡着啦喵...")
		}),
	)

	if err != nil {
		return "", fmt.Errorf("偷曲奇失败喵: %v", err)
	}

	time.Sleep(2 * time.Second)
	fmt.Println(ColorGreen + "[ok] 成功把曲奇偷出来啦~耶嘿！" + ColorReset)
	return cookieStr, nil
}

// 检查并登录
func checkAndLoginBaidu(reader *bufio.Reader) bool {
	currentDir, _ := os.Getwd()
	cookieFile := filepath.Join(currentDir, "baidu_cookie.pigeon")

	if fileData, err := os.ReadFile(cookieFile); err == nil {
		decrypted, err := DecryptCookie(string(fileData))
		if err == nil && decrypted != "" {
			fmt.Println("[+] 发现藏着加密曲奇~本喵正在偷吃喵...")
			loginCmd := exec.Command("./BaiduPCS-Go.exe", "login", "-cookies="+decrypted)
			loginCmd.Stdout = os.Stdout
			loginCmd.Stderr = os.Stderr
			loginCmd.Run()

			verifyCmd := exec.Command("./BaiduPCS-Go.exe", "who")
			verifyOut, _ := verifyCmd.CombinedOutput()
			if !strings.Contains(string(verifyOut), "uid: 0") {
				fmt.Println(ColorGreen + "[ok] 靠本地藏起来的曲奇就混进去啦~" + ColorReset)
				return true
			}
			fmt.Println(ColorRed + "[!] 哎呀，本地曲奇过期被吃光啦，得重新找喵~" + ColorReset)
		} else {
			fmt.Println(ColorRed + "[!] 坏了！曲奇被人撕坏了(解密失败)，要重新拿喵~" + ColorReset)
		}
	}

	fmt.Println("[+] 正在检查百度网盘登录状态喵...")
	cmd := exec.Command("./BaiduPCS-Go.exe", "who")
	output, _ := cmd.CombinedOutput()
	outputStr := string(output)

	if strings.Contains(outputStr, "uid: 0") || strings.Contains(outputStr, "用户名: ,") {
		fmt.Println(ColorRed + "[!] 主人，还没登录或者凭证过期了喵" + ColorReset)

		cookieStr, err := getBaiduCookieWithChromedp()

		if err == nil {
			encrypted, encErr := EncryptCookie(cookieStr)
			if encErr == nil {
				writeErr := os.WriteFile(cookieFile, []byte(encrypted), 0644)
				if writeErr == nil {
					fmt.Printf("%s[ok] 曲奇已经上锁藏在这里啦: %s%s\n", ColorGreen, cookieFile, ColorReset)
				} else {
					fmt.Printf("%s[error] 呜呜，写文件失败了: %v%s\n", ColorRed, writeErr, ColorReset)
				}
			} else {
				fmt.Printf("%s[error] 加密失败喵: %v%s\n", ColorRed, encErr, ColorReset)
			}

			fmt.Println("[+] 正在把曲奇喂给 BaiduPCS-Go 吃喵...")
			loginCmd := exec.Command("./BaiduPCS-Go.exe", "login", "-cookies="+cookieStr)
			loginCmd.Stdout = os.Stdout
			loginCmd.Stderr = os.Stderr
			loginCmd.Run()
		} else {
			fmt.Println(ColorRed + "[!] 自动找曲奇失败喵: " + err.Error() + ColorReset)

			fmt.Print("[?] 主人~请给本喵喂 BDUSS 吧 (直接回车跳过喵): ")
			inputBDUSS, _ := reader.ReadString('\n')
			inputBDUSS = strings.TrimSpace(inputBDUSS)
			if inputBDUSS == "" {
				fmt.Println(ColorRed + "[error] 呜呜，主人不要本喵了，取消登录喵..." + ColorReset)
				return false
			}

			fmt.Print("[?] 再给本喵喂 STOKEN 吧 (直接回车跳过喵): ")
			inputSTOKEN, _ := reader.ReadString('\n')
			inputSTOKEN = strings.TrimSpace(inputSTOKEN)

			inputCookie := fmt.Sprintf("BDUSS=%s; STOKEN=%s", inputBDUSS, inputSTOKEN)

			encrypted, encErr := EncryptCookie(inputCookie)
			if encErr == nil {
				writeErr := os.WriteFile(cookieFile, []byte(encrypted), 0644)
				if writeErr == nil {
					fmt.Printf("%s[ok] 手动喂的曲奇藏在这里啦: %s%s\n", ColorGreen, cookieFile, ColorReset)
				} else {
					fmt.Printf("%s[error] 呜呜，写文件失败了: %v%s\n", ColorRed, writeErr, ColorReset)
				}
			} else {
				fmt.Printf("%s[error] 加密失败喵: %v%s\n", ColorRed, encErr, ColorReset)
			}

			loginCmd := exec.Command("./BaiduPCS-Go.exe", "login", "-cookies="+inputCookie)
			loginCmd.Stdout = os.Stdout
			loginCmd.Stderr = os.Stderr
			loginCmd.Run()
		}

		verifyCmd := exec.Command("./BaiduPCS-Go.exe", "who")
		verifyOut, _ := verifyCmd.CombinedOutput()
		if strings.Contains(string(verifyOut), "uid: 0") {
			fmt.Println(ColorRed + "\n[error] 呜，登录没成功，或者主人中途跑掉了喵" + ColorReset)
			return false
		}
		fmt.Println(ColorGreen + "\n[ok] 耶！主人登录成功啦~！" + ColorReset)
		return true
	}

	fmt.Println(ColorGreen + "[ok] BaiduPCS-Go 还认得主人呢，不用重新登录喵" + ColorReset)
	return true
}

// 提取 ls 输出的文件名
func parseBaiduList(out string) []string {
	re := regexp.MustCompile(`\d{4}-\d{2}-\d{2}\s\d{2}:\d{2}:\d{2}\s+(.*)`)
	lines := strings.Split(out, "\n")
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "文件大小") || strings.Contains(line, "----") {
			continue
		}
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			files = append(files, strings.TrimSpace(matches[1]))
		}
	}
	return files
}

// 百度网盘交互模式
func runBaiduInteractive(reader *bufio.Reader) {
	if !checkAndLoginBaidu(reader) {
		return
	}

	// 下载路径走全局配置，防止软件在 C:\Windows 这类地方时写不进去
	downloadDir := appConfig.DownloadDir
	exec.Command("./BaiduPCS-Go.exe", "config", "set", "-savedir", downloadDir).Run()

	fmt.Println(ColorCyan + "\n[+] 认证完成主人！正在进入百度网盘命令行..." + ColorReset)
	fmt.Println("[+] ls -> 查看内容")
	fmt.Println("[+] cd -> 进入文件夹 | cd <\"文件夹\">/cd <序号>")
	fmt.Println("[+] download ->下载 |download <序号1,序号2>")
	fmt.Println("    [+]-p -> 多线程|download -p <线程数> <序号1,序号2>")
	fmt.Println("========================================")

	for {
		fmt.Print("baidu-disk > ")
		cmdLine, err := reader.ReadString('\n')

		//处理 Ctrl+C 或 EOF 导致的读取错误，防止死循环刷屏
		if err != nil {
			fmt.Println() // 打印一个换行，美观
			break         // 直接退出当前模式
		}

		cmdLine = strings.TrimSpace(cmdLine)

		if cmdLine == "" {
			continue
		}

		lowerCmd := strings.ToLower(cmdLine)
		if lowerCmd == "exit" || lowerCmd == "quit" || lowerCmd == "q" {
			break
		}

		// 1. 原生透传 ls
		if lowerCmd == "ls" {
			cmd := exec.Command("./BaiduPCS-Go.exe", "ls")
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			_ = cmd.Run()
			continue
		}

		// 获取文件列表的闭包函数
		getList := func() []string {
			fmt.Println("[+] 正在后台静默获取文件列表喵...")
			out, err := exec.Command("./BaiduPCS-Go.exe", "ls").Output()
			if err != nil {
				return nil
			}
			re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
			cleanOut := re.ReplaceAllString(string(out), "")
			return parseBaiduList(cleanOut)
		}

		// 2. 处理 cd 命令
		if strings.HasPrefix(lowerCmd, "cd ") {
			arg := strings.TrimSpace(cmdLine[3:])

			if (strings.HasPrefix(arg, "\"") && strings.HasSuffix(arg, "\"")) || (strings.HasPrefix(arg, "'") && strings.HasSuffix(arg, "'")) {
				realPath := strings.Trim(arg, "\"'")
				cmd := exec.Command("./BaiduPCS-Go.exe", "cd", realPath)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				_ = cmd.Run()
			} else if _, err := strconv.Atoi(arg); err == nil {
				currentFiles := getList()
				if currentFiles == nil {
					continue
				}

				idx, _ := strconv.Atoi(arg)
				if idx < 0 || idx >= len(currentFiles) {
					fmt.Printf("%s[error] 序号 %d 无效喵！%s\n", ColorRed, idx, ColorReset)
					continue
				}
				target := currentFiles[idx]

				cmd := exec.Command("./BaiduPCS-Go.exe", "cd", target)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				_ = cmd.Run()
			} else {
				cmd := exec.Command("./BaiduPCS-Go.exe", "cd", arg)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				_ = cmd.Run()
			}
			continue
		}

		// 3. 处理 download 命令（支持带参数）
		if strings.HasPrefix(lowerCmd, "download ") {
			args := strings.TrimSpace(cmdLine[9:])

			if (strings.HasPrefix(args, "\"") && strings.HasSuffix(args, "\"")) || (strings.HasPrefix(args, "'") && strings.HasSuffix(args, "'")) {
				realPath := strings.Trim(args, "\"'")
				cmd := exec.Command("./BaiduPCS-Go.exe", "download", realPath)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				_ = cmd.Run()
			} else {
				parts := strings.Fields(args)
				var indexStr string
				var extraArgs []string

				for _, p := range parts {
					if strings.HasPrefix(p, "-") || (len(extraArgs) > 0 && strings.HasPrefix(extraArgs[len(extraArgs)-1], "-")) {
						extraArgs = append(extraArgs, p)
					} else {
						indexStr += p
					}
				}

				indexStr = strings.TrimSuffix(indexStr, ",")

				isIndex := true
				for _, char := range indexStr {
					if !strings.Contains("0123456789,", string(char)) {
						isIndex = false
						break
					}
				}

				if isIndex && indexStr != "" {
					currentFiles := getList()
					if currentFiles == nil {
						continue
					}

					indexStr = strings.ReplaceAll(indexStr, "，", ",")
					parts := strings.Split(indexStr, ",")
					var realFileNames []string
					valid := true

					for _, p := range parts {
						idx, err := strconv.Atoi(p)
						if err != nil || idx < 0 || idx >= len(currentFiles) {
							fmt.Printf("%s[error] 序号 %s 无效喵！当前有 %d 个文件/目录。%s\n", ColorRed, p, len(currentFiles), ColorReset)
							valid = false
							break
						}
						realFileNames = append(realFileNames, currentFiles[idx])
					}

					if !valid {
						continue
					}

					downloadArgs := []string{"download"}
					downloadArgs = append(downloadArgs, realFileNames...)
					downloadArgs = append(downloadArgs, extraArgs...)

					fmt.Printf("[+] 主人~正在下载选中的 %d 项喵，附带参数: %v\n", len(realFileNames), extraArgs)

					cmd := exec.Command("./BaiduPCS-Go.exe", downloadArgs...)
					cmd.Stdin = os.Stdin
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					_ = cmd.Run()
				} else {
					cmdParts := append([]string{"download", args}, extraArgs...)
					cmd := exec.Command("./BaiduPCS-Go.exe", cmdParts...)
					cmd.Stdin = os.Stdin
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					_ = cmd.Run()
				}
			}
			continue
		}

		// 4. 其他命令
		cmdParts := strings.Fields(cmdLine)
		if len(cmdParts) > 0 {
			cmd := exec.Command("./BaiduPCS-Go.exe", cmdParts...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			_ = cmd.Run()
		}
	}

	fmt.Println(ColorGreen + "\n[ok] 已离开百度网盘命令行喵" + ColorReset)
}

// 网盘链接解析（蓝奏云/123云盘/夸克/UC/移动云等，NFD 按域名自动路由）
// 循环交互：Ctrl+C 返回上级菜单，q 返回上一步
// aria2 的 Referer 按链接域名自动识别（各网盘 CDN 脾气不一样）
func runNfdMode(reader *bufio.Reader, title string) {
	// 防止之前的 Ctrl+C 把后台服务误杀后一直躺着，进模式前先给服务做个体检
	startAria2Daemon()
	startNfdDaemon()

	// 123盘模式：先看一眼账号配没配，没配提前打个招呼（不拦人）
	if title == "123云盘" {
		if user, _, ymlOK := readNfd123Auth(); ymlOK && user == "" {
			fmt.Printf("%s[i] 还没给解析服务配 123 盘账号，等下解析会撞登录墙（到时可选当场配置喵）%s\n", ColorGray, ColorReset)
		}
	}

	// 全程监听 Ctrl+C，方便在解析/推送这种不读键盘的环节里也能及时收手
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stopCh)

	parser := &NfdParser{APIBase: "http://127.0.0.1:6400"}

	for {
		fmt.Printf("\n主人~请输入%s的分享链接喵 (输入 q 或按 Ctrl+C 返回): ", title)
		input, err := reader.ReadString('\n')
		if err != nil {
			// Ctrl+C 或 EOF，返回上级菜单
			fmt.Println()
			return
		}
		input = strings.TrimSpace(input)
		if strings.ToLower(input) == "q" || input == "" {
			return
		}

		pwd := ""
		if u, perr := urlParseHelper(input); perr == nil {
			pwd = u
			// 提取码已单独带走；URL 上的查询串不剥掉会让 NFD 认错网盘类型
			input = stripPwdQuery(input)
		}

		// 目录导航栈：进入文件夹时压栈，u 返回上一层；apiURL 为空 = 根层（用原始链接取列表）
		type dirLevel struct {
			apiURL string
			name   string
		}
		var dirStack []dirLevel
		current := dirLevel{name: "根目录"}
		helpShown := false // 命令表只在首次解析成功后列一次

	navLoop:
		for {
			// ---- 取当前层的文件列表 ----
			var fileList []FileItem
			if current.apiURL == "" {
				listOK := false
				for !listOK {
					fmt.Println("[+] 主人~正在获取文件列表喵...")
					fileList, err = parser.GetFileList(input, pwd)
					if err != nil {
						loginWall := strings.Contains(err.Error(), "登录") || strings.Contains(err.Error(), "认证")
						// "密码/提取码"是缺提取码；带"登录/认证"的是 123 盘登录墙，别混
						needPwd := (strings.Contains(err.Error(), "密码") || strings.Contains(err.Error(), "提取码")) && !loginWall
						if needPwd {
							fmt.Printf("%s[error] %s%s\n", ColorRed, err.Error(), ColorReset)
							fmt.Print("[+] 主人~请输入提取码 (输入 q 返回): ")
							pwdInput, perr := reader.ReadString('\n')
							if perr != nil {
								fmt.Println()
								return
							}
							pwd = strings.TrimSpace(pwdInput)
							if strings.ToLower(pwd) == "q" {
								break navLoop
							}
							continue
						}
						fmt.Printf("%s[error] %s%s\n", ColorRed, err.Error(), ColorReset)
						// 123盘撞登录墙：给指引，并支持当场配置（写 yml + 硬刷新服务后自动重试）
						if is123PanLink(input) && loginWall {
							print123AuthHint()
							if offerNfdAuthSetup(reader) {
								continue // 服务已带新配置重启，重试获取列表
							}
						}
						break navLoop // 换个链接
					}
					listOK = true
				}
			} else {
				fmt.Printf("[+] 正在进入「%s」喵...\n", current.name)
				ref := current.apiURL
				if !strings.HasPrefix(ref, "http") { // 自建导航引用（qkdir:/ucdir:）自带会话，不注入提取码
					ref = injectPwd(ref, pwd)
				}
				subList, subErr := parser.GetFileListVia(ref)
				if subErr != nil {
					fmt.Printf("%s[error] %s%s\n", ColorRed, subErr, ColorReset)
					// 进不去就退回上一层
					if n := len(dirStack); n > 0 {
						current = dirStack[n-1]
						dirStack = dirStack[:n-1]
						continue
					}
					break navLoop
				}
				fileList = subList
			}

			if len(fileList) == 0 {
				fmt.Println("[+] 这个目录是空的喵...")
				if n := len(dirStack); n > 0 {
					current = dirStack[n-1]
					dirStack = dirStack[:n-1]
					continue
				}
				break navLoop
			}

			printNfdFileList(fileList)
			if !helpShown {
				printPanCmdHelp()
				helpShown = true
			}

			// ---- 本层的命令循环（和百度网盘交互模式同款命令风格）----
			// 下载流程：解析直链并推送（arg 为归一化后的 0 / 1,3,5）
			doDownload := func(arg string) {
				indices, parseErr := parseFileSelection(arg, len(fileList))
				if parseErr != nil {
					fmt.Printf("%s[error] %v%s\n", ColorRed, parseErr, ColorReset)
					return
				}

				// 清空之前积压的 Ctrl+C，防止视监时留下的旧信号误伤新一轮解析
			drainSignals:
				for {
					select {
					case <-stopCh:
					default:
						break drainSignals
					}
				}

				var gids []string
				aborted := false
				for n, idx := range indices {
					select {
					case <-stopCh:
						aborted = true
					default:
					}
					if aborted {
						fmt.Printf("%s[!] 主人按了 Ctrl+C，剩下的文件先不解析了喵%s\n", ColorRed, ColorReset)
						break
					}

					f := fileList[idx]
					if f.FileType == "folder" {
						fmt.Printf("%s[!] 跳过文件夹 %s（用 cd %d 进去喵）%s\n", ColorGray, f.FileName, idx+1, ColorReset)
						continue
					}
					fmt.Printf("[+] (%d/%d) 正在解析 %s 的直链喵...\n", n+1, len(indices), f.FileName)
					directLink, dlCookie, dlErr := parser.GetDirectLink(f.FileID)
					if dlErr != nil {
						fmt.Printf("%s[error] %s 解析失败喵: %v%s\n", ColorRed, f.FileName, dlErr, ColorReset)
						continue
					}
					gid, addErr := addDownloadTask(directLink, appConfig.DownloadDir, f.FileName, panReferer(input), dlCookie)
					if addErr != nil {
						fmt.Printf("%s[error] %s 推送 aria2 失败喵: %v%s\n", ColorRed, f.FileName, addErr, ColorReset)
						continue
					}
					fmt.Printf("%s[ok] 已推送 %s (GID: %s)%s\n", ColorGreen, f.FileName, gid, ColorReset)
					gids = append(gids, gid)
				}

				if len(gids) == 0 {
					return
				}

				if len(gids) == 1 {
					checkStatus(gids[0])
				} else {
					checkStatusMulti(gids)
				}
				// 视监结束后回到本层命令行，还能继续挑文件或 cd 去别的文件夹
			}

			// 进入文件夹（idx 为列表下标，0 起算）
			enterFolder := func(idx int) bool {
				f := fileList[idx]
				if f.FileType != "folder" {
					fmt.Printf("%s[error] %s 不是文件夹喵，下载用 download %d%s\n", ColorRed, f.FileName, idx+1, ColorReset)
					return false
				}
				// 夸克走自建导航引用（SubRef），123盘走 NFD 的 parserUrl
				ref := f.SubRef
				if ref == "" {
					ref = f.ParserURL
				}
				if ref == "" {
					fmt.Printf("%s[error] 这个文件夹没有导航地址喵（NFD 版本可能太旧）%s\n", ColorRed, ColorReset)
					return false
				}
				dirStack = append(dirStack, current)
				current = dirLevel{apiURL: ref, name: f.FileName}
				return true
			}

			goUp := func() {
				if n := len(dirStack); n > 0 {
					current = dirStack[n-1]
					dirStack = dirStack[:n-1]
				} else {
					fmt.Println("[i] 已经在最上层了喵")
				}
			}

			for {
				fmt.Printf("\npan[%s] > ", current.name)
				cmdLine, cerr := reader.ReadString('\n')
				if cerr != nil {
					// Ctrl+C 或 EOF，返回上级菜单
					fmt.Println()
					return
				}
				cmdLine = strings.TrimSpace(cmdLine)
				if cmdLine == "" {
					continue
				}
				lower := strings.ToLower(cmdLine)

				switch {
				case lower == "q" || lower == "exit" || lower == "quit":
					break navLoop // 换链接

				case lower == "ls":
					continue navLoop // 重新取当前层列表

				case lower == "u" || lower == "cd ..":
					goUp()
					continue navLoop

				case lower == "cd":
					fmt.Printf("%s[error] 用法：cd <序号> 进文件夹，cd .. 返回上级%s\n", ColorRed, ColorReset)

				case strings.HasPrefix(lower, "cd "):
					arg := strings.TrimSpace(cmdLine[3:])
					if arg == ".." {
						goUp()
						continue navLoop
					}
					idx, aerr := strconv.Atoi(arg)
					if aerr != nil || idx < 1 || idx > len(fileList) {
						fmt.Printf("%s[error] cd 序号无效喵！范围 1-%d，或 cd .. 返回上级%s\n", ColorRed, len(fileList), ColorReset)
						continue
					}
					if enterFolder(idx - 1) {
						continue navLoop
					}

				case lower == "download":
					fmt.Printf("%s[error] 用法：download 0（全部）/ download 1,3,5（多选）%s\n", ColorRed, ColorReset)

				case strings.HasPrefix(lower, "download "):
					doDownload(normalizeSelStr(cmdLine[9:]))

				default:
					// 兼容裸数字：单序号指到文件夹 = 进入，其余 = 下载
					norm := normalizeSelStr(cmdLine)
					if isNumericSel(norm) {
						indices, perr := parseFileSelection(norm, len(fileList))
						if perr != nil {
							fmt.Printf("%s[error] %v%s\n", ColorRed, perr, ColorReset)
							continue
						}
						if len(indices) == 1 && fileList[indices[0]].FileType == "folder" {
							if enterFolder(indices[0]) {
								continue navLoop
							}
							continue
						}
						doDownload(norm)
						continue
					}
					fmt.Printf("%s[error] 不认识的命令喵：ls / cd <序号> / cd .. / download <0或1,3,5> / u / q%s\n", ColorRed, ColorReset)
				}
			}
		}
	}
}

// printNfdFileList 打印文件列表（干净版：文件夹不带大小，文件带大小）
func printNfdFileList(fileList []FileItem) {
	fmt.Println("\n============== 这是文件列表~ ==============")
	for i, f := range fileList {
		if f.FileType == "folder" {
			fmt.Printf("[%d] %s\n", i+1, f.FileName)
		} else if size := strings.TrimSpace(f.SizeStr); size != "" {
			fmt.Printf("[%d] %s (%s)\n", i+1, f.FileName, size)
		} else {
			fmt.Printf("[%d] %s\n", i+1, f.FileName)
		}
	}
	fmt.Println("==========================================")
}

// printPanCmdHelp 列出网盘链接模式的命令表（链接解析成功后展示一次）
func printPanCmdHelp() {
	fmt.Println("--------------- 命令说明 ---------------")
	fmt.Println("  ls              重新列出当前目录")
	fmt.Println("  cd <序号>        进入文件夹（如 cd 1）")
	fmt.Println("  cd .. / u       返回上一级")
	fmt.Println("  download 0      下载当前目录全部文件")
	fmt.Println("  download 1,3,5  多选下载")
	fmt.Println("  q / exit        换个链接")
	fmt.Println("----------------------------------------")
}

// print123AuthHint 123盘强制登录时的凭据配置指引
func print123AuthHint() {
	fmt.Printf("%s[!] 123云盘现在强制要求登录态才能解析分享，需要给 NFD 配置 123 盘账号喵%s\n", ColorRed, ColorReset)
	fmt.Println("    方式一：下面输入 y 当场配置（本喵自动写入 yml 并硬刷新解析服务）")
	fmt.Println("    方式二：手动打开 netdisk-fast-download/resources/app-dev.yml 拉到最底部，")
	fmt.Println("            在 auths: → ye: 下填 username（账号）和 password（密码），重启工具生效")
	fmt.Println("    注意：账号密码只保存在本机这个 yml 文件里喵")
}

// normalizeSelStr 归一化选择串：全角逗号转半角、去空格
func normalizeSelStr(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "，", ","), " ", ""))
}

// isNumericSel 是否纯数字序号串（0 或 1,3,5 这类）
func isNumericSel(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789,", c) {
			return false
		}
	}
	return true
}

// parseFileSelection 解析 "0" / "1,3,5" 这类选择输入，返回文件下标（0 起算）
// 输入 0 代表全选喵
func parseFileSelection(selStr string, count int) ([]int, error) {
	if selStr == "0" {
		indices := make([]int, 0, count)
		for i := 0; i < count; i++ {
			indices = append(indices, i)
		}
		return indices, nil
	}

	var indices []int
	seen := make(map[int]bool)
	for _, part := range strings.Split(selStr, ",") {
		num, err := strconv.Atoi(part)
		if err != nil || num < 1 || num > count {
			return nil, fmt.Errorf("序号 %s 无效喵！请输入 1-%d 之间的数字，或 0 全选", part, count)
		}
		if seen[num-1] {
			continue
		}
		seen[num-1] = true
		indices = append(indices, num-1)
	}
	if len(indices) == 0 {
		return nil, fmt.Errorf("没有选中任何文件喵")
	}
	return indices, nil
}

// 提取链接中的 pwd 参数
func urlParseHelper(rawurl string) (string, error) {
	idx := strings.Index(rawurl, "pwd=")
	if idx == -1 {
		return "", fmt.Errorf("no pwd")
	}
	pwd := rawurl[idx+4:]
	end := strings.Index(pwd, "&")
	if end != -1 {
		pwd = pwd[:end]
	}
	return pwd, nil
}
