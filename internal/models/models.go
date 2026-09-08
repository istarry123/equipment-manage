// Package models 定义与数据库表一一对应的 GORM 模型（设计见 docs/database-design.md）。
package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// ---------- 时间类型（SQLite TEXT 列统一存取） ----------

// timeLayouts 兼容入库与读取的时间格式（含统一存储格式）。
var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
}

// storeLayout 统一入库格式（RFC3339Nano，含时区，可排序）。
const storeLayout = time.RFC3339Nano

// Time 非空时间列；实现 driver.Valuer/sql.Scanner/JSON 以适配 TEXT 列。
type Time struct {
	time.Time
}

// FromTime 由 time.Time 构造 Time。
func FromTime(t time.Time) Time { return Time{Time: t} }

// Now 返回当前本地时间。
func Now() Time { return Time{Time: time.Now()} }

// Value 实现 driver.Valuer（统一以 storeLayout 写入 TEXT 列）。
func (t Time) Value() (driver.Value, error) { return t.Format(storeLayout), nil }

// Scan 实现 sql.Scanner（读取 TEXT 列回填）。
func (t *Time) Scan(v any) error {
	parsed, err := parseTimeValue(v)
	if err != nil {
		return err
	}
	t.Time = parsed
	return nil
}

// MarshalJSON 输出 RFC3339Nano 字符串。
func (t Time) MarshalJSON() ([]byte, error) { return json.Marshal(t.Format(storeLayout)) }

// NullTime 可空时间列。
type NullTime struct {
	Time  time.Time
	Valid bool
}

// FromPtr 由 *time.Time 构造 NullTime（nil → 空值）。
func FromPtr(p *time.Time) NullTime {
	if p == nil {
		return NullTime{}
	}
	return NullTime{Time: *p, Valid: true}
}

// ValidTime 由 time.Time 构造 NullTime。
func ValidTime(t time.Time) NullTime { return NullTime{Time: t, Valid: true} }

func (n NullTime) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Time.Format(storeLayout), nil
}

func (n *NullTime) Scan(v any) error {
	n.Valid = false
	if v == nil {
		return nil
	}
	parsed, err := parseTimeValue(v)
	if err != nil {
		return err
	}
	n.Time = parsed
	n.Valid = true
	return nil
}

func (n NullTime) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Time.Format(storeLayout))
}

func parseTimeValue(v any) (time.Time, error) {
	switch x := v.(type) {
	case time.Time:
		return x, nil
	case string:
		for _, l := range timeLayouts {
			if t, err := time.Parse(l, x); err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("无法解析时间文本 %q", x)
	case []byte:
		return parseTimeValue(string(x))
	default:
		return time.Time{}, fmt.Errorf("不支持的时间类型 %T", v)
	}
}

// ---------- 状态/动作常量 ----------

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

// ---------- 表模型 ----------

// Category 类别字典（决策基线 12：页面可维护）。
type Category struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Name      string `gorm:"uniqueIndex;not null" json:"name"`
	Sort      int    `gorm:"default:0" json:"sort"`
	Remark    string `json:"remark"`
	CreatedAt Time   `json:"created_at"`
	UpdatedAt Time   `json:"updated_at"`
}

func (Category) TableName() string { return "category" }

// Equipment 设备主表：保存“当前状态快照”，历史记录在 flow_record。
type Equipment struct {
	ID                    uint     `gorm:"primaryKey" json:"id"`
	EquipmentNo           *string  `gorm:"index:idx_equipment_no" json:"equipment_no"` // 真实编号；无编号设备为 NULL
	InternalCode          string   `gorm:"uniqueIndex;not null" json:"internal_code"`  // 系统内部码 EQ-xxxxx
	Name                  string   `gorm:"not null;index" json:"name"`
	Model                 string   `json:"model"`
	CategoryID            *uint    `gorm:"index" json:"category_id"`
	Status                string   `gorm:"not null;default:IN_STOCK;index" json:"status"`
	CurrentTeamID         *uint    `gorm:"index" json:"current_team_id"`
	CurrentBorrowerID     *uint    `gorm:"index" json:"current_borrower_id"`
	CurrentBorrowRecordID *uint    `gorm:"index" json:"current_borrow_record_id"`
	CurrentSince          NullTime `json:"current_since"` // 到达当前状态时间（交付时间口径）
	Remark                string   `json:"remark"`
	CreatedAt             Time     `json:"created_at"`
	UpdatedAt             Time     `json:"updated_at"`
}

