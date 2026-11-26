package main

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// minimalPNG 是一个合法的 1x1 PNG 图像字节序列，用于测试自定义封面写入。
var minimalPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
	0x42, 0x60, 0x82,
}

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	app := &App{ctx: context.Background(), version: "test", workDir: dir}

	cfg := app.LoadDesktopConfig()
	cfg.Split.Mode = "simple"
	cfg.Split.SimpleRegP1 = "[第]"
	cfg.Split.SimpleRegP2 = 1
	cfg.Style.LineHeight = 150
	cfg.Style.Indent = 2.0
	cfg.Advanced.AlwaysOnTop = true

	if err := app.SaveDesktopConfig(cfg); err != nil {
		t.Fatalf("SaveDesktopConfig: %v", err)
	}

	reloaded := app.LoadDesktopConfig()
	if reloaded.Split.Mode != "simple" || reloaded.Split.SimpleRegP2 != 1 {
		t.Errorf("Split 未正确往返: %+v", reloaded.Split)
	}
	if reloaded.Style.LineHeight != 150 || reloaded.Style.Indent != 2.0 {
		t.Errorf("Style 未正确往返: %+v", reloaded.Style)
	}
	if !reloaded.Advanced.AlwaysOnTop {
		t.Errorf("AlwaysOnTop 未保存")
	}
}

func TestChapterEditingAndCustomCoverConvert(t *testing.T) {
	dir := t.TempDir()
	app := &App{ctx: context.Background(), version: "test", workDir: dir}

	txtPath := filepath.Join(dir, "novel.txt")
	raw := strings.Join([]string{
		"书名：桌面测试书",
		"第1章 初遇",
		"　　第一段内容。",
		"第2章 误切的短章",
		"　　这行应该并入第一章。",
		"第3章 广告章",
		"　　这章会被彻底删除。",
		"第4章 终局",
		"　　最后一段。",
	}, "\n")
	if err := os.WriteFile(txtPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	coverPath := filepath.Join(dir, "cover.png")
	if err := os.WriteFile(coverPath, minimalPNG, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := app.LoadTxtFile(txtPath)
	if err != nil {
		t.Fatalf("LoadTxtFile: %v", err)
	}
	if loaded.InferredTit != "桌面测试书" {
		t.Errorf("推断书名=%q, 期望 桌面测试书", loaded.InferredTit)
	}

	cfg := app.LoadDesktopConfig()
	chs, err := app.DetectChapters(cfg.Split)
	if err != nil {
		t.Fatalf("DetectChapters: %v", err)
	}
	// 包含前置书名段 + 4 章 = 5 项
	if len(chs) != 5 {
		t.Fatalf("章节数=%d, 期望 5: %+v", len(chs), chs)
	}

	// 编辑操作：
	// - 重命名第1章 -> "第一章 改名后"
	// - 将第2章设为 excluded（合并入第1章）
	// - 将第3章设为 purged（彻底删除）
	chs[1].Title = "第一章 改名后"
	chs[2].State = "excluded"
	chs[3].State = "purged"

	preview, err := app.GetChapterPreview(chs[1], cfg.Split)
	if err != nil || len(preview.Paragraphs) == 0 {
		t.Fatalf("GetChapterPreview 失败: err=%v preview=%+v", err, preview)
	}

	outEpub := filepath.Join(dir, "out.epub")
	res, err := app.ConvertCurrent(ConvertRequest{
		Title:          loaded.InferredTit,
		Author:         "测试作者",
		OutputPath:     outEpub,
		CoverImagePath: coverPath,
		Split:          cfg.Split,
		Style:          cfg.Style,
		Advanced:       cfg.Advanced,
		Chapters:       chs,
	})
	if err != nil {
		t.Fatalf("ConvertCurrent: %v", err)
	}

	r, err := zip.OpenReader(res.EpubPath)
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer r.Close()

	entries := map[string]string{}
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		entries[f.Name] = string(b)
	}

	if _, ok := entries["OEBPS/cover.png"]; !ok {
		t.Errorf("EPUB 缺少自定义封面 OEBPS/cover.png")
	}
	if !strings.Contains(entries["OEBPS/cover.html"], `src="cover.png"`) {
		t.Errorf("cover.html 未引用 cover.png: %s", entries["OEBPS/cover.html"])
	}
	if !strings.Contains(entries["OEBPS/book-toc.html"], "第一章 改名后") {
		t.Errorf("重命名标题未写入目录: %s", entries["OEBPS/book-toc.html"])
	}
	if strings.Contains(entries["OEBPS/book-toc.html"], "误切的短章") {
		t.Errorf("excluded 章节不应出现在目录中")
	}
	if !strings.Contains(entries["OEBPS/chapter1.html"], "这行应该并入第一章") {
		t.Errorf("excluded 章节正文应并入上一章: %s", entries["OEBPS/chapter1.html"])
	}
	for name, body := range entries {
		if strings.HasPrefix(name, "OEBPS/chapter") && strings.Contains(body, "这章会被彻底删除") {
			t.Errorf("purged 章节正文不应出现在产物 %s 中", name)
		}
	}
}

func TestInsertChapterAtLine(t *testing.T) {
	dir := t.TempDir()
	app := &App{ctx: context.Background(), version: "test", workDir: dir}

	txtPath := filepath.Join(dir, "insert.txt")
	raw := "第1章 开端\n段落一\n手动切分点\n段落二\n"
	if err := os.WriteFile(txtPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := app.LoadTxtFile(txtPath); err != nil {
		t.Fatal(err)
	}
	cfg := app.LoadDesktopConfig()
	chs, err := app.DetectChapters(cfg.Split)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := app.InsertChapterAtLine(chs, 3, "第2章 手动插入", cfg.Split)
	if err != nil {
		t.Fatalf("InsertChapterAtLine: %v", err)
	}
	if len(updated) != 2 || updated[1].Title != "第2章 手动插入" || updated[1].StartLine != 3 {
		t.Fatalf("插入结果异常: %+v", updated)
	}
}
