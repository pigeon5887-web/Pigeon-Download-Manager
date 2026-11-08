package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const configFileName = "pigeon_config.json"

// AppConfig 本地配置
type AppConfig struct {
	DownloadDir string `json:"download_dir"`
}

// appConfig 全局配置（main 里 loadConfig 初始化）
var appConfig AppConfig

// configPath 配置文件路径：优先程序目录，没权限写就退回用户配置目录
// （应对软件被丢进 C:\Windows 这类写不进去的位置喵）
func configPath() string {
	if _, err := os.Stat(configFileName); err == nil {
		return configFileName
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "PigeonDownload", "config.json")
	}
	return configFileName
}

// loadConfig 启动时加载配置，没有就给默认值
func loadConfig() {
	if data, err := os.ReadFile(configPath()); err == nil {
		_ = json.Unmarshal(data, &appConfig)
	}

	if strings.TrimSpace(appConfig.DownloadDir) == "" {
		currentDir, _ := os.Getwd()
		appConfig.DownloadDir = filepath.Join(currentDir, "downloads")
	}

	if !ensureDirWritable(appConfig.DownloadDir) {
		fmt.Printf("%s[!] 注意喵：当前下载路径写不进去（软件可能被放在了受保护的位置）%s\n", ColorRed, ColorReset)
		fmt.Printf("%s    可以在主菜单选 3 重新设置下载路径哦~%s\n", ColorRed, ColorReset)
	}
}

// saveConfig 保存配置
func saveConfig() error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(appConfig, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ensureDirWritable 确保目录存在且真的能写
// （目录建好了不代表能写文件，C:\Windows 这种地方 mkdir 会骗人的喵）
func ensureDirWritable(dir string) bool {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false
	}
	probe := filepath.Join(dir, ".pigeon_write_test")
	if err := os.WriteFile(probe, []byte("ok"), 0644); err != nil {
		return false
	}
	_ = os.Remove(probe)
	return true
}

// runDownloadPathMenu 主菜单 3：查看/设置下载路径
func runDownloadPathMenu(reader *bufio.Reader) {
	fmt.Printf("\n当前下载路径喵: %s\n", appConfig.DownloadDir)
	fmt.Print("输入新的下载路径 (直接回车保持不变喵): ")
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println()
		return
	}

	// 去掉首尾空白和资源管理器复制路径时带上的引号
	input = strings.TrimSpace(strings.Trim(strings.TrimSpace(input), "\"'"))
	if input == "" {
		fmt.Println("[ok] 路径保持不变喵")
		return
	}

	// 相对路径转绝对路径
	if abs, absErr := filepath.Abs(input); absErr == nil {
		input = abs
	}

	if !ensureDirWritable(input) {
		fmt.Printf("%s[error] 这个路径本喵创建/写入失败了，可能是没权限喵，换一个吧~%s\n", ColorRed, ColorReset)
		return
	}

	appConfig.DownloadDir = input
	if err := saveConfig(); err != nil {
		fmt.Printf("%s[error] 配置文件保存失败喵: %v（本次会话内依然生效）%s\n", ColorRed, err, ColorReset)
	} else {
		fmt.Printf("%s[ok] 下载路径已经记在小本本上啦: %s%s\n", ColorGreen, appConfig.DownloadDir, ColorReset)
	}
}

// ================= NFD 解析服务的 123 盘账号配置 =================
// NFD 启动时把 app-dev.yml 读一次进内存，之后不再看磁盘上的文件，
// 所以改完配置必须"硬刷新"（杀进程重新拉起）才生效。这里把整套动作自动化。

const nfdYmlRelPath = "netdisk-fast-download/resources/app-dev.yml"

func readNfd123Auth() (string, string, bool) {
	return readNfd123AuthFrom(nfdYmlRelPath)
}

// readNfd123AuthFrom 读取 yml 里 auths.ye 的账号密码
// 只认 auths: 段之后的字段，不会误伤前面 dataSource 的数据库账密
func readNfd123AuthFrom(path string) (string, string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", false
	}
	content := string(data)
	idx := strings.Index(content, "auths:")
	if idx < 0 {
		return "", "", true
	}
	tail := content[idx:]
	// 注意用 [ \t] 而不是 \s：\s 会跨行，空 username 会错抓到下一行的 password
	userRe := regexp.MustCompile(`(?m)^[ \t]*username:[ \t]*(\S.*)$`)
	passRe := regexp.MustCompile(`(?m)^[ \t]*password:[ \t]*(\S.*)$`)
	user, pass := "", ""
	if m := userRe.FindStringSubmatch(tail); m != nil {
		user = yamlUnquote(m[1])
	}
	if m := passRe.FindStringSubmatch(tail); m != nil {
		pass = yamlUnquote(m[1])
	}
	return user, pass, true
}

