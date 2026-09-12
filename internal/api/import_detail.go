package api

import (
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

// 外借明细导入通道（v1.3 Phase 3）：上传 → 解析 → 与现有台账匹配 → 预览。
// 本阶段只读：不写库、不改状态（写入在 Phase 4）。

const (
	detailPreviewItemMax  = 200 // 明细预览行上限（避免响应过大；完整数据写入后见台账）
	detailPreviewBlockMax = 80
)

// detailSession 一次外借明细上传的会话（文件 + 解析 + 匹配结果，供 Phase 4 写入复用）。
type detailSession struct {
	path      string
	parse     *importer.DetailParseResult
	match     *importer.DetailMatchResult
	createdAt time.Time
}

type detailStore struct {
	mu   sync.Mutex
	data map[string]*detailSession
}

func newDetailStore() *detailStore {
	return &detailStore{data: map[string]*detailSession{}}
}

func (s *detailStore) put(id string, sess *detailSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.data) >= 10 { // 单机场景：最多保留 10 个会话，超出丢最旧
		var oldestKey string
		var oldest time.Time
		for k, v := range s.data {
			if oldestKey == "" || v.createdAt.Before(oldest) {
				oldestKey, oldest = k, v.createdAt
			}
		}
		if oldestKey != "" {
			if v, ok := s.data[oldestKey]; ok {
				os.Remove(v.path) //nolint:errcheck
			}
			delete(s.data, oldestKey)
		}
	}
	s.data[id] = sess
}

func (s *detailStore) get(id string) (*detailSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[id]
	return v, ok
}

type importDetailHandler struct {
	server *Server
	store  *detailStore
}

// detailSummary 外借明细预览摘要。
type detailSummary struct {
	Filename          string   `json:"filename"`
	Sheet             string   `json:"sheet"`
	Blocks            int      `json:"blocks"`
	BlockedBlocks     int      `json:"blocked_blocks"` // 结构不自洽、不会写入的块
	TotalCount        int      `json:"total_count"`    // C 列合计（口径②）
	Numbered          int      `json:"numbered"`
	SkippedUnnumbered int      `json:"skipped_unnumbered"` // 口径③
	TotalRowRaw       string   `json:"total_row_raw,omitempty"`
	Devices           int      `json:"devices"`
	Unique            int      `json:"unique"`
	ByLabel           int      `json:"by_label"`
	Ambiguous         int      `json:"ambiguous"`
	Missing           int      `json:"missing"`
	Borrowers         int      `json:"borrowers"`
	BorrowersNew      int      `json:"borrowers_new"`
	BlockIssues       int      `json:"block_issues"`
	Warnings          int      `json:"warnings"`
	Reviews           int      `json:"reviews"`
	MinBorrowDate     string   `json:"min_borrow_date"`
	MaxBorrowDate     string   `json:"max_borrow_date"`
	Companies         []string `json:"companies"`
	DupNos            []string `json:"dup_nos"`
	SameNoMultiNo     []string `json:"same_no_multi_no"`
}

