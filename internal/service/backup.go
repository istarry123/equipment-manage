package service

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Backup 相关：目录 backup/，文件名 equipment-YYYY-MM-DD_HHMMSS.db。
// 采用 VACUUM INTO 生成一致性快照（WAL 安全）。

// BackupFile 备份文件信息。
type BackupFile struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

var (
	ErrBackupDir = errors.New("备份目录不可用")
	ErrNoBackup  = errors.New("备份文件不存在或非法")
)

func backupPrefix(dbFile string) string {
	base := filepath.Base(dbFile)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return name + "-"
}

func backupName(dbFile string) string {
	return backupPrefix(dbFile) + time.Now().Format("2006-01-02_150405") + ".db"
}

func backupDir() string { return "backup" }

func ensureDir() error {
	return os.MkdirAll(backupDir(), 0o755)
}

// execVACUUMInto 对 db 执行一致性快照到 path。
func execVACUUMInto(db *gorm.DB, path string) error {
	esc := strings.ReplaceAll(path, "'", "''")
	return db.Exec("VACUUM INTO '" + esc + "'").Error
}

// BackupNow 立即备份并修剪超量旧备份，返回文件名（同秒冲突自动加序号）。
func BackupNow(db *gorm.DB, dbFile string, keep int) (string, error) {
	if err := ensureDir(); err != nil {
		return "", fmt.Errorf("%w: %v", ErrBackupDir, err)
	}
	base := backupName(dbFile)
	name := base
	path := filepath.Join(backupDir(), name)
	for i := 1; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		name = strings.TrimSuffix(base, ".db") + fmt.Sprintf("_%d.db", i)
		path = filepath.Join(backupDir(), name)
	}
	if err := execVACUUMInto(db, path); err != nil {
		return "", fmt.Errorf("备份失败: %w", err)
	}
	if keep > 0 {
		PruneBackups(keep)
	}
	return name, nil
}

// PruneBackups 只保留最近 keep 份。
func PruneBackups(keep int) {
	files := listFiles()
	for i := keep; i < len(files); i++ {
		os.Remove(filepath.Join(backupDir(), files[i].Name)) //nolint:errcheck
	}
}

// ListBackups 返回备份列表（新→旧）。
func ListBackups() ([]BackupFile, error) {
	if err := ensureDir(); err != nil {
		return nil, err
	}
	return listFiles(), nil
}

func listFiles() []BackupFile {
	entries, _ := os.ReadDir(backupDir())
	var out []BackupFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, BackupFile{
			Name:    e.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	return out
}

// BackupFilePath 校验并返回备份文件路径（仅允许 backup/ 目录下的 .db，防目录穿越）。
func BackupFilePath(name string) (string, error) {
	if err := ensureDir(); err != nil {
		return "", err
	}
	clean := filepath.Base(name)
	if clean != name || !strings.HasSuffix(clean, ".db") || strings.HasPrefix(clean, ".") {
		return "", ErrNoBackup
	}
	path := filepath.Join(backupDir(), name)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", ErrNoBackup
	}
	return path, nil
}

// FileReplace 用备份文件原子替换数据库文件（调用方须先关闭数据库连接）。
func FileReplace(backupPath, dbFile string) error {
	from, err := os.Open(backupPath)
	if err != nil {
		return ErrNoBackup
	}
	defer from.Close()
	tmp := dbFile + ".restore.tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, from); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	out.Close()
	if err := os.Rename(tmp, dbFile); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
