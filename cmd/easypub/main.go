// Package main 是 EasyPub 命令行入口。
//
// 用法:
//
//	easypub -i book.txt -o output.epub
//	easypub -i book.txt -o output.epub --title "书名" --author "作者"
//	easypub -i book.txt -o output.epub --config config.xml
//	easypub -i book.txt -o output.epub --mobi
//	easypub serve -addr :8080
//	easypub health http://127.0.0.1:8080/healthz
//	easypub -version
//
// 不带参数时打印帮助。
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/easypub/go-easypub/internal/batch"
	"github.com/easypub/go-easypub/internal/converter"
	"github.com/easypub/go-easypub/internal/webui"
)

// version 通过 -ldflags "-X main.version=..." 注入，默认 dev。
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "serve":
			if err := serveCmd(os.Args[2:]); err != nil {
				log.Printf("easypub serve: %v", err)
				os.Exit(2)
			}
			return
		case "health":
			// 容器内探活：distroless 无 curl/wget，用二进制自身发起请求。
			if err := healthCmd(os.Args[2:]); err != nil {
				os.Exit(1)
			}
			return
		}
	}
	if err := run(); err != nil {
		log.Fatalf("easypub: %v", err)
	}
}

// healthCmd 向指定 URL 发起 GET，2xx 视为健康，否则退出码 1。
// 专供 Docker HEALTHCHECK / k8s liveness 使用。
func healthCmd(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("缺少 URL 参数")
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(args[0])
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("状态码 %d", resp.StatusCode)
	}
	return nil
}

// serveCmd 启动 WebUI。
func serveCmd(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	addr := fs.String("addr", "127.0.0.1:8080", "监听地址(默认仅本机; 局域网请显式 0.0.0.0:8080,无鉴权)")
	workdir := fs.String("dir", "", "工作目录(用于查找 config.xml/ereaders.xml,默认当前目录)")
	token := fs.String("token", os.Getenv("EASYPUB_TOKEN"), "访问令牌(留空不启用鉴权; 也支持环境变量 EASYPUB_TOKEN)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "easypub serve - 启动 WebUI")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "用法: easypub serve [-addr 127.0.0.1:8080] [-dir workdir]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		// -h 时 flag 返回 ErrHelp，统一退出码 2。
		return err
	}
	dir := *workdir
	if dir == "" {
		if cwd, err := os.Getwd(); err == nil {
			dir = cwd
		}
	}
	srv, err := webui.New(webui.Config{Addr: *addr, WorkDir: dir, Version: version, Token: *token})
	if err != nil {
		return err
	}
	return srv.Run()
}

func run() error {
	var (
		input       = flag.String("i", "", "输入 TXT 文件或目录(目录时批量转换其中 *.txt/*.utf8/*.gbk,必填)")
		output      = flag.String("o", "", "输出路径/目录(单文件默认与输入同名 .epub; 目录输入时表示输出目录,默认=输入目录)")
		title       = flag.String("title", "", "书名(默认从 TXT 首个非空行或文件名推断; 批量时应用到所有文件)")
		author      = flag.String("author", "", "作者")
		configP     = flag.String("config", "", "config.xml 路径(可选,默认使用内建默认配置)")
		ereaders    = flag.String("ereaders", "", "ereaders.xml 路径(默认随 config 目录)")
		fontIdx     = flag.Int("font", -1, "ereaders.xml 中阅读器字体方案索引(从 0 开始,-1=用第一个)")
		enableMobi  = flag.Bool("mobi", false, "同时输出 MOBI(调用本机 kindlegen)")
		skipExist   = flag.Bool("skip-existing", false, "批量模式:目标 EPUB 已存在时跳过该文件(幂等)")
		quiet       = flag.Bool("quiet", false, "静默模式:不打印进度")
		showVersion = flag.Bool("version", false, "打印版本并退出")
	)
	flag.Usage = printUsage
	flag.Parse()

	if *showVersion {
		fmt.Printf("easypub %s\n", version)
		return nil
	}
	if *input == "" {
		printUsage()
		return fmt.Errorf("缺少必填参数 -i")
	}
	info, err := os.Stat(*input)
	if err != nil {
		return fmt.Errorf("输入不存在: %w", err)
	}
	if info.IsDir() {
		return runBatch(batchOptions{
			Input: *input, Output: *output, Title: *title, Author: *author,
			ConfigPath: *configP, EReadersPath: *ereaders, FontIndex: *fontIdx,
			EnableMobi: *enableMobi, SkipExisting: *skipExist, Quiet: *quiet,
		})
	}

	res, err := converter.Convert(converter.Options{
		Input:        *input,
		Output:       *output,
		Title:        *title,
		Author:       *author,
		ConfigPath:   *configP,
		EReadersPath: *ereaders,
		FontIndex:    *fontIdx,
		EnableMobi:   *enableMobi,
		Quiet:        *quiet,
	})
	if err != nil {
		return err
	}
	if !*quiet {
		log.Printf("编码: %s | 章节: %d | 已生成: %s", res.Encoding, res.ChapterCount, res.EpubPath)
		if res.MobiPath != "" {
			log.Printf("已生成 MOBI: %s", res.MobiPath)
		}
	}
	return nil
}

