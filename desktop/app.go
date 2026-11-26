package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/easypub/go-easypub/internal/epub"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 承载桌面端所有 Wails 绑定方法与会话状态。
type App struct {
	ctx     context.Context
	version string
	workDir string

	mu          sync.RWMutex
	currentPath string
	currentEnc  string
	currentText string
	lines       []string
}

// NewApp 创建桌面应用实例，自动定位配置基准目录。
func NewApp(ver string) *App {
	dir := resolveWorkDir()
	return &App{
		version: ver,
		workDir: dir,
	}
}

func resolveWorkDir() string {
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		if fileExists(filepath.Join(exeDir, "config.xml")) || fileExists(filepath.Join(exeDir, "ereaders.xml")) {
			return exeDir
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		// 开发模式在 desktop/ 下运行时兼容向上查找 configs/。
		if fileExists(filepath.Join(cwd, "config.xml")) {
			return cwd
		}
		if fileExists(filepath.Join(cwd, "..", "configs", "config.xml")) {
			return filepath.Clean(filepath.Join(cwd, "..", "configs"))
		}
		return cwd
	}
	return "."
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// 若配置中记录了置顶选项，启动时自动应用。
	cfg := a.LoadDesktopConfig()
	if cfg.Advanced.AlwaysOnTop {
		wailsruntime.WindowSetAlwaysOnTop(ctx, true)
	}
}

func (a *App) shutdown(ctx context.Context) {
	// 预留优雅退出钩子。
}

// GetVersion 返回桌面端版本号。
func (a *App) GetVersion() string {
	return a.version
}

// SetAlwaysOnTop 切换窗口总在最前。
func (a *App) SetAlwaysOnTop(on bool) {
	if a.ctx != nil {
		wailsruntime.WindowSetAlwaysOnTop(a.ctx, on)
	}
}

// PickTxtFile 弹出系统文件选择器选择 TXT 文件。
func (a *App) PickTxtFile() (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择要转换的 TXT 文件",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "文本文件 (*.txt;*.utf8;*.gbk)", Pattern: "*.txt;*.utf8;*.gbk;*.utf-8"},
			{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
		},
	})
}

// PickCoverImage 弹出系统文件选择器选择封面图片，返回路径及缩略 DataURL。
func (a *App) PickCoverImage() (*CoverInfo, error) {
	p, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择封面图片",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "图片文件 (*.jpg;*.jpeg;*.png;*.webp;*.gif)", Pattern: "*.jpg;*.jpeg;*.png;*.webp;*.gif"},
		},
	})
	if err != nil || p == "" {
		return nil, err
	}
	return a.LoadCoverPreview(p)
}

// LoadCoverPreview 读取封面图片并编码为 DataURL 供前端即时预览。
func (a *App) LoadCoverPreview(path string) (*CoverInfo, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取封面图片失败: %w", err)
	}
	if len(data) > 20<<20 {
		return nil, fmt.Errorf("封面图片超过 20 MB 限制")
	}
	_, mime, ok := epub.DetectImageType(data)
	if !ok {
		return nil, fmt.Errorf("不支持的图片格式（仅支持 JPG/PNG/GIF/WebP）")
	}
	return &CoverInfo{
		Path:    path,
		Name:    filepath.Base(path),
		DataURL: fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data)),
	}, nil
}

// PickDirectory 弹出系统文件夹选择器。
func (a *App) PickDirectory(title string) (string, error) {
	if title == "" {
		title = "选择文件夹"
	}
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: title,
	})
}

// PickConfigFile 弹出选择器加载外部 config.xml。
func (a *App) PickConfigFile() (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择 EasyPub config.xml",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "XML 配置文件 (*.xml)", Pattern: "*.xml"},
		},
	})
}

// PickKindleGenExe 弹出选择器定位 kindlegen 可执行文件。
func (a *App) PickKindleGenExe() (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择 kindlegen 可执行文件",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "可执行文件 (*.exe)", Pattern: "*.exe;*"},
		},
	})
}

// OpenInExplorer 在系统文件管理器中打开目标文件或目录。
func (a *App) OpenInExplorer(target string) error {
	if target == "" {
		return fmt.Errorf("路径为空")
	}
	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("路径不存在: %w", err)
	}
	switch runtime.GOOS {
	case "windows":
		if !info.IsDir() {
			return exec.Command("explorer.exe", "/select,"+filepath.Clean(target)).Start()
		}
		return exec.Command("explorer.exe", filepath.Clean(target)).Start()
	case "darwin":
		if !info.IsDir() {
			return exec.Command("open", "-R", target).Start()
		}
		return exec.Command("open", target).Start()
	default:
		dir := target
		if !info.IsDir() {
			dir = filepath.Dir(target)
		}
		return exec.Command("xdg-open", dir).Start()
	}
}

// IsDirectory 判断给定路径是否为目录（供文件拖拽时自动切换单本/批量模式）。
func (a *App) IsDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
