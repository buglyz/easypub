package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/easypub/go-easypub/internal/batch"
	"github.com/easypub/go-easypub/internal/converter"
	"github.com/easypub/go-easypub/internal/txt"
)

// ConvertCurrent 将当前加载的 TXT（结合用户在章节管理器中的编辑结果）生成 EPUB/MOBI。
func (a *App) ConvertCurrent(req ConvertRequest) (*ConvertResponse, error) {
	a.mu.RLock()
	srcPath := a.currentPath
	enc := a.currentEnc
	text := a.currentText
	lines := a.lines
	a.mu.RUnlock()

	if srcPath == "" || text == "" {
		return nil, fmt.Errorf("请先导入 TXT 文件")
	}

	start := time.Now()
	outPath := resolveOutputPath(srcPath, req.OutputPath, req.Advanced)
	txtOpt := splitConfigToTxtOptions(req.Split)

	// 若前端传入了可视化编辑后的章节列表，按 normal / excluded(合并入上一章) / purged(彻底删除) 组装最终章节。
	var chaptersOverride []txt.Chapter
	if len(req.Chapters) > 0 {
		chaptersOverride = buildEditedChapters(lines, req.Chapters, req.Split, txtOpt)
		if len(chaptersOverride) == 0 {
			return nil, fmt.Errorf("所有章节均已被彻底删除，至少需要保留一章正文")
		}
	}

	cfg := a.LoadDesktopConfig()
	co := converter.Options{
		Input:             srcPath,
		Output:            outPath,
		Title:             strings.TrimSpace(req.Title),
		Author:            strings.TrimSpace(req.Author),
		BaseDir:           a.workDir,
		EReadersPath:      cfg.EReadersPath,
		FontIndex:         req.Style.FontIndex,
		EnableMobi:        req.Advanced.EnableMobi,
		Quiet:             true,
		CoverImagePath:    strings.TrimSpace(req.CoverImagePath),
		ChaptersOverride:  chaptersOverride,
		TXTOverrides:      &txtOpt,
		KindleGenExe:      strings.TrimSpace(req.Advanced.KindleGenExe),
		KindleGenCompress: req.Advanced.KindleGenCompress,
		KindleGenOption:   strings.TrimSpace(req.Advanced.KindleGenOption),
		CSSOverrides: &converter.CSSOverrides{
			LineHeight:   req.Style.LineHeight,
			FontSize:     req.Style.FontSize,
			MarginTop:    req.Style.MarginTop,
			MarginTopSet: true,
			TextAlign:    req.Style.TextAlign,
			Indent:       req.Style.Indent,
		},
	}

	res, err := converter.ConvertText(a.ctx, co, text, enc)
	if err != nil {
		return nil, err
	}
	return &ConvertResponse{
		EpubPath:     res.EpubPath,
		MobiPath:     res.MobiPath,
		ChapterCount: res.ChapterCount,
		Encoding:     res.Encoding,
		DurationMs:   time.Since(start).Milliseconds(),
	}, nil
}

