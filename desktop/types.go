package main

// SplitConfig 描述章节切分相关参数（与原版 EasyPub config.xml 对齐）。
type SplitConfig struct {
	Mode                  string   `json:"mode"`                  // auto | simple | full | count | whole
	SplitCount            int      `json:"splitCount"`            // 按字数分章时的每章字数
	FullReg               string   `json:"fullReg"`               // 完整正则
	SimpleRegP1           string   `json:"simpleRegP1"`           // 简易正则前段
	SimpleRegP2           int      `json:"simpleRegP2"`           // 0=阿拉伯数字, 1=中文数字
	SimpleRegP3           string   `json:"simpleRegP3"`           // 简易正则尾段
	SimpleRegExt          string   `json:"simpleRegExt"`          // 简易正则扩展尾段
	SimpleRegLeadingSpace bool     `json:"simpleRegLeadingSpace"` // 允许行首空白
	AdditionalReg         []string `json:"additionalReg"`         // 附加正则列表
	RemoveBlankLine       bool     `json:"removeBlankLine"`       // 删除空行
	AddSpace              bool     `json:"addSpace"`              // 段间加空行
	AddSpaceCount         int      `json:"addSpaceCount"`         // 空行数
	ForceEmptyChapter     bool     `json:"forceEmptyChapter"`     // 保留空章节
}

// StyleConfig 描述排版与阅读器字体参数。
type StyleConfig struct {
	LineHeight int     `json:"lineHeight"` // 行高 %
	FontSize   int     `json:"fontSize"`   // 字号 %
	MarginTop  int     `json:"marginTop"`  // 段前距 px
	Indent     float64 `json:"indent"`     // 首行缩进 em
	TextAlign  int     `json:"textAlign"`  // 0=justify,1=left,2=center,3=right
	FontIndex  int     `json:"fontIndex"`  // ereaders.xml 阅读器索引
}

// AdvancedConfig 描述 MOBI / 输出与窗口设置。
type AdvancedConfig struct {
	EnableMobi        bool   `json:"enableMobi"`
	OutputToSrc       bool   `json:"outputToSrc"`
	OutputFolder      string `json:"outputFolder"`
	KindleGenExe      string `json:"kindleGenExe"`
	KindleGenCompress int    `json:"kindleGenCompress"` // 0/1/2
	KindleGenOption   string `json:"kindleGenOption"`
	AlwaysOnTop       bool   `json:"alwaysOnTop"`
}

// EReaderOption 描述 ereaders.xml 中的阅读器字体方案。
type EReaderOption struct {
	Index int      `json:"index"`
	Name  string   `json:"name"`
	Fonts []string `json:"fonts"`
}

// PresetConfig 描述 config.xml 中预定义正则模板列表。
type PresetConfig struct {
	FullRegs       []string `json:"fullRegs"`
	AdditionalRegs []string `json:"additionalRegs"`
}

// DesktopConfig 是前后端同步的完整桌面配置快照。
type DesktopConfig struct {
	WorkDir      string          `json:"workDir"`
	ConfigPath   string          `json:"configPath"`
	EReadersPath string          `json:"ereadersPath"`
	Split        SplitConfig     `json:"split"`
	Style        StyleConfig     `json:"style"`
	Advanced     AdvancedConfig  `json:"advanced"`
	Presets      PresetConfig    `json:"presets"`
	EReaders     []EReaderOption `json:"ereaders"`
}

// LoadedBook 描述已导入的 TXT 文件摘要。
type LoadedBook struct {
	Path        string `json:"path"`
	FileName    string `json:"fileName"`
	SizeBytes   int64  `json:"sizeBytes"`
	Encoding    string `json:"encoding"`
	TotalLines  int    `json:"totalLines"`
	TotalChars  int    `json:"totalChars"`
	InferredTit string `json:"inferredTitle"`
}

// ChapterItem 是可视化章节管理器中的单条章节记录。
// 状态: normal(正常) | excluded(移出目录/合并到上一章) | purged(彻底删除正文)
type ChapterItem struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	StartLine int    `json:"startLine"` // 1-based 原始起始行
	EndLine   int    `json:"endLine"`   // 1-based 原始结束行(含)
	LineCount int    `json:"lineCount"`
	CharCount int    `json:"charCount"`
	State     string `json:"state"`
	Custom    bool   `json:"custom"` // 手动按行号插入的章节
}

// ChapterPreview 返回单章排版段落与原始行文本。
type ChapterPreview struct {
	Title      string   `json:"title"`
	StartLine  int      `json:"startLine"`
	EndLine    int      `json:"endLine"`
	CharCount  int      `json:"charCount"`
	Paragraphs []string `json:"paragraphs"`
	RawLines   []string `json:"rawLines"`
	Truncated  bool     `json:"truncated"`
}

// CoverInfo 描述自定义封面图片预览。
type CoverInfo struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	DataURL string `json:"dataUrl"`
}

// ConvertRequest 是桌面端单本转换请求。
type ConvertRequest struct {
	Title          string         `json:"title"`
	Author         string         `json:"author"`
	OutputPath     string         `json:"outputPath"`
	CoverImagePath string         `json:"coverImagePath"`
	Split          SplitConfig    `json:"split"`
	Style          StyleConfig    `json:"style"`
	Advanced       AdvancedConfig `json:"advanced"`
	Chapters       []ChapterItem  `json:"chapters"`
}

// ConvertResponse 是单本转换结果。
type ConvertResponse struct {
	EpubPath     string `json:"epubPath"`
	MobiPath     string `json:"mobiPath"`
	ChapterCount int    `json:"chapterCount"`
	Encoding     string `json:"encoding"`
	DurationMs   int64  `json:"durationMs"`
}

// BatchRequest 是批量目录转换请求。
type BatchRequest struct {
	InputDir     string         `json:"inputDir"`
	OutputDir    string         `json:"outputDir"`
	Author       string         `json:"author"`
	SkipExisting bool           `json:"skipExisting"`
	Jobs         int            `json:"jobs"`
	EnableMobi   bool           `json:"enableMobi"`
	FontIndex    int            `json:"fontIndex"`
	ConfigPath   string         `json:"configPath"`
	EReadersPath string         `json:"ereadersPath"`
	Advanced     AdvancedConfig `json:"advanced"`
}

// BatchItemResult 是批量模式单文件结果。
type BatchItemResult struct {
	Input    string `json:"input"`
	FileName string `json:"fileName"`
	Output   string `json:"output"`
	Encoding string `json:"encoding"`
	Chapters int    `json:"chapters"`
	Status   string `json:"status"` // ok | skipped | failed
	Error    string `json:"error"`
}

// BatchResponse 是批量目录转换汇总。
type BatchResponse struct {
	OutputDir  string            `json:"outputDir"`
	Total      int               `json:"total"`
	Succeeded  int               `json:"succeeded"`
	Skipped    int               `json:"skipped"`
	Failed     int               `json:"failed"`
	DurationMs int64             `json:"durationMs"`
	Items      []BatchItemResult `json:"items"`
}
