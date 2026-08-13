package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunBatch_CountsSkipsAndFailures 验证 runBatch 的汇总统计
// （成功/失败/跳过）与退出错误语义。
func TestRunBatch_CountsSkipsAndFailures(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "书名：测试\n第1章 开端\n　　段落。\n"
	// 一个正常文件。
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// 预置 b.epub，令 b.txt 走跳过分支。
	if err := os.WriteFile(filepath.Join(src, "b.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(out, 0o755)
	if err := os.WriteFile(filepath.Join(out, "b.epub"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 第一次：a 转换、b 跳过。
	if err := runBatch(batchOptions{Input: src, Output: out, SkipExisting: true, Quiet: true}); err != nil {
		t.Fatalf("runBatch 不应报错: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "a.epub")); err != nil {
		t.Errorf("a.epub 应已生成: %v", err)
	}

	// 第二次：全部跳过，不应报错。
	if err := runBatch(batchOptions{Input: src, Output: out, SkipExisting: true, Quiet: true}); err != nil {
		t.Fatalf("全部跳过时不应报错: %v", err)
	}

	// 非法 config → 全部失败，应返回失败错误。
	bad := filepath.Join(dir, "bad.xml")
	if err := os.WriteFile(bad, []byte("not xml"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runBatch(batchOptions{Input: src, Output: out, ConfigPath: bad, Quiet: true})
	if err == nil || !strings.Contains(err.Error(), "失败") {
		t.Errorf("有文件失败时应返回错误, got %v", err)
	}
}
