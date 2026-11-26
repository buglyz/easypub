package main

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/easypub/go-easypub/internal/converter"
	"github.com/easypub/go-easypub/internal/txt"
	"github.com/easypub/go-easypub/internal/util"
)

// LoadTxtFile 读取本地 TXT 文件，缓存解码后的行数据并返回书籍摘要。
func (a *App) LoadTxtFile(path string) (*LoadedBook, error) {
	if path == "" {
		return nil, fmt.Errorf("文件路径为空")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("无法访问文件: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("请选择 TXT 文件而非目录")
	}

	text, enc, err := txt.ReadFileEncoded(path)
	if err != nil {
		return nil, fmt.Errorf("读取并识别编码失败: %w", err)
	}
	norm := util.NormalizeEOL(text)
	lines := strings.Split(norm, "\n")

	totalChars := 0
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		totalChars += utf8.RuneCountInString(strings.TrimLeft(trimmed, "\u3000 "))
	}

	a.mu.Lock()
	a.currentPath = path
	a.currentEnc = enc
	a.currentText = norm
	a.lines = lines
	a.mu.Unlock()

	return &LoadedBook{
		Path:        path,
		FileName:    filepath.Base(path),
		SizeBytes:   info.Size(),
		Encoding:    enc,
		TotalLines:  len(lines),
		TotalChars:  totalChars,
		InferredTit: converter.InferTitle(norm, path),
	}, nil
}

// DetectChapters 使用当前缓存的 TXT 与分章配置识别章节列表。
func (a *App) DetectChapters(cfg SplitConfig) ([]ChapterItem, error) {
	a.mu.RLock()
	text := a.currentText
	totalLines := len(a.lines)
	a.mu.RUnlock()

	if text == "" {
		return nil, fmt.Errorf("请先导入 TXT 文件")
	}

	opt := splitConfigToTxtOptions(cfg)
	chapters, err := txt.Parse(text, opt)
	if err != nil {
		return nil, err
	}

	items := make([]ChapterItem, 0, len(chapters))
	for i, c := range chapters {
		start := c.StartLine
		if start < 1 {
			start = 1
		}
		end := start + c.LineCount - 1
		if end < start {
			end = start
		}
		if end > totalLines {
			end = totalLines
		}
		items = append(items, ChapterItem{
			ID:        i + 1,
			Title:     c.Title,
			StartLine: start,
			EndLine:   end,
			LineCount: end - start + 1,
			CharCount: c.CharCount,
			State:     "normal",
		})
	}
	return items, nil
}

// GetChapterPreview 返回指定行区间的排版段落与原始文本预览（限制最大段落数，防大章卡顿）。
func (a *App) GetChapterPreview(item ChapterItem, cfg SplitConfig) (*ChapterPreview, error) {
	a.mu.RLock()
	lines := a.lines
	a.mu.RUnlock()

	if len(lines) == 0 {
		return nil, fmt.Errorf("尚未加载文本")
	}
	startIdx, endIdx := clampLineRange(item.StartLine, item.EndLine, len(lines))

	// 若该章带标题且首行是标题行，正文从 startIdx+1 开始（与 txt.Parse 对齐）。
	bodyStart := startIdx
	if strings.TrimSpace(item.Title) != "" && startIdx < endIdx {
		if strings.TrimSpace(lines[startIdx]) == strings.TrimSpace(item.Title) || !item.Custom {
			if cfg.Mode != "count" && cfg.Mode != "whole" {
				bodyStart = startIdx + 1
			}
		}
	}

	opt := splitConfigToTxtOptions(cfg)
	escapedBody, charCount := txt.AssembleBody(lines, bodyStart, endIdx, opt)

	const maxPreviewParas = 300
	truncated := false
	if len(escapedBody) > maxPreviewParas {
		escapedBody = escapedBody[:maxPreviewParas]
		truncated = true
	}
	paragraphs := make([]string, len(escapedBody))
	for i, p := range escapedBody {
		paragraphs[i] = html.UnescapeString(p)
	}

	const maxRawLines = 300
	rawEnd := endIdx
	if rawEnd-startIdx > maxRawLines {
		rawEnd = startIdx + maxRawLines
		truncated = true
	}
	rawLines := make([]string, 0, rawEnd-startIdx)
	for i := startIdx; i < rawEnd; i++ {
		rawLines = append(rawLines, lines[i])
	}

	return &ChapterPreview{
		Title:      item.Title,
		StartLine:  startIdx + 1,
		EndLine:    endIdx,
		CharCount:  charCount,
		Paragraphs: paragraphs,
		RawLines:   rawLines,
		Truncated:  truncated,
	}, nil
}