func (Equipment) TableName() string { return "equipment" }

// Team 班组。停用不清除历史引用（决策 6）。
type Team struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	Name       string `gorm:"uniqueIndex;not null" json:"name"`
	Department string `json:"department"`
	IsActive   bool   `gorm:"default:true" json:"is_active"`
	Remark     string `json:"remark"`
	CreatedAt  Time   `json:"created_at"`
	UpdatedAt  Time   `json:"updated_at"`
}

func (Team) TableName() string { return "team" }

// Borrower 外借方。
type Borrower struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Name      string `gorm:"uniqueIndex;not null" json:"name"`
	Contact   string `json:"contact"`
	Phone     string `json:"phone"`
	IsActive  bool   `gorm:"default:true" json:"is_active"`
	Remark    string `json:"remark"`
	CreatedAt Time   `json:"created_at"`
	UpdatedAt Time   `json:"updated_at"`
}

func (Borrower) TableName() string { return "borrower" }

// BorrowRecord 外借单：一设备一单（决策 1/2）。
type BorrowRecord struct {
	ID                 uint     `gorm:"primaryKey" json:"id"`
	EquipmentID        uint     `gorm:"not null;index" json:"equipment_id"`
	BorrowerID         uint     `gorm:"not null;index" json:"borrower_id"`
	BorrowDate         Time     `gorm:"not null" json:"borrow_date"`
	ExpectedReturnDate NullTime `json:"expected_return_date"`
	ActualReturnDate   NullTime `json:"actual_return_date"`
	Status             string   `gorm:"not null;default:OUTSTANDING;index" json:"status"`
	Remark             string   `json:"remark"`
	CreatedAt          Time     `json:"created_at"`
	UpdatedAt          Time     `json:"updated_at"`
}

func (BorrowRecord) TableName() string { return "borrow_record" }

// Transaction 流转历史：永久保存、禁止删除；保存名称快照（决策 14）。
// 物理表名 flow_record（避开 SQLite 保留关键字 TRANSACTION）。
type Transaction struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	EquipmentID    uint   `gorm:"not null;index:idx_txn_equipment_occurred" json:"equipment_id"`
	Action         string `gorm:"not null;index" json:"action"`
	FromStatus     string `gorm:"not null" json:"from_status"`
	ToStatus       string `gorm:"not null" json:"to_status"`
	FromTeamID     *uint  `json:"from_team_id"`
	ToTeamID       *uint  `json:"to_team_id"`
	FromTeamName   string `json:"from_team_name"` // 名称快照
	ToTeamName     string `json:"to_team_name"`   // 名称快照
	BorrowerID     *uint  `json:"borrower_id"`
	BorrowerName   string `json:"borrower_name"` // 名称快照
	BorrowRecordID *uint  `gorm:"index" json:"borrow_record_id"`
	OccurredAt     Time   `gorm:"not null" json:"occurred_at"`
	Operator       string `gorm:"not null" json:"operator"`
	Remark         string `json:"remark"`
	CreatedAt      Time   `json:"created_at"`
}

func (Transaction) TableName() string { return "flow_record" }

// AuditLog 敏感操作留痕（决策 13：受限更正等）。
type AuditLog struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Action    string `gorm:"not null;index" json:"action"`
	Target    string `json:"target"`
	Detail    string `json:"detail"`
	Reason    string `json:"reason"`
	Operator  string `gorm:"not null" json:"operator"`
	CreatedAt Time   `json:"created_at"`
}

func (AuditLog) TableName() string { return "audit_log" }

// Setting 系统设置键值。
type Setting struct {
	Key   string `gorm:"primaryKey" json:"key"`
	Value string `json:"value"`
}

func (Setting) TableName() string { return "settings" }