// RunBatchConvert 执行批量目录并发转换并返回逐文件结果。
func (a *App) RunBatchConvert(req BatchRequest) (*BatchResponse, error) {
	if strings.TrimSpace(req.InputDir) == "" {
		return nil, fmt.Errorf("请选择输入目录")
	}
	start := time.Now()
	jobs := req.Jobs
	if jobs <= 0 {
		jobs = runtime.NumCPU()
	}

	outDir := strings.TrimSpace(req.OutputDir)
	if outDir == "" {
		outDir = req.InputDir
	}

	cfgPath := strings.TrimSpace(req.ConfigPath)
	if cfgPath != "" && !fileExists(cfgPath) {
		cfgPath = ""
	}
	erPath := strings.TrimSpace(req.EReadersPath)
	if erPath != "" && !fileExists(erPath) {
		erPath = ""
	}

	results, err := batch.RunContext(a.ctx, batch.Options{
		InputDir:     req.InputDir,
		OutputDir:    outDir,
		Author:       strings.TrimSpace(req.Author),
		ConfigPath:   cfgPath,
		EReadersPath: erPath,
		FontIndex:    req.FontIndex,
		EnableMobi:   req.EnableMobi,
		Quiet:        true,
		SkipExisting: req.SkipExisting,
		Jobs:         jobs,
	})
	if err != nil {
		return nil, err
	}

	resp := &BatchResponse{
		OutputDir:  outDir,
		Total:      len(results),
		DurationMs: time.Since(start).Milliseconds(),
		Items:      make([]BatchItemResult, 0, len(results)),
	}
	for _, r := range results {
		item := BatchItemResult{
			Input:    r.Input,
			FileName: filepath.Base(r.Input),
			Output:   r.Output,
			Encoding: r.Encoding,
			Chapters: r.Chapters,
		}
		switch {
		case r.Err == nil:
			item.Status = "ok"
			resp.Succeeded++
		case errors.Is(r.Err, batch.ErrSkipped):
			item.Status = "skipped"
			item.Error = "目标已存在，已跳过"
			resp.Skipped++
		default:
			item.Status = "failed"
			item.Error = r.Err.Error()
			resp.Failed++
		}
		resp.Items = append(resp.Items, item)
	}
	return resp, nil
}

// buildEditedChapters 根据可视化编辑结果组装最终章节列表：
//   - purged：彻底丢弃该章标题与正文
//   - excluded：移出目录，其原始行（含原标题行）并入上一章正文尾部；若处于首章则以无标题前置章保留
//   - normal：保留为独立章节，使用编辑后的 Title
func buildEditedChapters(lines []string, items []ChapterItem, cfg SplitConfig, opt txt.Options) []txt.Chapter {
	var out []txt.Chapter
	for _, it := range items {
		if it.State == "purged" {
			continue
		}
		startIdx, endIdx := clampLineRange(it.StartLine, it.EndLine, len(lines))

		if it.State == "excluded" {
			body, count := txt.AssembleBody(lines, startIdx, endIdx, opt)
			if len(body) == 0 {
				continue
			}
			if len(out) > 0 {
				last := &out[len(out)-1]
				last.Body = append(last.Body, body...)
				last.LineCount += endIdx - startIdx
				last.CharCount += count
			} else {
				out = append(out, txt.Chapter{
					Title:     "",
					Body:      body,
					StartLine: startIdx + 1,
					LineCount: endIdx - startIdx,
					CharCount: count,
				})
			}
			continue
		}

		bodyStart := startIdx
		if strings.TrimSpace(it.Title) != "" && cfg.Mode != "count" && cfg.Mode != "whole" && startIdx < endIdx {
			bodyStart = startIdx + 1
		}
		body, count := txt.AssembleBody(lines, bodyStart, endIdx, opt)
		if len(body) == 0 && !opt.ForceEmptyChapter {
			continue
		}
		out = append(out, txt.Chapter{
			Title:     strings.TrimSpace(it.Title),
			Body:      body,
			StartLine: startIdx + 1,
			LineCount: endIdx - startIdx,
			CharCount: count,
		})
	}
	return out
}

func resolveOutputPath(srcPath, explicitOut string, adv AdvancedConfig) string {
	if strings.TrimSpace(explicitOut) != "" {
		p := strings.TrimSpace(explicitOut)
		if !strings.HasSuffix(strings.ToLower(p), ".epub") {
			p += ".epub"
		}
		return p
	}
	stem := strings.TrimSuffix(filepath.Base(srcPath), filepath.Ext(srcPath))
	outDir := filepath.Dir(srcPath)
	if !adv.OutputToSrc && strings.TrimSpace(adv.OutputFolder) != "" {
		outDir = strings.TrimSpace(adv.OutputFolder)
	}
	return filepath.Join(outDir, stem+".epub")
}
