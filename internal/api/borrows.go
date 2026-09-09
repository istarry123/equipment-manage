package api

import (
	"net/http"
	"strings"
	"time"

	"equipment/internal/models"
	"equipment/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ---------- 外借单列表/归还/延期 ----------

// borrowItem 外借单条目（含设备与状态/逾期）。
type borrowItem struct {
	ID                 uint   `json:"id"`
	EquipmentID        uint   `json:"equipment_id"`
	EquipmentNo        string `json:"equipment_no"`
	Name               string `json:"name"`
	Model              string `json:"model"`
	Category           string `json:"category"`
	BorrowerID         uint   `json:"borrower_id"`
	BorrowerName       string `json:"borrower_name"`
	BorrowDate         string `json:"borrow_date"`
	ExpectedReturnDate string `json:"expected_return_date"`
	ActualReturnDate   string `json:"actual_return_date"`
	Status             string `json:"status"`
	OverdueDays        int    `json:"overdue_days"`
}

type borrowListResponse struct {
	Total int64        `json:"total"`
	Items []borrowItem `json:"items"`
}

// ListBorrows GET /api/borrows?status=&q=&offset=&limit=
// status: OUTSTANDING / RETURNED / OVERDUE(派生：过预计归还日未还)
func (s *Server) ListBorrows(c *gin.Context) {
	offset := parsePositiveInt(c.Query("offset"), 0)
	limit := parsePositiveInt(c.Query("limit"), 20)
	if limit > 200 {
		limit = 200
	}
	status := strings.ToUpper(c.Query("status"))
	q := c.Query("q")

	type row struct {
		models.BorrowRecord
		EquipmentNo   string `gorm:"column:equipment_no"`
		EquipmentName string `gorm:"column:equipment_name"`
		Model         string `gorm:"column:model"`
		CategoryName  string `gorm:"column:category_name"`
		BorrowerName  string `gorm:"column:borrower_name"`
	}

	build := func(mode string) *gorm.DB {
		db := s.DB.Model(&models.BorrowRecord{}).
			Joins("JOIN equipment ON equipment.id = borrow_record.equipment_id").
			Joins("JOIN borrower ON borrower.id = borrow_record.borrower_id").
			Joins("LEFT JOIN category ON category.id = equipment.category_id")
		if status == "OVERDUE" {
			db = db.Where("borrow_record.status = ? AND borrow_record.expected_return_date IS NOT NULL AND borrow_record.expected_return_date < ?",
				models.BorrowOutstanding, time.Now().Format(time.RFC3339Nano))
		} else if status != "" && status != "ALL" {
			db = db.Where("borrow_record.status = ?", status)
		}
		if q != "" {
			like := "%" + q + "%"
			db = db.Where("borrow_record.id LIKE ? OR equipment.equipment_no LIKE ? OR equipment.name LIKE ? OR borrower.name LIKE ?",
				like, like, like, like)
		}
		if mode == "count" {
			return db
		}
		return db.Select(`borrow_record.*, COALESCE(equipment.equipment_no,'') AS equipment_no,
			equipment.name AS equipment_name, COALESCE(equipment.model,'') AS model,
			COALESCE(category.name,'') AS category_name, borrower.name AS borrower_name`).
			Order("borrow_record.status ASC, borrow_record.expected_return_date ASC, borrow_record.id DESC")
	}

	var total int64
	if err := build("count").Count(&total).Error; err != nil {
		writeError(c, http.StatusInternalServerError, "查询外借失败")
		return
	}
	var rows []row
	if err := build("list").Offset(offset).Limit(limit).Scan(&rows).Error; err != nil {
		writeError(c, http.StatusInternalServerError, "查询外借失败")
		return
	}
	now := time.Now().Truncate(24 * time.Hour)
	items := make([]borrowItem, 0, len(rows))
	for _, r := range rows {
		overdue := 0
		if r.Status == models.BorrowOutstanding && r.ExpectedReturnDate.Valid {
			exp := r.ExpectedReturnDate.Time.Truncate(24 * time.Hour)
			if exp.Before(now) {
				overdue = int(now.Sub(exp).Hours() / 24)
				if overdue < 1 {
					overdue = 1
				}
			}
		}
		no := r.EquipmentNo
		if no == "" {
			no = "-"
		}
		items = append(items, borrowItem{
			ID: r.ID, EquipmentID: r.EquipmentID, EquipmentNo: no,
			Name: r.EquipmentName, Model: r.Model, Category: r.CategoryName,
			BorrowerID: r.BorrowerID, BorrowerName: r.BorrowerName,
			BorrowDate:         r.BorrowDate.Format("2006-01-02"),
			ExpectedReturnDate: dateFmt(r.ExpectedReturnDate),
			ActualReturnDate:   dateFmt(r.ActualReturnDate),
			Status:             r.Status,
			OverdueDays:        overdue,
		})
	}
	c.JSON(http.StatusOK, borrowListResponse{Total: total, Items: items})
}

func dateFmt(n models.NullTime) string {
	if !n.Valid {
		return ""
	}
	return n.Time.Format("2006-01-02")
}

// ReturnBorrow POST /api/borrows/:id/return（决策 3：归一一律回仓库）
// v1.1 §二十三：可指定实际归还历史日期（默认今天，不得晚于当前）。
func (s *Server) ReturnBorrow(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body struct {
		Operator         string  `json:"operator"`
		Remark           string  `json:"remark"`
		ActualReturnDate *string `json:"actual_return_date"` // 可选历史日期
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	occ, err := optionalTime(body.ActualReturnDate, "实际归还日期格式错误（示例 2026-09-01）")
	if err != nil {
		writeError(c, http.StatusBadRequest, err.Error())
		return
	}
	var rec models.BorrowRecord
	if err := s.DB.First(&rec, id).Error; err != nil {
		writeError(c, http.StatusNotFound, "外借单不存在")
		return
	}
	if rec.Status != models.BorrowOutstanding {
		writeError(c, http.StatusConflict, "该外借单已归还，无需重复操作")
		return
	}
	var eq models.Equipment
	if err := s.DB.First(&eq, rec.EquipmentID).Error; err != nil {
		writeError(c, http.StatusInternalServerError, "设备不存在")
		return
	}
	if eq.Status != models.StatusBorrowed {
		writeError(c, http.StatusConflict, "设备当前状态与外借单不一致，无法归还（请先核对设备状态）")
		return
	}
	if _, err := service.Transition(s.DB, eq.ID, service.FlowRequest{
		Action: models.ActionReturnBorrow, Operator: body.Operator,
		OccurredAt: occ, Remark: body.Remark,
	}); err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": rec.ID, "returned": true})
}

// ExtendBorrow POST /api/borrows/:id/extend 修改预计归还日期（延期）。
func (s *Server) ExtendBorrow(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body struct {
		ExpectedReturnDate string `json:"expected_return_date"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	t, err := parseTime(body.ExpectedReturnDate)
	if err != nil {
		writeError(c, http.StatusBadRequest, "预计归还日期格式错误（示例 2026-10-01）")
		return
	}
	var rec models.BorrowRecord
	if err := s.DB.First(&rec, id).Error; err != nil {
		writeError(c, http.StatusNotFound, "外借单不存在")
		return
	}
	if rec.Status != models.BorrowOutstanding {
		writeError(c, http.StatusConflict, "该外借单已归还，不可延期")
		return
	}
	rec.ExpectedReturnDate = models.ValidTime(t)
	rec.UpdatedAt = models.Now()
	if err := s.DB.Save(&rec).Error; err != nil {
		writeError(c, http.StatusInternalServerError, "保存失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": rec.ID, "expected_return_date": t.Format("2006-01-02")})
}

// ---------- 外借方管理 ----------

type borrowerItem struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Contact  string `json:"contact"`
	Phone    string `json:"phone"`
	IsActive bool   `json:"is_active"`
}

// ListBorrowers GET /api/borrowers
func (s *Server) ListBorrowers(c *gin.Context) {
	var rows []models.Borrower
	if err := s.DB.Order("is_active DESC, name ASC").Find(&rows).Error; err != nil {
		writeError(c, http.StatusInternalServerError, "查询外借方失败")
		return
	}
	items := make([]borrowerItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, borrowerItem{ID: r.ID, Name: r.Name, Contact: r.Contact, Phone: r.Phone, IsActive: r.IsActive})
	}
	c.JSON(http.StatusOK, items)
}

// CreateBorrower POST /api/borrowers
func (s *Server) CreateBorrower(c *gin.Context) {
	var body struct {
		Name    string `json:"name"`
		Contact string `json:"contact"`
		Phone   string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	b, err := service.CreateBorrower(s.DB, body.Name, body.Contact, body.Phone)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": b.ID, "name": b.Name})
}

// UpdateBorrower PUT /api/borrowers/:id
func (s *Server) UpdateBorrower(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body struct {
		Name     string  `json:"name"`
		Contact  *string `json:"contact"`
		Phone    *string `json:"phone"`
		IsActive *bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	_, err := service.UpdateBorrower(s.DB, id, service.UpdateBorrowerInput{
		Name: body.Name, Contact: body.Contact, Phone: body.Phone, IsActive: body.IsActive,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id})
}
