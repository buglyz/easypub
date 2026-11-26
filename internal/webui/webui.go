// Package webui 提供基于标准库 net/http 的本地 Web 界面，
// 复用 internal/converter 完成 TXT→EPUB 转换。
package webui

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/easypub/go-easypub/internal/converter"
	"github.com/easypub/go-easypub/internal/txt"
)

//go:embed static/*
var staticFS embed.FS

// maxUploadBytes 是上传文件的硬上限(64MB)。
const maxUploadBytes = 64 << 20

// maxConcurrentConverts 限制同时进行的转换数，避免 kindlegen/大文件 OOM。
const maxConcurrentConverts = 2

// outputRetention 是生成产物的最长保留时间，超过即由后台清理删除。
// 服务器长期运行下避免 .easypub-output/ 无限堆积撑满磁盘。
const outputRetention = 24 * time.Hour

// cleanupInterval 是后台清理的扫描周期。
const cleanupInterval = 1 * time.Hour

// shutdownTimeout 是收到 SIGTERM 后等待在途请求的最长时间，
// 之后强制关闭连接。需小于容器编排的 kill grace(通常 30s)。
const shutdownTimeout = 20 * time.Second

// downloadNameRe 限制下载文件名格式，与 outPath 命名一致。
var downloadNameRe = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}-[0-9a-f]{8}\.(epub|mobi)$`)

// Config 描述服务配置。
type Config struct {
	Addr    string // 监听地址,如 "127.0.0.1:8080"
	WorkDir string // 工作目录,用于查找 config.xml/ereaders.xml,也是默认输出目录
	Version string
	// Token 访问令牌；非空时所有业务端点(除 /healthz)要求 X-Auth-Token 头、
	// Authorization: Bearer <token> 或 ?token= 查询参数。用于公网/局域网部署鉴权。
	Token string
}

// Server 是 WebUI 服务。
type Server struct {
	cfg     Config
	outputs string        // 输出/临时目录,保存生成的 epub 供下载
	sem     chan struct{} // 转换并发闸
}

// New 构造 Server。静默创建输出目录。
func New(cfg Config) (*Server, error) {
	if cfg.Addr == "" {
		// 默认仅绑定本机，避免局域网/公网无鉴权暴露。
		cfg.Addr = "127.0.0.1:8080"
	}
	if cfg.WorkDir == "" {
		if cwd, err := os.Getwd(); err == nil {
			cfg.WorkDir = cwd
		}
	}
	outputs := filepath.Join(cfg.WorkDir, ".easypub-output")
	if err := os.MkdirAll(outputs, 0o755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}
	return &Server{
		cfg:     cfg,
		outputs: outputs,
		sem:     make(chan struct{}, maxConcurrentConverts),
	}, nil
}

// Handler 构建包含完整路由与中间件链的 HTTP Handler，供 Run 与单元测试共用。
// 静态页面壳（/、/style.css、/app.js）不含用户数据，放行以便启用 -token 时前端能加载并弹出令牌输入框；
// 所有 /api/* 业务接口均强制校验令牌。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	public := func(h http.HandlerFunc) http.HandlerFunc {
		return s.withRequestLog(s.withSecurityHeaders(h))
	}
	// 中间件顺序：请求日志 → 安全头 → 令牌鉴权 → 业务 handler。
	protected := func(h http.HandlerFunc) http.HandlerFunc {
		return s.withRequestLog(s.withSecurityHeaders(s.withAuth(h)))
	}
	mux.HandleFunc("/", public(s.handleIndex))
	mux.HandleFunc("/api/auth", protected(s.handleAuthCheck))
	mux.HandleFunc("/api/detect", protected(s.handleDetect))
	mux.HandleFunc("/api/convert", protected(s.handleConvert))
	mux.HandleFunc("/api/download/", protected(s.handleDownload))
	// 轻量探活端点，供容器 healthcheck / 反代 / 监控使用。
	// 不带安全头、不鉴权，保证无令牌也能探活。
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

// Run 启动 HTTP 服务并阻塞，直至收到 SIGINT/SIGTERM 或发生致命错误。
// 优雅关闭：收到信号后等待在途请求最多 shutdownTimeout，再强制断开。
// 同时启动后台 goroutine 周期清理过期的输出产物，避免磁盘无限堆积。
func (s *Server) Run() error {
	mux := s.Handler()

	log.Printf("EasyPub WebUI 已启动: http://%s", s.cfg.Addr)
	if s.cfg.Version != "" {
		log.Printf("版本: %s", s.cfg.Version)
	}
	log.Printf("输出目录: %s", s.outputs)
	log.Printf("产物保留 %s，每 %s 清理一次", outputRetention, cleanupInterval)
	if s.cfg.Token != "" {
		log.Printf("已启用令牌鉴权(所有 /api/* 端点)")
	} else {
		log.Printf("警告: 未设置访问令牌,服务无鉴权; 公网部署请加 -token 或反向代理鉴权")
	}
	log.Printf("提示: 默认仅本机可访问；若需局域网访问请显式指定 -addr 0.0.0.0:8080 并注意无鉴权风险")
	srv := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       120 * time.Second,
		// 略大于 kindlegen 的 5 分钟超时，避免合法 MOBI 转换被提前切断。
		WriteTimeout:   320 * time.Second,
		IdleTimeout:    30 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// 后台周期清理过期产物。
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	defer cleanupCancel()
	go s.cleanupLoop(cleanupCtx)

	// 信号监听：SIGINT/SIGTERM 触发优雅关闭。
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		return err
	case sig := <-sigCh:
		log.Printf("收到信号 %s，开始优雅关闭（最长等待 %s）", sig, shutdownTimeout)
		cleanupCancel()
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("优雅关闭失败: %w", err)
		}
		log.Printf("已关闭")
		return nil
	}
}

// cleanupLoop 周期扫描输出目录，删除超过 outputRetention 的 epub/mobi/临时 txt。
// 下载是即时的，旧产物对用户无价值；长期运行下这是唯一的磁盘释放途径。
func (s *Server) cleanupLoop(ctx context.Context) {
	// 启动时先清理一次（可能在上次崩溃后残留）。
	s.cleanupOnce()
	t := time.NewTicker(cleanupInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.cleanupOnce()
		}
	}
}

// cleanupOnce 执行一轮清理。按 mtime 判断，删除过期文件。
func (s *Server) cleanupOnce() {
	entries, err := os.ReadDir(s.outputs)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-outputRetention)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(s.outputs, e.Name()))
		}
	}
}

// withSecurityHeaders 为所有响应附加安全头。
func (s *Server) withSecurityHeaders(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
		next(w, r)
	}
}

// withAuth 令牌鉴权中间件。cfg.Token 为空时不拦截任何请求。
// 令牌来源优先级：X-Auth-Token 头 > Authorization: Bearer <token> > ?token= 查询参数。
// 使用常量时间比较，避免时序侧信道。
func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Token == "" {
			next(w, r)
			return
		}
		if validToken(tokenFromRequest(r), s.cfg.Token) {
			next(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate", `Bearer realm="easypub"`)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "缺少或无效的访问令牌"})
	}
}

// tokenFromRequest 从请求中提取令牌。
func tokenFromRequest(r *http.Request) string {
	if v := r.Header.Get("X-Auth-Token"); v != "" {
		return v
	}
	if v := r.Header.Get("Authorization"); v != "" {
		// 兼容 "Bearer <token>" 形式。
		if len(v) > 7 && strings.EqualFold(v[:7], "Bearer ") {
			return strings.TrimSpace(v[7:])
		}
	}
	if v := r.URL.Query().Get("token"); v != "" {
		return v
	}
	return ""
}

// validToken 常量时间比较两个令牌。
func validToken(got, want string) bool {
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// statusRecorder 记录响应状态码，供请求日志使用。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// withRequestLog 记录每个业务请求的方法/路径/状态码/耗时与来源 IP。
// /healthz 不记录，避免探活刷屏。
func (s *Server) withRequestLog(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next(rec, r)
		log.Printf("%s %s %s -> %d (%s)",
			sanitizeLogField(clientIP(r)),
			sanitizeLogField(r.Method),
			sanitizeLogField(r.URL.Path),
			rec.status,
			time.Since(start).Round(time.Millisecond),
		)
	}
}

// sanitizeLogField 清理控制字符与换行符，防止日志注入。
func sanitizeLogField(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return ' '
		}
		return r
	}, s)
}

// clientIP 取来源 IP：优先 X-Forwarded-For(反代后)，否则 RemoteAddr。
// X-Forwarded-For 可能被伪造，仅用于日志展示。
func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.IndexByte(v, ','); i >= 0 {
			return strings.TrimSpace(v[:i])
		}
		return strings.TrimSpace(v)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// handleIndex 提供内嵌前端页面与静态资源。
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		b, err := fs.ReadFile(staticFS, "static/index.html")
		if err != nil {
			http.Error(w, "前端页面缺失", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(b)
		return
	}
	// 其余静态资源(如 /style.css, /app.js)。
	name := strings.TrimPrefix(r.URL.Path, "/")
	b, err := fs.ReadFile(staticFS, "static/"+name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType(name))
	w.Write(b)
}

// detectReq 是 /api/detect 的请求体。
// 注意: 不接受客户端 ConfigPath，避免任意本地文件读取/二阶 RCE。
type detectReq struct {
	Title       string `json:"title"`
	Author      string `json:"author"`
	SplitMode   int    `json:"splitMode"`
	SplitCount  int    `json:"splitCount"`
	FullReg     string `json:"fullReg"`
	AutoMark    bool   `json:"autoMark"`
	RemoveBlank bool   `json:"removeBlank"`
	AddSpace    bool   `json:"addSpace"`
	AddSpaceCnt int    `json:"addSpaceCount"`
}

// handleAuthCheck 供前端启动时探测令牌是否有效；鉴权逻辑由 withAuth 中间件统一执行。
func (s *Server) handleAuthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{
		"ok":           true,
		"authRequired": s.cfg.Token != "",
	})
}

// detectHandler 内部:解析文本返回章节标题列表与真实探测编码。由 handleDetect 调用。
func detectHandler(req *detectReq, text, encoding string) (map[string]interface{}, error) {
	opt := buildTxtOverrides(req)
	chapters, err := txt.Parse(text, opt)
	if err != nil {
		return nil, fmt.Errorf("解析失败: %w", err)
	}
	titles := make([]string, 0, len(chapters))
	for _, c := range chapters {
		t := strings.TrimSpace(c.Title)
		if t == "" {
			// 空标题不展示（书衣/声明等前置段仍会进 EPUB spine，但不进目录预览）。
			continue
		}
		titles = append(titles, t)
	}
	count := len(titles)
	if opt.SplitMode == 2 && count == 0 {
		// 整本一章模式无分章标题，但正文仍为 1 章。
		count = len(chapters)
	}
	if encoding == "" {
		encoding = "UTF-8"
	}
	return map[string]interface{}{
		"count":    count,
		"titles":   titles,
		"encoding": encoding,
	}, nil
}

// buildTxtOverrides 从请求构造 txt.Options，并做范围校验。
func buildTxtOverrides(req *detectReq) txt.Options {
	splitMode := req.SplitMode
	if splitMode < 0 || splitMode > 2 {
		splitMode = 0
	}
	fullReg := truncateRunes(req.FullReg, 200)
	opt := txt.Options{
		AutoMark:          req.AutoMark,
		RemoveBlankLine:   req.RemoveBlank,
		SplitMode:         splitMode,
		SplitCount:        req.SplitCount,
		FullReg:           fullReg,
		AddSpace:          req.AddSpace,
		AddSpaceCount:     req.AddSpaceCnt,
		ForceEmptyChapter: true,
	}
	if opt.FullReg == "" && opt.SplitMode == 0 {
		opt.AutoMark = true
	}
	if opt.AddSpace && opt.AddSpaceCount <= 0 {
		opt.AddSpaceCount = 1
	}
	return opt
}

// handleDetect 上传 TXT 并返回章节识别预览(不写文件)。
func (s *Server) handleDetect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	req, text, encoding, err := parseUpload(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": safeErr(err)})
		return
	}
	res, err := detectHandler(req, text, encoding)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": safeErr(err)})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// convertReq 是 /api/convert 的排版与输出选项。
type convertReq struct {
	LineHeight   int
	FontSize     int
	MarginTop    int
	MarginTopSet bool
	TextAlign    int
	Indent       float64
	EnableMobi   bool
}

// handleConvert 上传 TXT 并转换,将 EPUB 写入输出目录,返回下载标识。
func (s *Server) handleConvert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// 并发闸：满了直接 429。
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	default:
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "服务器忙，请稍后重试"})
		return
	}

	req, text, encoding, fileName, err := parseUploadWithName(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": safeErr(err)})
		return
	}
	marginRaw := strings.TrimSpace(r.FormValue("marginTop"))
	creq := &convertReq{
		LineHeight:   clampInt(atoi(r.FormValue("lineHeight")), 50, 300, 120),
		FontSize:     clampInt(atoi(r.FormValue("fontSize")), 50, 300, 100),
		MarginTop:    clampInt(atoi(marginRaw), 0, 50, 5),
		MarginTopSet: marginRaw != "",
		TextAlign:    clampInt(atoi(r.FormValue("textAlign")), 0, 3, 0),
		Indent:       clampFloat(atof(r.FormValue("indent")), 0, 4, 0),
		EnableMobi:   parseBool(r.FormValue("enableMobi")),
	}

	base := time.Now().Format("20060102-150405") + "-" + randSuffix()
	outPath := filepath.Join(s.outputs, base+".epub")
	// 仅使用 WorkDir 下的 config.xml（若存在），绝不接受客户端 configPath。
	cfgPath := ""
	if p := filepath.Join(s.cfg.WorkDir, "config.xml"); fileExists(p) {
		cfgPath = p
	}
	txtOpt := buildTxtOverrides(req)
	co := converter.Options{
		Input:        fileName, // 仅用于书名回退，不再重复落盘读取
		Output:       outPath,
		Title:        req.Title,
		Author:       req.Author,
		BaseDir:      s.cfg.WorkDir,
		ConfigPath:   cfgPath,
		EnableMobi:   creq.EnableMobi,
		TXTOverrides: &txtOpt,
		CSSOverrides: &converter.CSSOverrides{
			LineHeight:   creq.LineHeight,
			FontSize:     creq.FontSize,
			MarginTop:    creq.MarginTop,
			MarginTopSet: creq.MarginTopSet,
			TextAlign:    creq.TextAlign,
			Indent:       creq.Indent,
		},
	}

	res, err := converter.ConvertText(r.Context(), co, text, encoding)
	if err != nil {
		var inErr *converter.InputError
		if errors.As(err, &inErr) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": safeErr(inErr)})
			return
		}
		log.Printf("convert error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "转换失败"})
		return
	}
	// 磁盘文件用随机名防冲突；下载展示名保留上传原文件名。
	dlName := filepath.Base(res.EpubPath)
	epubDisplay := displayNameFromUpload(fileName, ".epub")
	mobiName := ""
	mobiDisplay := ""
	mobiDownload := ""
	if res.MobiPath != "" {
		mobiName = filepath.Base(res.MobiPath)
		mobiDisplay = displayNameFromUpload(fileName, ".mobi")
		mobiDownload = "/api/download/" + mobiName + "?name=" + url.QueryEscape(mobiDisplay)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"download":     "/api/download/" + dlName + "?name=" + url.QueryEscape(epubDisplay),
		"epub":         dlName,
		"epubName":     epubDisplay,
		"mobi":         mobiName,
		"mobiName":     mobiDisplay,
		"mobiDownload": mobiDownload,
		"chapters":     res.ChapterCount,
		"encoding":     res.Encoding,
	})
}

// handleDownload 提供已生成文件的下载。
// 路径段必须是服务端生成的随机名；?name= 仅影响 Content-Disposition 展示名，不参与路径解析。
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/download/")
	// 白名单文件名格式 + 防目录穿越。
	if name == "" || !downloadNameRe.MatchString(name) {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}
	full := filepath.Join(s.outputs, name)
	if !safeJoin(s.outputs, full) {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	if !fileExists(full) {
		http.NotFound(w, r)
		return
	}
	ext := strings.ToLower(filepath.Ext(name))
	ct := "application/octet-stream"
	if ext == ".epub" {
		ct = "application/epub+zip"
	} else if ext == ".mobi" {
		ct = "application/x-mobipocket-ebook"
	}
	display := sanitizeDownloadName(r.URL.Query().Get("name"), ext)
	if display == "" {
		display = name
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", contentDispositionAttachment(display))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, full)
}

// displayNameFromUpload 由上传文件名生成下载展示名（主名 + 目标扩展名）。
func displayNameFromUpload(uploadName, wantExt string) string {
	base := filepath.Base(strings.ReplaceAll(uploadName, "\\", "/"))
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	stem = sanitizeFileStem(stem)
	if stem == "" {
		stem = "book"
	}
	wantExt = strings.ToLower(wantExt)
	if wantExt != ".epub" && wantExt != ".mobi" {
		wantExt = ".epub"
	}
	return stem + wantExt
}

// sanitizeFileStem 去掉路径分隔与 Windows 非法字符，保留可读主名。
func sanitizeFileStem(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r < 32 || r == 127:
			continue
		case strings.ContainsRune(`<>:"/\|?*`, r):
			b.WriteByte('_')
		case unicode.IsSpace(r):
			// 保留普通空格，折叠由 Trim 处理两端。
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	out = strings.Trim(out, ". ")
	// 限制长度，避免超长 Content-Disposition。
	const maxRunes = 120
	if rs := []rune(out); len(rs) > maxRunes {
		out = string(rs[:maxRunes])
		out = strings.TrimRight(out, ". ")
	}
	if out == "." || out == ".." {
		return ""
	}
	return out
}

