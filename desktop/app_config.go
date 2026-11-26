package main

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/easypub/go-easypub/internal/config"
)

// LoadDesktopConfig 加载当前工作目录的 config.xml 与 ereaders.xml；不存在时返回合理默认值。
func (a *App) LoadDesktopConfig() DesktopConfig {
	cfgPath := filepath.Join(a.workDir, "config.xml")
	return a.loadDesktopConfigFrom(cfgPath)
}

// LoadDesktopConfigFrom 从用户指定的 config.xml 加载桌面配置。
func (a *App) LoadDesktopConfigFrom(cfgPath string) (DesktopConfig, error) {
	if !fileExists(cfgPath) {
		return DesktopConfig{}, fmt.Errorf("配置文件不存在: %s", cfgPath)
	}
	if _, err := config.Load(cfgPath); err != nil {
		return DesktopConfig{}, fmt.Errorf("解析 config.xml 失败: %w", err)
	}
	return a.loadDesktopConfigFrom(cfgPath), nil
}

func (a *App) loadDesktopConfigFrom(cfgPath string) DesktopConfig {
	out := defaultDesktopConfig(a.workDir)
	out.ConfigPath = cfgPath

	cfgDir := filepath.Dir(cfgPath)
	var root *config.Root
	if fileExists(cfgPath) {
		if c, err := config.Load(cfgPath); err == nil {
			root = c
		}
	}

	if root != nil {
		applyRootToDesktop(&out, root)
	}

	erName := "ereaders.xml"
	if root != nil && root.EReadersConfig != "" {
		erName = root.EReadersConfig
	}
	erPath := filepath.Join(cfgDir, erName)
	if !fileExists(erPath) {
		erPath = filepath.Join(a.workDir, "ereaders.xml")
	}
	out.EReadersPath = erPath
	if fileExists(erPath) {
		if er, err := config.LoadEReadersFile(erPath); err == nil && len(er.Models) > 0 {
			models := make([]EReaderOption, 0, len(er.Models))
			for i, m := range er.Models {
				models = append(models, EReaderOption{
					Index: i,
					Name:  m.Name,
					Fonts: m.Fonts,
				})
			}
			out.EReaders = models
		}
	}
	if out.Style.FontIndex < 0 || out.Style.FontIndex >= len(out.EReaders) {
		out.Style.FontIndex = 0
	}
	return out
}

// SaveDesktopConfig 将当前桌面配置保存回 config.xml（与原版 EasyPub 格式完全兼容）。
func (a *App) SaveDesktopConfig(cfg DesktopConfig) error {
	target := cfg.ConfigPath
	if target == "" {
		target = filepath.Join(a.workDir, "config.xml")
	}

	// 若原文件存在则基于原 Root 增量回写，保留未暴露的高级字段。
	root := &config.Root{
		XMLVersion:     "2",
		EReadersConfig: "ereaders.xml",
	}
	if fileExists(target) {
		if existing, err := config.Load(target); err == nil {
			root = existing
		}
	}

	applyDesktopToRoot(root, cfg)
	return config.Save(target, root)
}

func defaultDesktopConfig(workDir string) DesktopConfig {
	return DesktopConfig{
		WorkDir:      workDir,
		ConfigPath:   filepath.Join(workDir, "config.xml"),
		EReadersPath: filepath.Join(workDir, "ereaders.xml"),
		Split: SplitConfig{
			Mode:                  "auto",
			SplitCount:            10000,
			FullReg:               "",
			SimpleRegP1:           "[第卷]",
			SimpleRegP2:           0,
			SimpleRegP3:           "[章回卷节集部]",
			SimpleRegExt:          "",
			SimpleRegLeadingSpace: true,
			AdditionalReg:         []string{},
			RemoveBlankLine:       true,
			AddSpace:              false,
			AddSpaceCount:         1,
			ForceEmptyChapter:     true,
		},
		Style: StyleConfig{
			LineHeight: 120,
			FontSize:   100,
			MarginTop:  5,
			Indent:     0,
			TextAlign:  0,
			FontIndex:  0,
		},
		Advanced: AdvancedConfig{
			EnableMobi:        false,
			OutputToSrc:       true,
			OutputFolder:      "",
			KindleGenExe:      "kindlegen_v2.9.exe",
			KindleGenCompress: 1,
			KindleGenOption:   "",
			AlwaysOnTop:       false,
		},
		Presets: PresetConfig{
			FullRegs: []string{
				`^\s*[第卷][0123456789一二三四五六七八九十零〇百千两]*[章回部节集卷].*`,
				`^\s*Chapter\s*[0123456789]+.*`,
			},
			AdditionalRegs: []string{
				`^\s*(简介|序言|序[1-9]|序曲|后记|尾声)`,
				`^\s*(前言|自序|附录|楔子|番外)`,
			},
		},
		EReaders: []EReaderOption{
			{
				Index: 0,
				Name:  "Nook(和其它Android设备)",
				Fonts: []string{
					"res:///system/fonts/DroidSansFallback.ttf",
					"res:///ebook/fonts/../../system/fonts/DroidSansFallback.ttf",
				},
			},
		},
	}
}

