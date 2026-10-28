package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
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

func addDownloadTask(url string, dir string, outName string) (string, error) {
	options := map[string]interface{}{
		"dir":             dir,
		"out":             outName,
		"file-allocation": "none",
		"header": []string{
			"User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Referer: https://www.lanzou.com/",
		},
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

func checkStatus(gid string) {
	fmt.Println("[+] 开始视监下载进度喵~（按 Ctrl+C 结束）...")
	for {
		result, err := callAria2("aria2.tellStatus", []interface{}{gid})
		if err != nil {
			fmt.Printf("%s[error] 查询失败: %v%s\n", ColorRed, err, ColorReset)
			return
		}

		statusMap := result.(map[string]interface{})
		status := statusMap["status"].(string)

		if status == "error" {
			fmt.Printf("\n%s[error] 下载失败了！主人看看错误代码: %v%s\n", ColorRed, statusMap["errorCode"], ColorReset)
			break
		}
		if status == "complete" {
			fmt.Printf("\n%s[ok] 咔哒~！下载完成喵！%s\n", ColorGreen, ColorReset)
			break
		}

		totalLen := statusMap["totalLength"].(string)
		completedLen := statusMap["completedLength"].(string)
		speed := statusMap["downloadSpeed"].(string)

		// 清理当前行，防止旧进度条残留
		fmt.Print("\r\033[K")
		fmt.Printf("%s %s | %s/s",
			generateProgressBar(completedLen, totalLen),
			formatBytes(completedLen),
			formatBytes(speed),
		)

		time.Sleep(1 * time.Second) // 单任务监控改成 1 秒刷新，看起来更流畅
	}
	fmt.Println() // 结束后换个行
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
		shortName := fileName
		if idx := strings.LastIndex(fileName, "/"); idx != -1 {
			shortName = fileName[idx+1:]
		}
		if idx := strings.LastIndex(shortName, "\\"); idx != -1 {
			shortName = shortName[idx+1:]
		}

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