// InsertChapterAtLine 在指定 1-based 行号处手动插入新章节，并自动重算相邻章节的行区间与字数。
func (a *App) InsertChapterAtLine(existing []ChapterItem, lineNo int, title string, cfg SplitConfig) ([]ChapterItem, error) {
	a.mu.RLock()
	lines := a.lines
	a.mu.RUnlock()

	if len(lines) == 0 {
		return nil, fmt.Errorf("请先导入 TXT 文件")
	}
	if lineNo < 1 || lineNo > len(lines) {
		return nil, fmt.Errorf("行号越界，允许范围 1 ~ %d", len(lines))
	}
	t := strings.TrimSpace(title)
	if t == "" {
		t = strings.TrimSpace(lines[lineNo-1])
	}
	if t == "" {
		t = fmt.Sprintf("自定义章节(第%d行)", lineNo)
	}

	nextID := 1
	for _, it := range existing {
		if it.StartLine == lineNo {
			return nil, fmt.Errorf("第 %d 行已存在章节起点", lineNo)
		}
		if it.ID >= nextID {
			nextID = it.ID + 1
		}
	}

	updated := make([]ChapterItem, len(existing)+1)
	copy(updated, existing)
	updated[len(existing)] = ChapterItem{
		ID:        nextID,
		Title:     t,
		StartLine: lineNo,
		State:     "normal",
		Custom:    true,
	}
	sort.Slice(updated, func(i, j int) bool {
		return updated[i].StartLine < updated[j].StartLine
	})

	opt := splitConfigToTxtOptions(cfg)
	for i := range updated {
		endLine := len(lines)
		if i+1 < len(updated) {
			endLine = updated[i+1].StartLine - 1
		}
		if endLine < updated[i].StartLine {
			endLine = updated[i].StartLine
		}
		updated[i].EndLine = endLine
		updated[i].LineCount = endLine - updated[i].StartLine + 1
		bodyStart := updated[i].StartLine - 1
		if strings.TrimSpace(updated[i].Title) != "" && cfg.Mode != "count" && cfg.Mode != "whole" && bodyStart+1 < endLine {
			bodyStart++
		}
		_, updated[i].CharCount = txt.AssembleBody(lines, bodyStart, endLine, opt)
	}
	return updated, nil
}

// splitConfigToTxtOptions 将前端分章配置转换为 internal/txt.Options。
func splitConfigToTxtOptions(cfg SplitConfig) txt.Options {
	opt := txt.Options{
		RemoveBlankLine:   cfg.RemoveBlankLine,
		AddSpace:          cfg.AddSpace,
		AddSpaceCount:     cfg.AddSpaceCount,
		ForceEmptyChapter: cfg.ForceEmptyChapter,
		AdditionalReg:     filterNonEmpty(cfg.AdditionalReg),
	}
	if opt.AddSpace && opt.AddSpaceCount <= 0 {
		opt.AddSpaceCount = 1
	}
	switch cfg.Mode {
	case "count":
		opt.SplitMode = 1
		opt.SplitCount = cfg.SplitCount
		if opt.SplitCount < 100 {
			opt.SplitCount = 10000
		}
	case "whole":
		opt.SplitMode = 2
	case "full":
		opt.SplitMode = 0
		opt.FullReg = strings.TrimSpace(cfg.FullReg)
		opt.AutoMark = opt.FullReg == ""
	case "simple":
		opt.SplitMode = 0
		opt.SimpleRegP1 = strings.TrimSpace(cfg.SimpleRegP1)
		opt.SimpleRegP2 = cfg.SimpleRegP2
		opt.SimpleRegP3 = strings.TrimSpace(cfg.SimpleRegP3)
		opt.SimpleRegExt = strings.TrimSpace(cfg.SimpleRegExt)
		opt.SimpleRegLeadingSpace = cfg.SimpleRegLeadingSpace
		opt.AutoMark = opt.SimpleRegP1 == ""
	default:
		opt.SplitMode = 0
		opt.AutoMark = true
	}
	return opt
}

func clampLineRange(startLine, endLine, total int) (int, int) {
	s := startLine - 1
	if s < 0 {
		s = 0
	}
	if s >= total {
		s = total - 1
	}
	e := endLine
	if e <= s {
		e = s + 1
	}
	if e > total {
		e = total
	}
	return s, e
}

func filterNonEmpty(list []string) []string {
	out := make([]string, 0, len(list))
	for _, s := range list {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}