func applyRootToDesktop(out *DesktopConfig, root *config.Root) {
	r := root.Recent
	adv := root.Advanced

	switch r.SplitMode {
	case 1:
		out.Split.Mode = "count"
	case 2:
		out.Split.Mode = "whole"
	default:
		if r.AutoMark == 1 {
			out.Split.Mode = "auto"
		} else if strings.TrimSpace(r.FullReg) != "" {
			out.Split.Mode = "full"
		} else if strings.TrimSpace(r.SimpleRegP1) != "" {
			out.Split.Mode = "simple"
		} else {
			out.Split.Mode = "auto"
		}
	}
	if r.SplitCount >= 100 {
		out.Split.SplitCount = r.SplitCount
	}
	out.Split.FullReg = strings.TrimSpace(r.FullReg)
	if strings.TrimSpace(r.SimpleRegP1) != "" {
		out.Split.SimpleRegP1 = strings.TrimSpace(r.SimpleRegP1)
	}
	out.Split.SimpleRegP2 = r.SimpleRegP2
	if strings.TrimSpace(r.SimpleRegP3) != "" {
		out.Split.SimpleRegP3 = strings.TrimSpace(r.SimpleRegP3)
	}
	out.Split.SimpleRegExt = strings.TrimSpace(r.SimpleRegExt)
	out.Split.SimpleRegLeadingSpace = r.SimpleRegLeadSpace == 1
	out.Split.RemoveBlankLine = r.RemoveBlankLine == 1
	out.Split.AddSpace = r.AddSpace == 1
	if r.AddSpaceCount > 0 {
		out.Split.AddSpaceCount = r.AddSpaceCount
	}
	out.Split.ForceEmptyChapter = adv.ForceEmptyChapter == 1

	if len(root.MyRegExp.FullReg) > 0 {
		out.Presets.FullRegs = root.MyRegExp.FullReg
	}
	if len(root.MyRegExp.AdditionalReg) > 0 {
		out.Presets.AdditionalRegs = root.MyRegExp.AdditionalReg
		out.Split.AdditionalReg = root.MyRegExp.AdditionalReg
	}

	if r.LineHeight > 0 {
		out.Style.LineHeight = r.LineHeight
	}
	if r.FontSize > 0 {
		out.Style.FontSize = r.FontSize
	}
	if r.MarginTop >= 0 {
		out.Style.MarginTop = r.MarginTop
	}
	if r.Indent > 0 {
		out.Style.Indent = float64(r.Indent) / 10.0
	}
	if r.TextAlign >= 0 && r.TextAlign <= 3 {
		out.Style.TextAlign = r.TextAlign
	}
	if r.MachineID >= 0 {
		out.Style.FontIndex = r.MachineID
	}

	out.Advanced.OutputFolder = strings.TrimSpace(r.OutputFolder)
	out.Advanced.OutputToSrc = adv.OutputToSrc == 1 || out.Advanced.OutputFolder == ""
	out.Advanced.EnableMobi = adv.OutputFormat == 2
	if strings.TrimSpace(adv.KindleGenExe) != "" {
		out.Advanced.KindleGenExe = strings.TrimSpace(adv.KindleGenExe)
	}
	out.Advanced.KindleGenCompress = adv.KindleGenCompress
	out.Advanced.KindleGenOption = strings.TrimSpace(adv.KindleGenOption)
	out.Advanced.AlwaysOnTop = adv.AlwaysOnTop == 1
}

func applyDesktopToRoot(root *config.Root, cfg DesktopConfig) {
	switch cfg.Split.Mode {
	case "count":
		root.Recent.SplitMode = 1
		root.Recent.AutoMark = 0
	case "whole":
		root.Recent.SplitMode = 2
		root.Recent.AutoMark = 0
	case "full":
		root.Recent.SplitMode = 0
		root.Recent.AutoMark = 0
	case "simple":
		root.Recent.SplitMode = 0
		root.Recent.AutoMark = 0
	default:
		root.Recent.SplitMode = 0
		root.Recent.AutoMark = 1
	}
	root.Recent.SplitCount = cfg.Split.SplitCount
	root.Recent.FullReg = cfg.Split.FullReg
	root.Recent.SimpleRegP1 = cfg.Split.SimpleRegP1
	root.Recent.SimpleRegP2 = cfg.Split.SimpleRegP2
	root.Recent.SimpleRegP3 = cfg.Split.SimpleRegP3
	root.Recent.SimpleRegExt = cfg.Split.SimpleRegExt
	root.Recent.SimpleRegLeadSpace = boolToInt(cfg.Split.SimpleRegLeadingSpace)
	root.Recent.RemoveBlankLine = boolToInt(cfg.Split.RemoveBlankLine)
	root.Recent.AddSpace = boolToInt(cfg.Split.AddSpace)
	root.Recent.AddSpaceCount = cfg.Split.AddSpaceCount
	root.Advanced.ForceEmptyChapter = boolToInt(cfg.Split.ForceEmptyChapter)

	if len(cfg.Split.AdditionalReg) > 0 {
		root.MyRegExp.AdditionalReg = cfg.Split.AdditionalReg
	}

	root.Recent.LineHeight = cfg.Style.LineHeight
	root.Recent.FontSize = cfg.Style.FontSize
	root.Recent.MarginTop = cfg.Style.MarginTop
	root.Recent.Indent = int(math.Round(cfg.Style.Indent * 10.0))
	root.Recent.TextAlign = cfg.Style.TextAlign
	root.Recent.MachineID = cfg.Style.FontIndex

	root.Recent.OutputFolder = cfg.Advanced.OutputFolder
	root.Advanced.OutputToSrc = boolToInt(cfg.Advanced.OutputToSrc)
	if cfg.Advanced.EnableMobi {
		root.Advanced.OutputFormat = 2
	} else {
		root.Advanced.OutputFormat = 1
	}
	root.Advanced.KindleGenExe = cfg.Advanced.KindleGenExe
	root.Advanced.KindleGenCompress = cfg.Advanced.KindleGenCompress
	root.Advanced.KindleGenOption = cfg.Advanced.KindleGenOption
	root.Advanced.AlwaysOnTop = boolToInt(cfg.Advanced.AlwaysOnTop)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
