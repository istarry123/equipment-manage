package importer

import (
	"fmt"
	"sort"

	"equipment/internal/models"
	"equipment/internal/service"

	"gorm.io/gorm"
)

// Reconcile（决策 18 / §四十七）：导入完成后把“Excel 解析的应导入设备”与
// “数据库实际写入该批次的设备”做逐台对账（以 source_key 为锚），任何差异明确列出。
// 对账只针对某批次（import_batch_id 匹配）；其他批次/手工设备不计入“多出”，避免误报。

// ReconDiff 单台差异。
type ReconDiff struct {
	SourceKey    string `json:"source_key"`
	DisplayNo    string `json:"display_no"`
	EquipmentNo  string `json:"equipment_no"`
	Name         string `json:"name"`
	Model        string `json:"model"`
	GroupRows    string `json:"group_rows"`
	InternalCode string `json:"internal_code,omitempty"`
	Side         string `json:"side"` // excel（缺失）/ db（多出）
	Detail       string `json:"detail,omitempty"`
}

// ReconcileReport 对账报告。
type ReconcileReport struct {
	BatchID        uint   `json:"batch_id"`
	SourceHash     string `json:"source_hash"`
	SourceName     string `json:"source_name"`
	TotalD         int    `json:"total_d"`          // Excel 台账数量
	ExcelOKDevs    int    `json:"excel_ok_devs"`    // Excel 解析且可导入台数（OK 组）
	ExcelBlockDevs int    `json:"excel_block_devs"` // Excel 解析但被 BLOCK/REVIEW 未导入台数
	DBBatchDevs    int    `json:"db_batch_devs"`    // 数据库该批次设备数
	NumExcel       int    `json:"num_excel"`        // Excel 有编号台数（OK）
	NumDB          int    `json:"num_db"`           // DB 该批次有编号台数
	UnExcel        int    `json:"unnumbered_excel"`
	UnDB           int    `json:"unnumbered_db"`
	// 同号多台（no+name+model 计数>1）组数
	DupGroupsExcel int `json:"dup_groups_excel"`
	DupGroupsDB    int `json:"dup_groups_db"`
	// 历史借出无法匹配（Excel 侧解析统计；§四十七）
	BorrowUnmatched int         `json:"borrow_unmatched"`
	Missing         []ReconDiff `json:"missing,omitempty"` // Excel 有、DB 无
	Extra           []ReconDiff `json:"extra,omitempty"`   // DB 有、Excel 无
	Pass            bool        `json:"pass"`
	Message         string      `json:"message"`
}

// reconRow DB 该批次设备行（Scan 用）。
type reconRow struct {
	ID           uint
	EquipmentNo  *string
	EquipmentSeq int
	Name         string
	Model        string
	SourceKey    string
	InternalCode string
	NoGrpCount   int64
}