// batchOptions 是 runBatch 的参数聚合。
type batchOptions struct {
	Input, Output   string
	Title, Author   string
	ConfigPath      string
	EReadersPath    string
	FontIndex       int
	EnableMobi      bool
	SkipExisting    bool
	Quiet           bool
}

// runBatch 将目录内全部 TXT 转换为 EPUB。
func runBatch(o batchOptions) error {
	results, err := batch.Run(batch.Options{
		InputDir:     o.Input,
		OutputDir:    o.Output,
		Title:        o.Title,
		Author:       o.Author,
		ConfigPath:   o.ConfigPath,
		EReadersPath: o.EReadersPath,
		FontIndex:    o.FontIndex,
		EnableMobi:   o.EnableMobi,
		Quiet:        o.Quiet,
		SkipExisting: o.SkipExisting,
	})
	if err != nil {
		return err
	}
	ok, failed, skipped := 0, 0, 0
	for _, r := range results {
		switch {
		case r.Err == nil:
			ok++
			if !o.Quiet {
				log.Printf("OK   %s -> %s (编码 %s, %d 章)", filepath.Base(r.Input), r.Output, r.Encoding, r.Chapters)
			}
		case errors.Is(r.Err, batch.ErrSkipped):
			skipped++
			if !o.Quiet {
				log.Printf("SKIP %s (%v)", filepath.Base(r.Input), r.Err)
			}
		default:
			failed++
			if !o.Quiet {
				log.Printf("FAIL %s: %v", filepath.Base(r.Input), r.Err)
			}
		}
	}
	log.Printf("批量完成: 成功 %d / 失败 %d / 跳过 %d / 共 %d", ok, failed, skipped, len(results))
	if failed > 0 {
		return fmt.Errorf("%d 个文件转换失败", failed)
	}
	return nil
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "EasyPub - TXT 转 EPUB 命令行工具 (Go 重写版)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "用法:")
	fmt.Fprintln(os.Stderr, "  easypub -i <input.txt> [options]       转换单个 TXT→EPUB")
	fmt.Fprintln(os.Stderr, "  easypub -i <dir> [options]            批量转换目录内 *.txt/*.utf8/*.gbk")
	fmt.Fprintln(os.Stderr, "  easypub serve [-addr 127.0.0.1:8080] [-dir d] [-token T] 启动 WebUI")
	fmt.Fprintln(os.Stderr, "  easypub health <url>                  容器探活（GET url，2xx 健康）")
	fmt.Fprintln(os.Stderr, "  easypub -version                      打印版本")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "转换选项:")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "示例:")
	fmt.Fprintln(os.Stderr, "  easypub -i book.txt")
	fmt.Fprintln(os.Stderr, "  easypub -i book.txt -o out.epub --title \"书名\" --author \"作者\"")
	fmt.Fprintln(os.Stderr, "  easypub -i /data/books -o /data/epub --skip-existing")
	fmt.Fprintln(os.Stderr, "  easypub -i book.txt --config config.xml --mobi")
	fmt.Fprintln(os.Stderr, "  easypub serve -addr 0.0.0.0:8080 -dir /data -token mysecret")
}
