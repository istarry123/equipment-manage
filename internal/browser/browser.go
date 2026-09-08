// Package browser 负责启动后自动打开默认/指定浏览器（Win7 优先 Chrome）。
package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Open 打开 url：
//  1. 探测常见 Chrome 安装路径并优先使用（Win7 上默认浏览器可能是 IE，必须规避）；
//  2. 未找到则调用系统默认浏览器（rundll32 url.dll,FileProtocolHandler）。
func Open(url string) error {
	if runtime.GOOS != "windows" {
		return openWith("", url)
	}
	for _, p := range chromeCandidates() {
		if _, err := os.Stat(p); err == nil {
			return openWith(p, url)
		}
	}
	return openDefault(url)
}

func chromeCandidates() []string {
	var out []string
	roots := []string{
		os.Getenv("ProgramFiles(x86)"),
		os.Getenv("ProgramFiles"),
		os.Getenv("LocalAppData"),
	}
	for _, root := range roots {
		if root == "" {
			continue
		}
		out = append(out, filepath.Join(root, "Google", "Chrome", "Application", "chrome.exe"))
	}
	return out
}

func openWith(exe, url string) error {
	cmd := exec.Command(exe, url)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // 防僵尸
	return nil
}

func openDefault(url string) error {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("自动打开浏览器失败，请手动访问 %s: %w", url, err)
	}
	go cmd.Wait()
	return nil
}
