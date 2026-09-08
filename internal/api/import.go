package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"equipment/internal/importer"

	"github.com/gin-gonic/gin"
)

const (
	uploadsDir       = "uploads"
	maxUploadSize    = 60 << 20 // 60MB
	previewGroupMax  = 30
	previewIssuesMax = 120
)

// importSession 一次上传解析的会话（文件 + 解析结果，供确认导入复用）。
type importSession struct {
	path      string
	res       *importer.ParseResult
	createdAt time.Time
}

type parseStore struct {
	mu   sync.Mutex
	data map[string]*importSession
}

func newParseStore() *parseStore {
	return &parseStore{data: map[string]*importSession{}}
}

func (s *parseStore) put(id string, sess *importSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 简单清理：超过 10 个旧会话即丢弃最旧的（单机场景足够）
	if len(s.data) >= 10 {
		var oldestKey string
		var oldest time.Time
		for k, v := range s.data {
			if oldestKey == "" || v.createdAt.Before(oldest) {
				oldestKey, oldest = k, v.createdAt
			}
		}
		if oldestKey != "" {
			s.removeLocked(oldestKey)
		}
	}
	s.data[id] = sess
}

func (s *parseStore) get(id string) (*importSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[id]
	return v, ok
}

func (s *parseStore) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeLocked(id)
}

func (s *parseStore) removeLocked(id string) {
	if v, ok := s.data[id]; ok {
		os.Remove(v.path) //nolint:errcheck
		delete(s.data, id)
	}
}

type importHandler struct {
	server *Server
	store  *parseStore
}

// summary 解析预览摘要（供前端第一步展示）。
type importSummary struct {
	Filename    string `json:"filename"`
	Sheet       string `json:"sheet"`
	TotalD      int    `json:"total_d"`      // 源台账数量
	Groups      int    `json:"groups"`       // 分组数
	OKGroups    int    `json:"ok_groups"`    // 可导入分组
	BlockGroups int    `json:"block_groups"` // BLOCK 分组
	OKDevices   int    `json:"ok_devices"`   // 可导入台数
	BlockDevs   int    `json:"block_devs"`   // 被阻塞台数
	Blocks      int    `json:"blocks"`       // BLOCK 级问题条数
	Warnings    int    `json:"warnings"`     // WARN 级问题条数
	Duplicates  int    `json:"duplicates"`   // 重复编号多录次数
	Unnumbered  int    `json:"unnumbered"`   // 可导入中的无编号台数
}

// Parse POST /api/import/parse（multipart 字段 file）：上传 → 解析 → 预览。
func (h *importHandler) Parse(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)
	file, err := c.FormFile("file")
	if err != nil {
		writeError(c, http.StatusBadRequest, "请选择 .xlsx 文件上传（字段名 file）")
		return
	}
	if !strings.EqualFold(filepath.Ext(file.Filename), ".xlsx") {
		writeError(c, http.StatusBadRequest, "仅支持 .xlsx 格式（请勿上传其他文件）")
		return
	}
	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		writeError(c, http.StatusInternalServerError, "无法创建上传目录")
		return
	}
	id, err := randomID()
	if err != nil {
		writeError(c, http.StatusInternalServerError, "无法生成会话 ID")
		return
	}
	dest := filepath.Join(uploadsDir, id+".xlsx")
	src, err := file.Open()
	if err != nil {
		writeError(c, http.StatusBadRequest, "读取上传文件失败")
		return
	}
	dst, err := os.Create(dest)
	if err != nil {
		src.Close()
		writeError(c, http.StatusInternalServerError, "保存上传文件失败")
		return
	}
	if _, err := io.Copy(dst, src); err != nil {
		src.Close()
		dst.Close()
		os.Remove(dest)
		writeError(c, http.StatusInternalServerError, "保存上传文件失败")
		return
	}
	src.Close()
	dst.Close()

	res, err := importer.Parse(dest)
	if err != nil {
		os.Remove(dest)
		writeError(c, http.StatusBadRequest, err.Error())
		return
	}
	res.Filename = filepath.Base(file.Filename)
	h.store.put(id, &importSession{path: dest, res: res, createdAt: time.Now()})

	c.JSON(http.StatusOK, gin.H{
		"parse_id": id,
		"summary":  buildSummary(res),
		"preview":  buildPreview(res),
	})
}

func buildSummary(res *importer.ParseResult) importSummary {
	s := importSummary{Filename: res.Filename, Sheet: res.Sheet, TotalD: res.TotalD, Groups: len(res.Groups)}
	dup := 0
	for _, g := range res.Groups {
		if g.OK {
			s.OKGroups++
			s.OKDevices += importer.DeviceCount(g)
			s.Unnumbered += g.Unnumbered
		} else {
			s.BlockGroups++
			s.BlockDevs += importer.DeviceCount(g)
		}
		for n, c := range dupMap(g.Numbers) {
			_ = n
			dup += c
		}
	}
	s.Duplicates = dup
	for _, is := range res.Issues {
		if is.Level == "BLOCK" {
			s.Blocks++
		} else {
			s.Warnings++
		}
	}
	return s
}

func dupMap(nums []string) map[string]int {
	seen := map[string]int{}
	out := map[string]int{}
	for _, n := range nums {
		seen[n]++
	}
	for n, c := range seen {
		if c > 1 {
			out[n] = c - 1
		}
	}
	return out
}

func buildPreview(res *importer.ParseResult) gin.H {
	gs := make([]gin.H, 0, previewGroupMax)
	for i, g := range res.Groups {
		if i >= previewGroupMax {
			break
		}
		gs = append(gs, gin.H{
			"name":       g.Name,
			"model":      g.Model,
			"category":   g.Category,
			"rows":       fmt.Sprintf("R%d-R%d", g.RowFrom, g.RowTo),
			"d":          g.DSum,
			"numbered":   len(g.Numbers),
			"unnumbered": g.Unnumbered,
			"ok":         g.OK,
			"sample_nos": joinLimit(g.Numbers, 6),
		})
	}
	is := make([]gin.H, 0, previewIssuesMax)
	for i, v := range res.Issues {
		if i >= previewIssuesMax {
			break
		}
		is = append(is, gin.H{
			"row": v.Row, "code": v.Code, "level": v.Level, "group": v.Group, "message": v.Message,
		})
	}
	return gin.H{"groups": gs, "issues": is}
}

func joinLimit(s []string, n int) string {
	if len(s) > n {
		return strings.Join(s[:n], " ") + " …"
	}
	return strings.Join(s, " ")
}

// Run POST /api/import/run：确认并执行导入（单事务）。
func (h *importHandler) Run(c *gin.Context) {
	var body struct {
		ParseID string `json:"parse_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.ParseID == "" {
		writeError(c, http.StatusBadRequest, "缺少 parse_id")
		return
	}
	sess, ok := h.store.get(body.ParseID)
	if !ok {
		writeError(c, http.StatusNotFound, "解析会话不存在或已过期，请重新上传")
		return
	}
	report, err := importer.Import(h.server.DB, sess.res)
	if err != nil {
		if errors.Is(err, importer.ErrDBNotEmpty) {
			writeError(c, http.StatusConflict, err.Error())
			return
		}
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	// 导入成功后清理会话与临时文件
	h.store.remove(body.ParseID)
	c.JSON(http.StatusOK, gin.H{"report": report})
}

func randomID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
