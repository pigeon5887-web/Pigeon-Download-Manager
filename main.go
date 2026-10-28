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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

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
		fmt.Printf("%s[error] 启动 NFD 服务失败: %v%s\n", ColorRed, err, ColorReset)
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
	startAria2Daemon()
	startNfdDaemon()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cleanupServices()
		os.Exit(0)
	}()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("主人~欢迎喏！请选择你要的网盘，开始今天的工作！")
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
			cleanupServices()
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

func getBaiduCookieWithChromedp() (string, error) {
	fmt.Println("[+] 正在启动本地浏览器提取曲奇喵~...")
	currentDir, _ := os.Getwd()
	userDataDir := filepath.Join(currentDir, "user-data-baidu-chromedp")

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
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
	fmt.Println("[!] 主人，如果弹出的浏览器未登录，请扫码登录百度网盘喵~！")
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
			return fmt.Errorf("等待登录超时喵...")
		}),
	)

	if err != nil {
		return "", fmt.Errorf("获取曲奇失败喵: %v", err)
	}

	time.Sleep(2 * time.Second)
	fmt.Println(ColorGreen + "[ok] 成功获取到曲奇~耶嘿！" + ColorReset)
	return cookieStr, nil
}

func checkAndLoginBaidu(reader *bufio.Reader) bool {
	cookieFile := "baidu_cookie.txt"

	if fileData, err := os.ReadFile(cookieFile); err == nil && len(fileData) > 20 {
		cookieStr := strings.TrimSpace(string(fileData))
		fmt.Println("[+] 发现本地放着的曲奇~正在进行偷吃喵...")
		loginCmd := exec.Command("./BaiduPCS-Go.exe", "login", "-cookies="+cookieStr)
		loginCmd.Stdout = os.Stdout
		loginCmd.Stderr = os.Stderr
		loginCmd.Run()

		verifyCmd := exec.Command("./BaiduPCS-Go.exe", "who")
		verifyOut, _ := verifyCmd.CombinedOutput()
		if !strings.Contains(string(verifyOut), "uid: 0") {
			fmt.Println(ColorGreen + "[ok] 已通过本地的曲奇自动通过~" + ColorReset)
			return true
		}
		fmt.Println(ColorRed + "[!] 本地曲奇吃完了，需要重新获取喵~" + ColorReset)
	}

	fmt.Println("[+] 正在检查百度网盘登录状态喵...")
	cmd := exec.Command("./BaiduPCS-Go.exe", "who")
	output, _ := cmd.CombinedOutput()
	outputStr := string(output)

	if strings.Contains(outputStr, "uid: 0") || strings.Contains(outputStr, "用户名: ,") {
		fmt.Println(ColorRed + "[!] 主人，未登录或登录已过期喵" + ColorReset)

		cookieStr, err := getBaiduCookieWithChromedp()
		if err == nil {
			_ = os.WriteFile(cookieFile, []byte(cookieStr), 0644)
			fmt.Println(ColorGreen + "[ok] 曲奇已放到到 baidu_cookie.txt " + ColorReset)

			fmt.Println("[+] 正在将曲奇放到BaiduPCS-Go中喵...")
			loginCmd := exec.Command("./BaiduPCS-Go.exe", "login", "-cookies="+cookieStr)
			loginCmd.Stdout = os.Stdout
			loginCmd.Stderr = os.Stderr
			loginCmd.Run()
		} else {
			fmt.Println(ColorRed + "[!] 自动获取失败喵: " + err.Error() + ColorReset)
			fmt.Print("[?] 请手动给本喵曲奇~ (直接回车跳过): ")
			inputCookie, _ := reader.ReadString('\n')
			inputCookie = strings.TrimSpace(inputCookie)

			if inputCookie == "" {
				fmt.Println(ColorRed + "[error] 主人取消了登录..." + ColorReset)
				return false
			}

			_ = os.WriteFile(cookieFile, []byte(inputCookie), 0644)
			loginCmd := exec.Command("./BaiduPCS-Go.exe", "login", "-cookies="+inputCookie)
			loginCmd.Stdout = os.Stdout
			loginCmd.Stderr = os.Stderr
			loginCmd.Run()
		}

		verifyCmd := exec.Command("./BaiduPCS-Go.exe", "who")
		verifyOut, _ := verifyCmd.CombinedOutput()
		if strings.Contains(string(verifyOut), "uid: 0") {
			fmt.Println(ColorRed + "\n[error] 登录失败或被中断喵" + ColorReset)
			return false
		}
		fmt.Println(ColorGreen + "\n[ok] 主人登录成功~！" + ColorReset)
		return true
	}

	fmt.Println(ColorGreen + "[ok] BaiduPCS-Go 已处于登录状态喵" + ColorReset)
	return true
}

func runBaiduInteractive(reader *bufio.Reader) {
	if !checkAndLoginBaidu(reader) {
		return
	}

	currentDir, _ := os.Getwd()
	downloadDir := filepath.Join(currentDir, "downloads")
	exec.Command("./BaiduPCS-Go.exe", "config", "set", "-savedir", downloadDir).Run()

	fmt.Println(ColorCyan + "\n[+] 认证完成主人！正在进入 BaiduPCS-Go 命令行中..." + ColorReset)
	fmt.Println("[+] 常用命令提示：")
	fmt.Println("    ls              查看当前目录文件")
	fmt.Println("    cd <目录名>     进入文件夹")
	fmt.Println("    download <文件名> 下载文件 (支持多线程)")
	fmt.Println("    exit            退出命令行")
	fmt.Println("========================================")

	cmd := exec.Command("./BaiduPCS-Go.exe")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	fmt.Println(ColorGreen + "\n[ok] 已离开百度网盘命令行喵" + ColorReset)
}

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
