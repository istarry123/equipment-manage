// Package models 定义与数据库表一一对应的 GORM 模型（设计见 docs/database-design.md）。
package models

import "time"

// 设备状态（第一版六态，中文展示由前端映射）。
const (
	StatusInStock     = "IN_STOCK"
	StatusInTeam      = "IN_TEAM"
	StatusBorrowed    = "BORROWED"
	StatusMaintenance = "MAINTENANCE"
	StatusScrapped    = "SCRAPPED"
	StatusOther       = "OTHER"
)

// 流转动作（白名单，见 docs/state-machine.md）。
const (
	ActionImportInit      = "IMPORT_INIT"      // 导入固化（初始状态）
	ActionOutToTeam       = "OUT_TO_TEAM"      // 出库给班组
	ActionReturnFromTeam  = "RETURN_FROM_TEAM" // 班组归还入库
	ActionHandover        = "HANDOVER"         // 班组转交
	ActionBorrow          = "BORROW"           // 外借
	ActionReturnBorrow    = "RETURN_BORROW"    // 外借归还
	ActionToMaintenance   = "TO_MAINTENANCE"   // 送修
	ActionFromMaintenance = "FROM_MAINTENANCE" // 维修完成
	ActionScrap           = "SCRAP"            // 报废
	ActionCorrect         = "CORRECT"          // 受限更正
)

// 外借单状态。
const (
	BorrowOutstanding = "OUTSTANDING" // 外借中
	BorrowReturned    = "RETURNED"    // 已归还
)

// Category 类别字典（决策基线 12：页面可维护）。
type Category struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"uniqueIndex;not null" json:"name"`
	Sort      int       `gorm:"default:0" json:"sort"`
	Remark    string    `json:"remark"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Category) TableName() string { return "category" }

// Equipment 设备主表：保存“当前状态快照”，历史记录在 transaction。
type Equipment struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	EquipmentNo           *string    `gorm:"index:idx_equipment_no" json:"equipment_no"` // 真实编号；无编号设备为 NULL
	InternalCode          string     `gorm:"uniqueIndex;not null" json:"internal_code"`  // 系统内部码 EQ-xxxxx
	Name                  string     `gorm:"not null;index" json:"name"`
	Model                 string     `json:"model"`
	CategoryID            *uint      `gorm:"index" json:"category_id"`
	Status                string     `gorm:"not null;default:IN_STOCK;index" json:"status"`
	CurrentTeamID         *uint      `gorm:"index" json:"current_team_id"`
	CurrentBorrowerID     *uint      `gorm:"index" json:"current_borrower_id"`
	CurrentBorrowRecordID *uint      `gorm:"index" json:"current_borrow_record_id"`
	CurrentSince          *time.Time `json:"current_since"` // 到达当前状态的时间（交付时间口径）
	Remark                string     `json:"remark"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

func (Equipment) TableName() string { return "equipment" }

// Team 班组。停用不清除历史引用（决策 6）。
type Team struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"uniqueIndex;not null" json:"name"`
	Department string    `json:"department"`
	IsActive   bool      `gorm:"default:true" json:"is_active"`
	Remark     string    `json:"remark"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (Team) TableName() string { return "team" }

// Borrower 外借方。
type Borrower struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"uniqueIndex;not null" json:"name"`
	Contact   string    `json:"contact"`
	Phone     string    `json:"phone"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	Remark    string    `json:"remark"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Borrower) TableName() string { return "borrower" }

// BorrowRecord 外借单：一设备一单（决策 1/2）。
type BorrowRecord struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	EquipmentID        uint       `gorm:"not null;index" json:"equipment_id"`
	BorrowerID         uint       `gorm:"not null;index" json:"borrower_id"`
	BorrowDate         time.Time  `gorm:"not null" json:"borrow_date"`
	ExpectedReturnDate *time.Time `json:"expected_return_date"`
	ActualReturnDate   *time.Time `json:"actual_return_date"`
	Status             string     `gorm:"not null;default:OUTSTANDING;index" json:"status"`
	Remark             string     `json:"remark"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func (BorrowRecord) TableName() string { return "borrow_record" }

// Transaction 流转历史：永久保存、禁止删除；保存名称快照（决策 14）。
type Transaction struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	EquipmentID    uint      `gorm:"not null;index:idx_txn_equipment_occurred" json:"equipment_id"`
	Action         string    `gorm:"not null;index" json:"action"`
	FromStatus     string    `gorm:"not null" json:"from_status"`
	ToStatus       string    `gorm:"not null" json:"to_status"`
	FromTeamID     *uint     `json:"from_team_id"`
	ToTeamID       *uint     `json:"to_team_id"`
	FromTeamName   string    `json:"from_team_name"` // 名称快照
	ToTeamName     string    `json:"to_team_name"`   // 名称快照
	BorrowerID     *uint     `json:"borrower_id"`
	BorrowerName   string    `json:"borrower_name"` // 名称快照
	BorrowRecordID *uint     `gorm:"index" json:"borrow_record_id"`
	OccurredAt     time.Time `gorm:"not null" json:"occurred_at"`
	Operator       string    `gorm:"not null" json:"operator"`
	Remark         string    `json:"remark"`
	CreatedAt      time.Time `json:"created_at"`
}

func (Transaction) TableName() string { return "flow_record" } // SQLite 保留关键字 TRANSACTION → 物理表 flow_record

// AuditLog 敏感操作留痕（决策 13：受限更正等）。
type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Action    string    `gorm:"not null;index" json:"action"`
	Target    string    `json:"target"`
	Detail    string    `json:"detail"`
	Reason    string    `json:"reason"`
	Operator  string    `gorm:"not null" json:"operator"`
	CreatedAt time.Time `json:"created_at"`
}

func (AuditLog) TableName() string { return "audit_log" }

// Setting 系统设置键值。
type Setting struct {
	Key   string `gorm:"primaryKey" json:"key"`
	Value string `json:"value"`
}

func (Setting) TableName() string { return "settings" }