// ParseDetail POST /api/import/borrow-detail/parse（multipart 字段 file）。
func (h *importDetailHandler) ParseDetail(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)
	file, err := c.FormFile("file")
	if err != nil {
		writeError(c, http.StatusBadRequest, "请选择外借明细 .xlsx 文件上传（字段名 file）")
		return
	}
	if !strings.EqualFold(filepath.Ext(file.Filename), ".xlsx") {
		writeError(c, http.StatusBadRequest, "仅支持 .xlsx 格式")
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
	dest := filepath.Join(uploadsDir, "detail-"+id+".xlsx")
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

	parse, err := importer.ParseBorrowDetail(dest)
	if err != nil {
		os.Remove(dest)
		writeError(c, http.StatusBadRequest, err.Error())
		return
	}
	parse.Filename = filepath.Base(file.Filename)
	match, err := importer.MatchBorrowDetail(h.server.DB, parse)
	if err != nil {
		os.Remove(dest)
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.store.put(id, &detailSession{path: dest, parse: parse, match: match, createdAt: time.Now()})

	c.JSON(http.StatusOK, gin.H{
		"parse_id": id,
		"summary":  buildDetailSummary(parse, match),
		"preview":  buildDetailPreview(parse, match),
	})
}

func buildDetailSummary(p *importer.DetailParseResult, m *importer.DetailMatchResult) detailSummary {
	s := detailSummary{
		Filename: p.Filename, Sheet: p.Sheet,
		Blocks: len(p.Blocks), TotalCount: p.TotalCount, Numbered: p.NumberedCount,
		SkippedUnnumbered: p.SkippedUnnum, TotalRowRaw: p.TotalRowRaw,
		Devices: m.Total, Unique: m.Unique, ByLabel: m.ByLabel,
		Ambiguous: m.Ambiguous, Missing: m.Missing,
		Borrowers: len(m.Borrowers),
		Companies: p.Companies, DupNos: m.DupNos, SameNoMultiNo: m.SameNoMultiNo,
		MinBorrowDate: m.MinBorrowDate, MaxBorrowDate: m.MaxBorrowDate,
	}
	for _, b := range p.Blocks {
		if !b.OK {
			s.BlockedBlocks++
		}
	}
	bump := func(level string) {
		switch level {
		case "BLOCK":
			s.BlockIssues++
		case importer.IssueLevelReview:
			s.Reviews++
		default:
			s.Warnings++
		}
	}
	for _, is := range p.Issues {
		bump(is.Level)
	}
	for _, is := range m.Issues {
		bump(is.Level)
	}
	for _, b := range m.Borrowers {
		if !b.Exists {
			s.BorrowersNew++
		}
	}
	return s
}

func buildDetailPreview(p *importer.DetailParseResult, m *importer.DetailMatchResult) gin.H {
	// 块级预览
	bs := make([]gin.H, 0, detailPreviewBlockMax)
	for i, b := range p.Blocks {
		if i >= detailPreviewBlockMax {
			break
		}
		date := ""
		if b.ArriveAt != nil {
			date = b.ArriveAt.Format("2006-01-02")
		}
		bs = append(bs, gin.H{
			"rows": b.BlockRows, "borrow_date": date, "borrow_raw": b.ArriveRaw,
			"company": b.Company, "count": b.Count, "numbered": len(b.Numbers),
			"unnumbered": b.Unnumbered, "ok": b.OK, "remark": b.Remark,
			"raw_lines": b.RawLines,
		})
	}
	// 明细行预览（分桶）
	items := make([]gin.H, 0, detailPreviewItemMax)
	for i, it := range m.Items {
		if i >= detailPreviewItemMax {
			break
		}
		items = append(items, detailItemJSON(it))
	}
	amb := []gin.H{}
	miss := []gin.H{}
	for _, it := range m.Items {
		switch it.Status {
		case importer.MatchAmbiguous:
			amb = append(amb, detailItemJSON(it))
		case importer.MatchMissing:
			miss = append(miss, detailItemJSON(it))
		}
	}
	// 问题清单（解析 + 匹配）
	issues := make([]gin.H, 0, previewIssuesMax)
	addIssue := func(row int, code, level, msg string) {
		if len(issues) >= previewIssuesMax {
			return
		}
		issues = append(issues, gin.H{"row": row, "code": code, "level": level, "message": msg})
	}
	for _, is := range p.Issues {
		addIssue(is.Row, is.Code, is.Level, is.Message)
	}
	for _, is := range m.Issues {
		addIssue(is.Row, is.Code, is.Level, is.Message)
	}
	borrowers := make([]gin.H, 0, len(m.Borrowers))
	for _, b := range m.Borrowers {
		borrowers = append(borrowers, gin.H{
			"name": b.Name, "exists": b.Exists, "borrower_id": b.BorrowerID,
			"devices": b.Devices, "blocks": b.Blocks,
		})
	}
	return gin.H{
		"blocks": bs, "items": items, "item_total": len(m.Items),
		"ambiguous": amb, "missing": miss, "borrowers": borrowers, "issues": issues,
	}
}

func detailItemJSON(it *importer.DetailMatchItem) gin.H {
	h := gin.H{
		"source_key": it.SourceKey, "row": it.Row, "block_rows": it.BlockRows,
		"equipment_no": it.EquipmentNo, "company": it.Company,
		"borrow_date": it.BorrowDate, "borrow_raw": it.BorrowRaw,
		"occurrence": it.Occurrence, "occurrences": it.Occurrences,
		"status": it.Status, "note": it.Note, "evidence": it.Evidence,
		"suggested_equipment_id": it.SuggestedID,
	}
	if it.Chosen != nil {
		h["chosen"] = candidateJSON(it.Chosen)
	}
	if len(it.Candidates) > 0 {
		cs := make([]gin.H, 0, len(it.Candidates))
		for _, c := range it.Candidates {
			cs = append(cs, candidateJSON(c))
		}
		h["candidates"] = cs
	}
	return h
}

func candidateJSON(c *importer.MatchCandidate) gin.H {
	return gin.H{
		"equipment_id": c.EquipmentID, "internal_code": c.InternalCode,
		"equipment_no": c.EquipmentNo, "name": c.Name, "model": c.Model,
		"category": c.Category, "status": c.Status, "equipment_seq": c.Seq,
		"label": c.Label, "display_no": c.DisplayNo, "is_current": c.IsCurrent,
	}
}
