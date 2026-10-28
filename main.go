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
		fmt.Println("q. 退出这里喵")
		fmt.Println("========================================")
		fmt.Print("请选择功能 (1/2/q): ")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(strings.ToLower(choice))

		if choice == "q" {
			fmt.Println("[+] 正在离开喵...")
			break
		}

		switch choice {
		case "1":
			runNfdMode(reader)
		case "2":
			runBaiduInteractive(reader)
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
	fmt.Println("========================================================\n")

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

	currentDir, _ := os.Getwd()
	downloadDir := filepath.Join(currentDir, "downloads")
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

// 蓝奏云解析
func runNfdMode(reader *bufio.Reader) {
	parser := &NfdParser{APIBase: "http://127.0.0.1:6400"}

	fmt.Print("\n主人~请输入蓝奏云等分享链接喵 (输入 q 返回): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if strings.ToLower(input) == "q" || input == "" {
		return
	}

	pwd := ""
	if u, err := urlParseHelper(input); err == nil {
		pwd = u
	}

	var fileList []FileItem
	var err error

	for {
		fmt.Println("[+] 主人~正在获取文件列表喵...")
		fileList, err = parser.GetFileList(input, pwd)
		if err != nil {
			if strings.Contains(err.Error(), "密码") {
				fmt.Printf("%s[error] %s%s\n", ColorRed, err.Error(), ColorReset)
				fmt.Print("[+] 主人~请输入提取码: ")
				pwdInput, _ := reader.ReadString('\n')
				pwd = strings.TrimSpace(pwdInput)
				if pwd == "q" {
					return
				}
				continue
			}
			fmt.Printf("%s[error] %s%s\n", ColorRed, err.Error(), ColorReset)
			return
		}
		break
	}

	if len(fileList) == 0 {
		return
	}

	fmt.Println("\n============== 这是文件列表~ ==============")
	for i, f := range fileList {
		fmt.Printf("[%d] %s (%s)\n", i+1, f.FileName, f.SizeStr)
	}
	fmt.Println("==========================================")

	var selectedFile FileItem
	for {
		fmt.Printf("请选择序号喵 (1-%d): ", len(fileList))
		inputStr, _ := reader.ReadString('\n')
		num, err := strconv.Atoi(strings.TrimSpace(inputStr))
		if err != nil || num < 1 || num > len(fileList) {
			continue
		}
		selectedFile = fileList[num-1]
		break
	}

	fmt.Printf("[+] 正在获取直链喏...\n")
	directLink, err := parser.GetDirectLink(selectedFile.FileID)
	if err != nil {
		fmt.Printf("%s[error] %v%s\n", ColorRed, err, ColorReset)
		return
	}

	fmt.Println("[+] 主人~正在推送到 aria2 喵...")
	currentDir, _ := os.Getwd()
	dir := filepath.Join(currentDir, "downloads")
	gid, err := addDownloadTask(directLink, dir, selectedFile.FileName)
	if err != nil {
		fmt.Printf("%s[error] %v%s\n", ColorRed, err, ColorReset)
		return
	}
	fmt.Printf("%s[ok] GID: %s%s\n", ColorGreen, gid, ColorReset)
	checkStatus(gid)
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