// Reconcile 对账（db 中须已存在 res 对应的批次记录）。
func Reconcile(db *gorm.DB, res *ParseResult) (*ReconcileReport, error) {
	rep := &ReconcileReport{
		SourceHash: res.SourceHash, SourceName: res.Filename,
		TotalD: res.TotalD,
	}
	// 1) 找批次
	var batch models.ImportBatch
	if res.SourceHash == "" {
		return nil, fmt.Errorf("解析结果缺少文件指纹（source_hash），无法关联批次")
	}
	if err := db.Where("source_hash = ? AND status = ?", res.SourceHash, batchStatusDone).First(&batch).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("数据库中不存在该文件对应的导入批次（source_hash=%s）；请先执行导入再对账", res.SourceHash)
		}
		return nil, err
	}
	rep.BatchID = batch.ID

	// 2) Excel 期望设备（仅 OK 组可导入行）
	preview := ExpandPreviewDevices(res)
	expByKey := map[string]*PreviewDevice{}
	expOK := 0
	blockDevs := 0
	for _, d := range preview {
		if d.OK {
			expByKey[d.SourceKey] = d
			expOK++
			if !d.Unnumbered {
				rep.NumExcel++
			} else {
				rep.UnExcel++
			}
		} else {
			blockDevs++
		}
	}
	rep.ExcelOKDevs = expOK
	rep.ExcelBlockDevs = blockDevs

	// 3) DB 该批次设备
	var rows []reconRow
	if err := db.Table("equipment").
		Select("equipment.id, equipment.equipment_no, equipment.equipment_seq, equipment.name, equipment.model, "+
			"equipment.source_key, equipment.internal_code, "+service.EquipmentSelectNoGrp).
		Where("equipment.import_batch_id = ?", batch.ID).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	dbByKey := map[string]reconRow{}
	for _, r := range rows {
		dbByKey[r.SourceKey] = r
		if r.EquipmentNo == nil {
			rep.UnDB++
		} else {
			rep.NumDB++
		}
	}
	rep.DBBatchDevs = len(rows)

	// 4) 缺失（Excel 有、DB 无）
	for _, sk := range sortedKeys(expByKey) {
		d := expByKey[sk]
		if _, ok := dbByKey[sk]; ok {
			continue
		}
		rep.Missing = append(rep.Missing, ReconDiff{
			SourceKey: sk, DisplayNo: d.DisplayNo, EquipmentNo: noStr(d.No),
			Name: d.Name, Model: d.Model, GroupRows: d.GroupRows,
			Side: "excel", Detail: "Excel 解析可导入，但数据库未写入该台",
		})
	}
	// 5) 多出（DB 有、Excel 无）
	for _, sk := range sortedRowKeys(dbByKey) {
		r := dbByKey[sk]
		if _, ok := expByKey[sk]; ok {
			continue
		}
		rep.Extra = append(rep.Extra, ReconDiff{
			SourceKey:   sk,
			DisplayNo:   service.DisplayNo(r.EquipmentNo, r.Name, r.Model, r.EquipmentSeq, r.NoGrpCount),
			EquipmentNo: noStr(r.EquipmentNo),
			Name:        r.Name, Model: r.Model,
			InternalCode: r.InternalCode,
			Side:         "db", Detail: "数据库存在该 source_key，但 Excel 解析中未出现（异常）",
		})
	}

	// 6) 同号多台组数
	rep.DupGroupsExcel = dupGroupsInPreview(expByKey)
	rep.DupGroupsDB = dupGroupsInDB(rows)

	// 7) 历史借出无法匹配（Excel 侧解析统计）
	if res.Derived != nil {
		rep.BorrowUnmatched = res.Derived.BorrowUnmatched
	}

	// 8) 判定
	rep.Pass = len(rep.Missing) == 0 && len(rep.Extra) == 0 &&
		rep.ExcelOKDevs == rep.DBBatchDevs
	if rep.Pass {
		rep.Message = fmt.Sprintf("对账通过（PASS）：Excel 可导入 %d 台 与 数据库该批次 %d 台 完全一致，无缺失/多出",
			rep.ExcelOKDevs, rep.DBBatchDevs)
	} else {
		rep.Message = fmt.Sprintf("对账失败（FAIL）：Excel 可导入 %d 台，数据库该批次 %d 台；缺失 %d 台、多出 %d 台",
			rep.ExcelOKDevs, rep.DBBatchDevs, len(rep.Missing), len(rep.Extra))
	}
	return rep, nil
}

func noStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func sortedKeys(m map[string]*PreviewDevice) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func sortedRowKeys(m map[string]reconRow) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// dupGroupsInPreview 统计 Excel 预览中 同号多台（编号相同且组内出现>1）的“编号组”数。
func dupGroupsInPreview(exp map[string]*PreviewDevice) int {
	seen := map[string]int{} // 组+编号 → 出现次数
	for _, d := range exp {
		if d.No == nil {
			continue
		}
		key := d.Name + "\x00" + d.Model + "\x00" + *d.No
		seen[key]++
	}
	n := 0
	for _, c := range seen {
		if c > 1 {
			n++
		}
	}
	return n
}

// dupGroupsInDB 统计 DB 中同号多台的“编号组”数（按 no+name+model，但仅统计该批次行范围）。
func dupGroupsInDB(rows []reconRow) int {
	seen := map[string]int{}
	for _, r := range rows {
		if r.EquipmentNo == nil {
			continue
		}
		key := r.Name + "\x00" + r.Model + "\x00" + *r.EquipmentNo
		seen[key]++
	}
	n := 0
	for _, c := range seen {
		if c > 1 {
			n++
		}
	}
	return n
}
