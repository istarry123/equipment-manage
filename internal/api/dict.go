package api

import (
	"errors"
	"net/http"
	"strings"

	"equipment/internal/models"
	"equipment/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// categoryItem 类别字典项。
type categoryItem struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Sort int    `json:"sort"`
}

// ListCategories GET /api/categories
func (s *Server) ListCategories(c *gin.Context) {
	var rows []models.Category
	if err := s.DB.Order("sort ASC, id ASC").Find(&rows).Error; err != nil {
		writeError(c, http.StatusInternalServerError, "查询类别失败")
		return
	}
	items := make([]categoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, categoryItem{ID: r.ID, Name: r.Name, Sort: r.Sort})
	}
	c.JSON(http.StatusOK, items)
}

// CreateCategory POST /api/categories 新增类别（重名返回 409）。
func (s *Server) CreateCategory(c *gin.Context) {
	var body struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeError(c, http.StatusBadRequest, "类别名称不能为空")
		return
	}
	var existing models.Category
	err := s.DB.Where("name = ?", name).First(&existing).Error
	if err == nil {
		writeError(c, http.StatusConflict, "该类别已存在")
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		writeError(c, http.StatusInternalServerError, "查询类别失败")
		return
	}
	now := models.Now()
	cat := models.Category{Name: name, Sort: 0, CreatedAt: now, UpdatedAt: now}
	if err := s.DB.Create(&cat).Error; err != nil {
		writeError(c, http.StatusInternalServerError, "新增类别失败")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": cat.ID, "name": cat.Name})
}

// teamItem 班组/内部单位字典项。
type teamItem struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

// ListTeams GET /api/teams 班组/内部单位列表（含停用标记）。
func (s *Server) ListTeams(c *gin.Context) {
	var rows []models.Team
	if err := s.DB.Order("is_active DESC, name ASC").Find(&rows).Error; err != nil {
		writeError(c, http.StatusInternalServerError, "查询班组失败")
		return
	}
	items := make([]teamItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, teamItem{ID: r.ID, Name: r.Name, IsActive: r.IsActive})
	}
	c.JSON(http.StatusOK, items)
}

// CreateTeam POST /api/teams 新增班组/内部单位。
func (s *Server) CreateTeam(c *gin.Context) {
	var body struct {
		Name       string `json:"name"`
		Department string `json:"department"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	t, err := service.CreateTeam(s.DB, body.Name, body.Department)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": t.ID, "name": t.Name})
}

// UpdateTeam PUT /api/teams/:id（改名/部门/停用）。
func (s *Server) UpdateTeam(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body struct {
		Name       string  `json:"name"`
		Department *string `json:"department"`
		IsActive   *bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	t, err := service.UpdateTeam(s.DB, id, service.UpdateTeamInput{
		Name: body.Name, Department: body.Department, IsActive: body.IsActive,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": t.ID, "name": t.Name, "is_active": t.IsActive})
}
