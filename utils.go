package main

import (
	"os/exec"
	"strings"
	"syscall"
)

// 隐藏控制台窗口 (仅 Windows)
func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow: true,
	}
}

// 通过端口号查找并杀死进程 (兼容 Windows)
func killProcessByPort(port string) {
	// 运行 netstat -ano 查找占用的 PID
	out, err := exec.Command("netstat", "-ano").Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		// 寻找处于 LISTENING 状态的对应端口
		if strings.Contains(line, ":"+port) && strings.Contains(line, "LISTENING") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				pid := fields[len(fields)-1]
				// 执行 taskkill 强杀
				killCmd := exec.Command("taskkill", "/F", "/PID", pid)
				killCmd.SysProcAttr = getSysProcAttr() // 隐藏 taskkill 弹出的黑框
				killCmd.Run()
			}
		}
	}
}

// 统一清理后台服务
func cleanupServices() {
	println("\n[+] 女仆正在打扫后台服务~...")
	killProcessByPort("6800") // 杀掉 Aria2
	killProcessByPort("6400") // 杀掉 NFD (Java)
	println("[ok] 后台服务已清理完毕~主人")
}
