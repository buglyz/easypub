package batch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleContent = "书名：测试书\n第1章 开端\n　　段落一。\n第2章 发展\n　　段落二。\n"

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func TestRun_ConvertsAllTxt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), sampleContent)
	writeFile(t, filepath.Join(dir, "b.txt"), sampleContent)
	// 非目标扩展名不应参与。
	writeFile(t, filepath.Join(dir, "readme.md"), "# hi")

	results, err := Run(Options{InputDir: dir})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("结果数=%d 期望 2", len(results))
	}
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("文件 %s 转换失败: %v", r.Input, r.Err)
		}
		if r.Chapters <= 0 {
			t.Errorf("文件 %s 章节数=%d 期望 >0", r.Input, r.Chapters)
		}
		if _, err := os.Stat(r.Output); err != nil {
			t.Errorf("输出缺失 %s: %v", r.Output, err)
		}
	}
	// 输出与输入同目录（未指定 OutputDir）。
	if _, err := os.Stat(filepath.Join(dir, "a.epub")); err != nil {
		t.Errorf("默认输出目录应为输入目录: %v", err)
	}
}

func TestRun_OutputDir(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	writeFile(t, filepath.Join(dir, "a.txt"), sampleContent)

	results, err := Run(Options{InputDir: dir, OutputDir: out})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("结果异常: %+v", results)
	}
	if _, err := os.Stat(results[0].Output); err != nil {
		t.Fatalf("输出缺失 %s: %v", results[0].Output, err)
	}
	if !strings.HasPrefix(results[0].Output, out) {
		t.Errorf("输出应位于输出目录, got %s", results[0].Output)
	}
}

func TestRun_SkipExisting(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), sampleContent)
	// 预置一个 a.epub。
	existing := filepath.Join(dir, "a.epub")
	writeFile(t, existing, "old")

	results, err := Run(Options{InputDir: dir, SkipExisting: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("结果数=%d 期望 1", len(results))
	}
	if results[0].Err != ErrSkipped {
		t.Errorf("期望 ErrSkipped, got %v", results[0].Err)
	}
	// 旧文件不应被覆盖。
	b, _ := os.ReadFile(existing)
	if string(b) != "old" {
		t.Errorf("跳过模式不应覆盖已有 EPUB")
	}
}

func TestRun_ContinuesOnError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), sampleContent)
	writeFile(t, filepath.Join(dir, "b.txt"), sampleContent)
	// 非法 config.xml 使每个文件转换失败,但循环不应中断。
	badCfg := filepath.Join(dir, "bad.xml")
	writeFile(t, badCfg, "not <<< xml")

	results, err := Run(Options{InputDir: dir, ConfigPath: badCfg})
	if err != nil {
		t.Fatalf("Run 不应整体失败: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("结果数=%d 期望 2(单文件失败不应中断)", len(results))
	}
	for _, r := range results {
		if r.Err == nil {
			t.Errorf("非法 config 下文件 %s 应失败", r.Input)
		}
	}
}

func TestRun_NoMatchReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "readme.md"), "# hi")
	if _, err := Run(Options{InputDir: dir}); err == nil {
		t.Fatal("无匹配文件应返回错误")
	}
}

func TestRun_MissingDirReturnsError(t *testing.T) {
	if _, err := Run(Options{InputDir: filepath.Join(t.TempDir(), "nope")}); err == nil {
		t.Fatal("目录不存在应返回错误")
	}
}

func TestRun_ParallelJobsPreservesOrder(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	names := []string{"01.txt", "02.txt", "03.txt", "04.txt", "05.txt", "06.txt"}
	for _, n := range names {
		writeFile(t, filepath.Join(dir, n), sampleContent)
	}

	results, err := Run(Options{InputDir: dir, OutputDir: out, Jobs: 4})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results) != len(names) {
		t.Fatalf("结果数=%d 期望 %d", len(results), len(names))
	}
	for i, r := range results {
		if r.Err != nil {
			t.Errorf("%s 失败: %v", r.Input, r.Err)
		}
		if filepath.Base(r.Input) != names[i] {
			t.Errorf("顺序错乱: index=%d got=%s want=%s", i, filepath.Base(r.Input), names[i])
		}
		if _, err := os.Stat(r.Output); err != nil {
			t.Errorf("输出缺失 %s: %v", r.Output, err)
		}
	}
}
