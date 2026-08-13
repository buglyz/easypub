// Package batch 提供目录批量转换能力，面向服务器/脚本场景：
//
//	将某个目录下的全部 TXT 逐文件转换为 EPUB，单个文件失败不中断，
//	全部结束后返回逐文件结果汇总，便于 cron / 脚本集成。
package batch

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/easypub/go-easypub/internal/converter"
)

// Options 描述一次批量转换。
type Options struct {
	// InputDir 扫描的 TXT 目录。
	InputDir string
	// OutputDir 输出目录；为空时使用 InputDir（与单文件模式"输出到输入同目录"一致）。
	OutputDir string
	// Title/Author 应用到所有文件；为空时逐文件自动推断。
	Title  string
	Author string
	// ConfigPath / EReadersPath / FontIndex 透传给单文件转换。
	ConfigPath   string
	EReadersPath string
	FontIndex    int
	EnableMobi   bool
	// Quiet 抑制 kindlegen 输出。
	Quiet bool
	// SkipExisting 已存在同名 EPUB 时跳过该文件，便于重复执行（幂等）。
	SkipExisting bool
	// Exts 参与转换的扩展名；为空时使用默认 [.txt .utf8 .gbk]。
	Exts []string
}

// Result 记录单个文件的转换结果。
type Result struct {
	Input    string
	Output   string
	Encoding string
	Chapters int
	Err      error
}

// defaultExts 默认参与转换的扩展名。
var defaultExts = []string{".txt", ".utf8", ".gbk"}

// Run 转换 InputDir 内所有匹配扩展名的文件（按文件名排序，顺序执行）。
// 返回逐文件结果；至少扫描到一个文件时返回 nil 错误（单文件错误在 Result.Err 中），
// 目录不存在或无匹配文件时返回错误。
func Run(opt Options) ([]Result, error) {
	if opt.InputDir == "" {
		return nil, fmt.Errorf("输入目录为空")
	}
	entries, err := os.ReadDir(opt.InputDir)
	if err != nil {
		return nil, fmt.Errorf("读取输入目录失败: %w", err)
	}

	exts := opt.Exts
	if len(exts) == 0 {
		exts = defaultExts
	}
	lower := make(map[string]bool, len(exts))
	for _, e := range exts {
		e = strings.ToLower(e)
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		lower[e] = true
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if lower[strings.ToLower(filepath.Ext(e.Name()))] {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("目录 %s 中没有匹配的 TXT 文件(支持 %s)", opt.InputDir, strings.Join(exts, "/"))
	}
	sort.Strings(files)

	outDir := opt.OutputDir
	if outDir == "" {
		outDir = opt.InputDir
	}

	results := make([]Result, 0, len(files))
	for _, name := range files {
		inPath := filepath.Join(opt.InputDir, name)
		stem := strings.TrimSuffix(name, filepath.Ext(name))
		outPath := filepath.Join(outDir, stem+".epub")

		res := Result{Input: inPath, Output: outPath}
		if opt.SkipExisting {
			if _, err := os.Stat(outPath); err == nil {
				res.Err = ErrSkipped
				results = append(results, res)
				continue
			}
		}

		cr, cerr := converter.Convert(converter.Options{
			Input:        inPath,
			Output:       outPath,
			Title:        opt.Title,
			Author:       opt.Author,
			ConfigPath:   opt.ConfigPath,
			EReadersPath: opt.EReadersPath,
			FontIndex:    opt.FontIndex,
			EnableMobi:   opt.EnableMobi,
			Quiet:        opt.Quiet,
		})
		if cerr != nil {
			res.Err = cerr
		} else {
			res.Encoding = cr.Encoding
			res.Chapters = cr.ChapterCount
		}
		results = append(results, res)
	}
	return results, nil
}

// ErrSkipped 标记因目标已存在而跳过的文件。
var ErrSkipped = fmt.Errorf("目标 EPUB 已存在,已跳过(--skip-existing)")
