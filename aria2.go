package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	rpcURL    = "http://127.0.0.1:6800/jsonrpc"
	rpcSecret = "mytoken"

	ColorRed   = "\033[31m"
	ColorGreen = "\033[32m"
	ColorCyan  = "\033[36m"
	ColorGray  = "\033[90m"
	ColorReset = "\033[0m"
)

type RpcRequest struct {
	JsonRpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      string        `json:"id"`
}

type RpcResponse struct {
	JsonRpc string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
	Error   interface{} `json:"error"`
	ID      string      `json:"id"`
}

func callAria2(method string, params []interface{}) (interface{}, error) {
	actualParams := append([]interface{}{"token:" + rpcSecret}, params...)
	reqBody := RpcRequest{JsonRpc: "2.0", Method: method, Params: actualParams, ID: "go-client"}
	jsonData, _ := json.Marshal(reqBody)

	resp, err := http.Post(rpcURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var rpcResp RpcResponse
	json.Unmarshal(body, &rpcResp)

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC 报错了喵~: %v", rpcResp.Error)
	}
	return rpcResp.Result, nil
}

func addDownloadTask(url string, dir string, outName string, referer string, cookie string) (string, error) {
	// 不同网盘的 CDN 对 Referer/Cookie 要求不一样，由调用方按网盘传
	headers := []string{
		"User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
	if referer != "" {
		headers = append(headers, "Referer: "+referer)
	}
	if cookie != "" {
		headers = append(headers, "Cookie: "+cookie)
	}
	options := map[string]interface{}{
		"dir":                       dir,
		"out":                       outName,
		"file-allocation":           "none",
		"split":                     "16",
		"max-connection-per-server": "16",
		"min-split-size":            "1M",
		"header":                    headers,
	}
	params := []interface{}{[]string{url}, options}
	result, err := callAria2("aria2.addUri", params)
	if err != nil {
		return "", err
	}
	return result.(string), nil
}

// ================== 新增：格式化工具 ==================

// 将字节数转换为人类可读的格式 (KB, MB, GB)
func formatBytes(bytesStr string) string {
	bytes, _ := strconv.ParseFloat(bytesStr, 64)
	if bytes == 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for bytes >= 1024 && i < len(units)-1 {
		bytes /= 1024
		i++
	}
	return fmt.Sprintf("%.2f %s", bytes, units[i])
}

// 生成可视化的进度条
func generateProgressBar(currentStr, totalStr string) string {
	current, _ := strconv.ParseFloat(currentStr, 64)
	total, _ := strconv.ParseFloat(totalStr, 64)

	if total == 0 {
		return "[等本喵看看大小喵...]"
	}

	percent := (current / total) * 100
	if percent > 100 {
		percent = 100
	}

	// 进度条总宽度
	width := 25
	completedWidth := int((percent / 100) * float64(width))

	// 拼接进度条，用绿色代表已完成，灰色代表未完成
	bar := strings.Repeat("█", completedWidth) + strings.Repeat("░", width-completedWidth)

	return fmt.Sprintf("%s[%s]%s %.1f%%", ColorGreen, bar, ColorReset, percent)
}

// ================== 监控进度更新 ==================

// watchStopSignal 监听 Ctrl+C，用来"只停止视监、不打断下载"
// 用完记得 signal.Stop(ch) 归还喵
func watchStopSignal() chan os.Signal {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	return sigCh
}

// shortFileName 去掉路径，只留文件名
func shortFileName(path string) string {
	if idx := strings.LastIndex(path, "/"); idx != -1 {
		return path[idx+1:]
	}
	if idx := strings.LastIndex(path, "\\"); idx != -1 {
		return path[idx+1:]
	}
	return path
}

// taskDisplayName 从任务状态里抠出文件名，抠不到就用 gid 兜底
func taskDisplayName(statusMap map[string]interface{}, gid string) string {
	if files, ok := statusMap["files"].([]interface{}); ok && len(files) > 0 {
		if f, ok := files[0].(map[string]interface{}); ok {
			if p, ok := f["path"].(string); ok && p != "" {
				return shortFileName(p)
			}
		}
	}
	return "任务 gid=" + gid
}

func checkStatus(gid string) {
	sigCh := watchStopSignal()
	defer signal.Stop(sigCh)

	fmt.Println("[+] 开始视监下载进度喵~（按 Ctrl+C 停止视监，任务会继续在后台跑哦）...")
	for {
		result, err := callAria2("aria2.tellStatus", []interface{}{gid})
		if err != nil {
			fmt.Printf("%s[error] 查询失败: %v%s\n", ColorRed, err, ColorReset)
			return
		}

		statusMap, _ := result.(map[string]interface{})
		status, _ := statusMap["status"].(string)

		if status == "error" {
			fmt.Printf("\n%s[error] 下载失败了！主人看看错误代码: %v%s\n", ColorRed, statusMap["errorCode"], ColorReset)
			break
		}
		if status == "complete" {
			fmt.Printf("\n%s[ok] 咔哒~！下载完成喵！%s\n", ColorGreen, ColorReset)
			break
		}

		totalLen, _ := statusMap["totalLength"].(string)
		completedLen, _ := statusMap["completedLength"].(string)
		speed, _ := statusMap["downloadSpeed"].(string)

		// 清理当前行，防止旧进度条残留
		fmt.Print("\r\033[K")
		fmt.Printf("%s %s | %s/s",
			generateProgressBar(completedLen, totalLen),
			formatBytes(completedLen),
			formatBytes(speed),
		)

		// 等待刷新的这一秒里顺便听听 Ctrl+C
		select {
		case <-sigCh:
			fmt.Printf("\n%s[ok] 好嘞，本喵先不看了，任务还在后台偷偷跑喵~%s\n", ColorGreen, ColorReset)
			return
		case <-time.After(1 * time.Second): // 单任务监控 1 秒刷新，看起来更流畅
		}
	}
	fmt.Println() // 结束后换个行
}

// checkStatusMulti 视监多个任务，每个任务一行原地刷新，
// 全部完结或主人按 Ctrl+C 后返回（任务继续在后台跑）
func checkStatusMulti(gids []string) {
	sigCh := watchStopSignal()
	defer signal.Stop(sigCh)

	fmt.Printf("[+] 开始视监 %d 个任务喵~（按 Ctrl+C 停止视监，任务会继续在后台跑哦）...\n", len(gids))

	finalLine := make(map[string]string) // 已完结任务的定格显示行
	frames := 0

	for {
		doneCount := 0
		lines := make([]string, 0, len(gids))

		for _, gid := range gids {
			if line, ok := finalLine[gid]; ok {
				lines = append(lines, line)
				doneCount++
				continue
			}

			result, err := callAria2("aria2.tellStatus", []interface{}{gid})
			if err != nil {
				finalLine[gid] = fmt.Sprintf("%s[失败] gid=%s 查询失败: %v%s", ColorRed, gid, err, ColorReset)
				lines = append(lines, finalLine[gid])
				doneCount++
				continue
			}

			statusMap, _ := result.(map[string]interface{})
			status, _ := statusMap["status"].(string)
			name := taskDisplayName(statusMap, gid)

			switch status {
			case "complete":
				finalLine[gid] = fmt.Sprintf("%s[完成] %s%s", ColorGreen, name, ColorReset)
				doneCount++
			case "error":
				finalLine[gid] = fmt.Sprintf("%s[失败] %s (错误代码: %v)%s", ColorRed, name, statusMap["errorCode"], ColorReset)
				doneCount++
			case "removed":
				finalLine[gid] = fmt.Sprintf("%s[移除] %s%s", ColorGray, name, ColorReset)
				doneCount++
			default:
				// 还在下载中，拼一条实时进度
				totalLen, _ := statusMap["totalLength"].(string)
				completedLen, _ := statusMap["completedLength"].(string)
				speed, _ := statusMap["downloadSpeed"].(string)
				lines = append(lines, fmt.Sprintf("%s %s | %s / %s | %s/s",
					name,
					generateProgressBar(completedLen, totalLen),
					formatBytes(completedLen),
					formatBytes(totalLen),
					formatBytes(speed),
				))
				continue
			}
			lines = append(lines, finalLine[gid])
		}

		// 光标上移覆盖上一帧重画，防止进度条刷屏
		if frames > 0 {
			fmt.Printf("\033[%dA", len(gids))
		}
		for _, line := range lines {
			fmt.Print("\r\033[K" + line + "\n")
		}
		frames++

		if doneCount == len(gids) {
			fmt.Printf("%s[ok] 这一拨任务都处理完啦喵~%s\n", ColorGreen, ColorReset)
			return
		}

		select {
		case <-sigCh:
			fmt.Printf("%s[ok] 好嘞，本喵先不看了，没完成的任务还在后台跑喵~%s\n", ColorGreen, ColorReset)
			return
		case <-time.After(1 * time.Second):
		}
	}
}

// ================== 全局任务大屏更新 ==================

func getActiveTasks() {
	result, err := callAria2("aria2.tellActive", []interface{}{})
	if err != nil {
		fmt.Printf("%s[error] 主人~查询任务失败喵: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	tasks := result.([]interface{})
	if len(tasks) == 0 {
		fmt.Println("[+] 主人~当前没有正在下载的任务喵")
		return
	}

	fmt.Printf("%s================ 当前正在下载喵 ================%s\n", ColorCyan, ColorReset)
	for _, t := range tasks {
		task := t.(map[string]interface{})
		totalLen := task["totalLength"].(string)
		completedLen := task["completedLength"].(string)
		speed := task["downloadSpeed"].(string)

		files := task["files"].([]interface{})
		fileName := "未知文件"
		if len(files) > 0 {
			file := files[0].(map[string]interface{})
			if path, ok := file["path"].(string); ok {
				fileName = path
			}
		}

		// 提取文件名（去掉前面的绝对路径）
		shortName := shortFileName(fileName)

		fmt.Printf("%s文件:%s %s\n", ColorGray, ColorReset, shortName)
		fmt.Printf("%s      %s | %s / %s | %s/s%s\n",
			ColorGray,
			generateProgressBar(completedLen, totalLen),
			formatBytes(completedLen),
			formatBytes(totalLen),
			formatBytes(speed),
			ColorReset,
		)
		fmt.Println("----------------------------------------------")
	}
}
