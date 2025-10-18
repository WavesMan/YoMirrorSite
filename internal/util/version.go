package util

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// VersionRecord 版本记录
type VersionRecord struct {
	RepositoryURL string `json:"repository_url"`
	Branch        string `json:"branch"`
	LatestVersion string `json:"latest_version"`
	LastUpdated   string `json:"last_updated"`
}

// VersionManager 版本管理器
type VersionManager struct {
	filePath string
	records  []VersionRecord
	mu       sync.RWMutex
}

// NewVersionManager 创建版本管理器
func NewVersionManager(filePath string) (*VersionManager, error) {
	vm := &VersionManager{
		filePath: filePath,
	}

	// 创建目录（如果不存在）
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	// 加载现有版本记录
	if err := vm.load(); err != nil {
		// 如果文件不存在，创建一个新的
		if os.IsNotExist(err) {
			vm.records = make([]VersionRecord, 0)
			return vm, nil
		}
		return nil, err
	}

	return vm, nil
}

// GetLatestVersion 获取版本记录
func (vm *VersionManager) GetVersion(repoURL, branch string) (string, bool) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	for _, record := range vm.records {
		if record.RepositoryURL == repoURL && record.Branch == branch {
			return record.LatestVersion, true
		}
	}

	return "", false
}

// UpdateVersion 更新版本记录
func (vm *VersionManager) UpdateVersion(repoURL, branch, version, lastUpdated string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	found := false
	for i, record := range vm.records {
		if record.RepositoryURL == repoURL && record.Branch == branch {
			vm.records[i].LatestVersion = version
			vm.records[i].LastUpdated = lastUpdated
			found = true
			break
		}
	}

	if !found {
		vm.records = append(vm.records, VersionRecord{
			RepositoryURL: repoURL,
			Branch:        branch,
			LatestVersion: version,
			LastUpdated:   lastUpdated,
		})
	}

	return vm.save()
}

// load 加载版本记录
func (vm *VersionManager) load() error {
	data, err := os.ReadFile(vm.filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &vm.records)
}

// save 保存版本记录
func (vm *VersionManager) save() error {
	data, err := json.MarshalIndent(vm.records, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(vm.filePath, data, 0644)
}