// sanitizeDownloadName 校验客户端传入的展示名：只允许安全字符，扩展名必须匹配产物。
func sanitizeDownloadName(raw, wantExt string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	base := filepath.Base(strings.ReplaceAll(raw, "\\", "/"))
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	stem = sanitizeFileStem(stem)
	if stem == "" {
		return ""
	}
	wantExt = strings.ToLower(wantExt)
	if wantExt != ".epub" && wantExt != ".mobi" {
		return ""
	}
	return stem + wantExt
}

// contentDispositionAttachment 生成兼容中文的 Content-Disposition。
func contentDispositionAttachment(name string) string {
	// ASCII 回退名：非 ASCII 替换为 _，供旧客户端使用。
	var ascii strings.Builder
	for _, r := range name {
		if r < 128 && r >= 32 && r != '"' && r != '\\' {
			ascii.WriteByte(byte(r))
		} else {
			ascii.WriteByte('_')
		}
	}
	fallback := ascii.String()
	if fallback == "" || fallback == "." || fallback == ".." {
		fallback = "download"
	}
	// RFC 5987 filename*
	encoded := url.PathEscape(name)
	// PathEscape 保留部分字符，对 Content-Disposition 再把空格编成 %20。
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, fallback, encoded)
}

// safeErr 返回不泄漏绝对路径/系统细节的错误文案。
func safeErr(err error) string {
	if err == nil {
		return "未知错误"
	}
	msg := truncateRunes(err.Error(), 200)
	if strings.Contains(msg, ":\\") || strings.Contains(msg, "/") {
		// 常见 os 错误含路径，统一脱敏。
		if strings.Contains(msg, "解析上传") {
			return "上传失败或文件过大"
		}
		if strings.Contains(msg, "编码") {
			return "编码识别失败"
		}
		if strings.Contains(msg, "文件") {
			return "文件处理失败"
		}
		return "请求无效"
	}
	return msg
}

// truncateRunes 按 Unicode 字符数截断字符串，避免截断 UTF-8 多字节字符。
func truncateRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	n := 0
	for i := range s {
		if n == max {
			return s[:i]
		}
		n++
	}
	return s
}

func clampInt(v, min, max, def int) int {
	if v == 0 && def != 0 {
		// 表单缺省时 atoi 得 0；对 lineHeight/fontSize 用默认。
		if min > 0 && v < min {
			return def
		}
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func clampFloat(v, min, max, def float64) float64 {
	if v < min {
		if v == 0 && def != 0 {
			return def
		}
		return min
	}
	if v > max {
		return max
	}
	return v
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func contentType(name string) string {
	switch {
	case strings.HasSuffix(name, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	}
	return "application/octet-stream"
}

func safeJoin(base, p string) bool {
	rel, err := filepath.Rel(base, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
