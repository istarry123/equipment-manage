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
	"equipment/internal/models"
	"equipment/internal/service"

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
	Filename       string `json:"filename"`
	Sheet          string `json:"sheet"`
	TotalD         int    `json:"total_d"`         // 源台账数量
	Groups         int    `json:"groups"`          // 分组数
	OKGroups       int    `json:"ok_groups"`       // 可导入分组
	BlockGroups    int    `json:"block_groups"`    // BLOCK 分组
	OKDevices      int    `json:"ok_devices"`      // 可导入台数
	BlockDevs      int    `json:"block_devs"`      // 被阻塞台数
	Blocks         int    `json:"blocks"`          // BLOCK 级问题条数
	Warnings       int    `json:"warnings"`        // WARN 级问题条数
	Reviews        int    `json:"reviews"`         // REVIEW 级问题条数（需人工确认）
	Duplicates     int    `json:"duplicates"`      // 重复编号多录次数（同号多台真机，展开导入）
	Unnumbered     int    `json:"unnumbered"`      // 可导入中的无编号台数
	BorrowEvents   int    `json:"borrow_events"`   // J 列历史借出事件总数（§三十一）
	InternalEvents int    `json:"internal_events"` // 内部单位（双发系）调拨历史事件数
	Suspected      int    `json:"suspected"`       // 疑似在借候选（外部公司，可勾选）
	ReviewItems    int    `json:"review_items"`    // REVIEW 需人工确认条目数（§三十三）
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
		switch is.Level {
		case "BLOCK":
			s.Blocks++
		case importer.IssueLevelReview:
			s.Reviews++
		default:
			s.Warnings++
		}
	}
	// Preview/Review 汇总（Phase 9）
	_, suspected, reviews, borrowEv, internalEv := importer.BuildReviewView(res)
	s.Suspected = len(suspected)
	s.ReviewItems = len(reviews)
	s.BorrowEvents = borrowEv
	s.InternalEvents = internalEv
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
	// Phase 9：设备级预览 + 疑似在借候选 + REVIEW 项
	devices, suspected, reviews, _, _ := importer.BuildReviewView(res)
	const previewDeviceMax = 100
	dvs := make([]gin.H, 0, minInt(len(devices), previewDeviceMax))
	for i, d := range devices {
		if i >= previewDeviceMax {
			break
		}
		no := ""
		if d.No != nil {
			no = *d.No
		}
		dvs = append(dvs, gin.H{
			"source_key": d.SourceKey, "display_no": d.DisplayNo, "equipment_no": no,
			"name": d.Name, "model": d.Model, "category": d.Category,
			"unnumbered": d.Unnumbered, "ok": d.OK, "group_rows": d.GroupRows,
		})
	}
	revs := make([]gin.H, 0, len(reviews))
	for _, r := range reviews {
		revs = append(revs, gin.H{
			"key": r.Key, "level": r.Level, "group": r.Group, "row": r.Row,
			"message": r.Message, "raw": r.Raw, "suggestion": r.Suggestion,
		})
	}
	sus := make([]gin.H, 0, len(suspected))
	for _, s := range suspected {
		sus = append(sus, gin.H{
			"source_key": s.SourceKey, "display_no": s.DisplayNo, "name": s.Name, "model": s.Model,
			"company": s.Company, "borrow_date": s.BorrowDate, "row": s.Row, "remark": s.Remark,
		})
	}
	return gin.H{
		"groups": gs, "issues": is,
		"devices": dvs, "device_total": len(devices),
		"suspected": sus, "reviews": revs,
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func joinLimit(s []string, n int) string {
	if len(s) > n {
		return strings.Join(s[:n], " ") + " …"
	}
	return strings.Join(s, " ")
}

// Run POST /api/import/run：确认并执行导入（单事务）。
// body: { parse_id, confirmed_suspects: [source_key...], acknowledged_reviews: [review_key...] }
// §三十三：REVIEW 项未全部确认 → 拒绝导入（返回 422 及未确认清单）。
func (h *importHandler) Run(c *gin.Context) {
	var body struct {
		ParseID            string   `json:"parse_id"`
		ConfirmedSuspects  []string `json:"confirmed_suspects"`
		AcknowledgedReview []string `json:"acknowledged_reviews"`
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
	// REVIEW 门禁（§三十三）
	_, _, reviews, _, _ := importer.BuildReviewView(sess.res)
	if len(reviews) > 0 {
		ack := map[string]bool{}
		for _, k := range body.AcknowledgedReview {
			ack[k] = true
		}
		var pending []string
		for _, r := range reviews {
			if !ack[r.Key] {
				pending = append(pending, r.Key)
			}
		}
		if len(pending) > 0 {
			writeError(c, http.StatusUnprocessableEntity,
				fmt.Sprintf("仍有 %d 项需人工确认（REVIEW）未处理，请逐项确认后再导入", len(pending)))
			return
		}
	}
	report, err := importer.ImportWithOptions(h.server.DB, sess.res, importer.ImportOptions{
		ConfirmedSuspects: body.ConfirmedSuspects,
	})
	if err != nil {
		if errors.Is(err, importer.ErrDBNotEmpty) || errors.Is(err, importer.ErrBatchImported) {
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

// Reset POST /api/import/reset：清空重导前置（危险操作，决策18⑤/铁律5）。
// body: { confirm: true } —— 必须先二次确认；执行顺序：自动备份 → 单事务清空业务数据
// （equipment/flow_record/borrow_record/import_batch，保留字典与 audit 历史）→ audit 留痕。
func (h *importHandler) Reset(c *gin.Context) {
	var body struct {
		Confirm bool `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if !body.Confirm {
		writeError(c, http.StatusBadRequest, "清空是危险操作：请先备份并确认（confirm: true）")
		return
	}
	// 提示当前规模（防误清）
	if n, err := service.CountEquipment(h.server.DB); err != nil {
		writeError(c, http.StatusInternalServerError, "统计设备数失败")
		return
	} else if n == 0 {
		// 空库无需清空；直接返回（幂等安全）
		c.JSON(http.StatusOK, gin.H{"reset": true, "message": "数据库为空，无需清空", "equipment_deleted": 0})
		return
	}
	dbFile, keep := runtimeDBInfo()
	// 1) 自动备份
	backupName, err := service.BackupNow(h.server.DB, dbFile, keep)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "清空前自动备份失败，已中止: "+err.Error())
		return
	}
	// 2) 单事务清空业务数据
	res, err := service.ClearImportData(h.server.DB)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	// 3) audit 留痕
	h.server.DB.Create(&models.AuditLog{ //nolint:errcheck
		Action: "IMPORT_RESET", Target: backupName,
		Detail: fmt.Sprintf("清空重导前置：清空业务数据（设备 %d / 流转 %d / 外借单 %d / 批次 %d），自动备份 %s",
			res.EquipmentDeleted, res.FlowDeleted, res.BorrowDeleted, res.BatchDeleted, backupName),
		Operator: "系统导入", CreatedAt: models.Now(),
	})
	c.JSON(http.StatusOK, gin.H{
		"reset": true, "backup_name": backupName,
		"equipment_deleted": res.EquipmentDeleted, "flow_deleted": res.FlowDeleted,
		"borrow_deleted": res.BorrowDeleted, "batch_deleted": res.BatchDeleted,
		"message": "业务数据已清空（已自动备份为 " + backupName + "）。可重新上传 Excel 执行首次全量导入。",
	})
}

func randomID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