// yamlUnquote 解 YAML 单引号字符串：去掉外层引号并把 '' 还原成 '
func yamlUnquote(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'") {
		return strings.ReplaceAll(s[1:len(s)-1], "''", "'")
	}
	return strings.Trim(s, "\"'")
}

func writeNfd123Auth(user, pass string) error {
	return writeNfd123AuthTo(nfdYmlRelPath, user, pass)
}

// writeNfd123AuthTo 把账号密码写进 auths.ye 段（先备份 .bak，值加单引号防 YAML 特殊字符）
func writeNfd123AuthTo(path, user, pass string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)
	idx := strings.Index(content, "auths:")
	if idx < 0 {
		return fmt.Errorf("yml 里找不到 auths: 段")
	}
	head, tail := content[:idx], content[idx:]

	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	if tail, err = yamlSetFirst(tail, "username", quote(user)); err != nil {
		return err
	}
	if tail, err = yamlSetFirst(tail, "password", quote(pass)); err != nil {
		return err
	}

	_ = os.WriteFile(path+".bak", data, 0644) // 改前留个后路
	return os.WriteFile(path, []byte(head+tail), 0644)
}

// yamlSetFirst 替换文本中第一个 `字段: ...` 行，保留原缩进
func yamlSetFirst(text, field, value string) (string, error) {
	re := regexp.MustCompile(`(?m)^([ \t]*)` + field + `:.*$`)
	loc := re.FindStringSubmatchIndex(text)
	if loc == nil {
		return "", fmt.Errorf("yml 里找不到 %s 字段", field)
	}
	indent := text[loc[2]:loc[3]]
	return text[:loc[0]] + indent + field + ": " + value + text[loc[1]:], nil
}

// restartNfdService 硬刷新解析服务：杀掉 6400 上的进程 → 重新拉起 → 重读配置
func restartNfdService() bool {
	fmt.Println("[+] 正在硬刷新解析服务喵（杀旧进程 → 重新拉起）...")
	killProcessByPort("6400")
	time.Sleep(1 * time.Second) // 等端口彻底松开

	startNfdDaemon()
	for i := 0; i < 8; i++ { // Java 启动要几秒，多等一会儿
		time.Sleep(1 * time.Second)
		conn, err := net.Dial("tcp", "127.0.0.1:6400")
		if err == nil {
			conn.Close()
			return true
		}
	}
	return false
}

// offerNfdAuthSetup 引导当场配置 123 盘账号并硬刷新服务
// 返回 true = 配置完成且服务已重启，可以重试解析
func offerNfdAuthSetup(reader *bufio.Reader) bool {
	user, _, ymlOK := readNfd123Auth()
	if !ymlOK {
		fmt.Printf("%s[!] 找不到 %s，自动配置失败喵，请手动检查文件位置%s\n", ColorRed, nfdYmlRelPath, ColorReset)
		return false
	}
	if user != "" {
		fmt.Printf("[i] 已经配置过账号：%s（密码本喵就不念出来了）\n", user)
	}

	fmt.Print("[?] 要现在当场配置吗？(y=现在配置 / 其他=跳过): ")
	choice, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println()
		return false
	}
	if strings.TrimSpace(strings.ToLower(choice)) != "y" {
		return false
	}

	fmt.Print("账号: ")
	user, _ = reader.ReadString('\n')
	user = strings.TrimSpace(user)
	if user == "" {
		fmt.Printf("%s[error] 账号不能为空喵%s\n", ColorRed, ColorReset)
		return false
	}
	fmt.Print("密码: ")
	pass, _ := reader.ReadString('\n')
	pass = strings.TrimSpace(pass)
	if pass == "" {
		fmt.Printf("%s[error] 密码不能为空喵%s\n", ColorRed, ColorReset)
		return false
	}

	fmt.Printf("%s[i] 账号密码会明文写进本机 yml（写之前先备份 .bak）%s\n", ColorGray, ColorReset)
	if err := writeNfd123Auth(user, pass); err != nil {
		fmt.Printf("%s[error] 写入配置失败喵: %v%s\n", ColorRed, err, ColorReset)
		return false
	}
	fmt.Printf("%s[ok] 配置已写入（原文件备份为 app-dev.yml.bak）%s\n", ColorGreen, ColorReset)

	if !restartNfdService() {
		fmt.Printf("%s[error] 解析服务重启失败喵，请退出工具重新打开%s\n", ColorRed, ColorReset)
		return false
	}
	fmt.Printf("%s[ok] 解析服务带着新配置复活了喵！%s\n", ColorGreen, ColorReset)
	return true
}
