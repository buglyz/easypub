// Package epub 生成 EPUB 2.0 包。
//
// 输出与 EasyPub v1.50 生成的结构一致：
//
//	mimetype
//	META-INF/container.xml
//	OEBPS/content.opf
//	OEBPS/toc.ncx
//	OEBPS/style.css
//	OEBPS/cover.html
//	OEBPS/book-toc.html
//	OEBPS/chapter0.html ... chapterN.html
package epub

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/easypub/go-easypub/internal/txt"
)

// Book 是待生成的 EPUB 元数据与章节。
type Book struct {
	Title    string
	Author   string
	Language string
	Date     string // 如 "2025"；为空则取当前年份。
	UID      string // 如 "easypub-6687cffa"；为空则随机生成。
	CSS      string // style.css 内容。
	Chapters []txt.Chapter
}

// Generator 负责流式组装并打包成 .epub。
type Generator struct {
	book *Book
}

const (
	mimetypeContent = "application/epub+zip"
	containerXML    = `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`
)

var (
	utf8BOM   = []byte{0xEF, 0xBB, 0xBF}
	crlfBytes = []byte("\r\n")
	// fixedZipTime 固定时间戳，避免每次生成因 Modified 不同导致字节级 diff。
	fixedZipTime = time.Unix(0, 0).UTC()
)

// New 创建生成器。
func New(b *Book) *Generator {
	if b.Language == "" {
		b.Language = "zh-CN"
	}
	if b.Date == "" {
		b.Date = fmt.Sprintf("%d", time.Now().Year())
	}
	if b.UID == "" {
		b.UID = "easypub-" + randomID(8)
	}
	return &Generator{book: b}
}

// Build 生成 .epub 到 outPath。
// 使用同目录临时文件 + 原子重命名，避免中断留下损坏产物；
// 并逐条流式写入 zip，避免把全书章节副本二次缓存在内存中。
func (g *Generator) Build(outPath string) error {
	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".easypub-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	success := false
	defer func() {
		if !success {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if err := g.WriteEPUB(tmp); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, outPath); err != nil {
		return err
	}
	success = true
	return nil
}

// WriteEPUB 将完整 EPUB ZIP 写入 dst。
// 条目顺序、压缩方式、CRLF 与 UTF-8 BOM 规则与 EasyPub v1.50 样例字节级一致：
//   - mimetype：Store（不压缩）、无 BOM、无 CRLF
//   - META-INF/container.xml / OEBPS/style.css：Deflate、CRLF、无 BOM
//   - 其余 OEBPS/*.html|*.opf|*.ncx：Deflate、CRLF、有 BOM
func (g *Generator) WriteEPUB(dst io.Writer) error {
	w := zip.NewWriter(dst)

	if err := writeRawEntry(w, "mimetype", zip.Store, []byte(mimetypeContent)); err != nil {
		return err
	}
	if err := writeTextEntry(w, "META-INF/container.xml", containerXML, false); err != nil {
		return err
	}
	if err := writeTextEntry(w, "OEBPS/style.css", g.book.CSS, false); err != nil {
		return err
	}
	if err := writeTextEntry(w, "OEBPS/cover.html", coverHTML(g.book), true); err != nil {
		return err
	}
	if err := writeTextEntry(w, "OEBPS/book-toc.html", bookTOCHTML(g.book), true); err != nil {
		return err
	}
	for i := range g.book.Chapters {
		name := fmt.Sprintf("OEBPS/chapter%d.html", i)
		if err := writeTextEntry(w, name, chapterHTML(g.book, i), true); err != nil {
			return err
		}
	}
	if err := writeTextEntry(w, "OEBPS/content.opf", opfXML(g.book), true); err != nil {
		return err
	}
	if err := writeTextEntry(w, "OEBPS/toc.ncx", ncxXML(g.book), true); err != nil {
		return err
	}
	return w.Close()
}

func writeRawEntry(w *zip.Writer, name string, method uint16, content []byte) error {
	fw, err := w.CreateHeader(&zip.FileHeader{
		Name:     name,
		Method:   method,
		Modified: fixedZipTime,
	})
	if err != nil {
		return err
	}
	_, err = fw.Write(content)
	return err
}

// writeTextEntry 流式写入带 CRLF 行尾（可选 UTF-8 BOM）的文本条目，避免整章内存拷贝。
func writeTextEntry(w *zip.Writer, name, text string, withBOM bool) error {
	fw, err := w.CreateHeader(&zip.FileHeader{
		Name:     name,
		Method:   zip.Deflate,
		Modified: fixedZipTime,
	})
	if err != nil {
		return err
	}
	if withBOM && !strings.HasPrefix(text, "\uFEFF") {
		if _, err := fw.Write(utf8BOM); err != nil {
			return err
		}
	}
	return writeCRLF(fw, text)
}

// writeCRLF 将 text 中的 \r\n 或 \n 统一按 \r\n 写入 w，无额外大块内存分配。
func writeCRLF(w io.Writer, text string) error {
	for len(text) > 0 {
		idx := strings.IndexByte(text, '\n')
		if idx < 0 {
			_, err := io.WriteString(w, text)
			return err
		}
		seg := text[:idx]
		if len(seg) > 0 && seg[len(seg)-1] == '\r' {
			seg = seg[:len(seg)-1]
		}
		if len(seg) > 0 {
			if _, err := io.WriteString(w, seg); err != nil {
				return err
			}
		}
		if _, err := w.Write(crlfBytes); err != nil {
			return err
		}
		text = text[idx+1:]
	}
	return nil
}

// randomID 生成 n 字节的十六进制 ID，如 "6687cffa"。
func randomID(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "00000000"
	}
	return hex.EncodeToString(b)
}
