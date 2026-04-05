package repository

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	_ "github.com/lib/pq" // PostgreSQL驱动
	"gorm.io/gorm"

	"github.com/chenzanhong/formallanglab-ai/internal/domain/model"
	"github.com/chenzanhong/zlog"
)

type QACacheLoader struct{}

func NewQACacheLoader() *QACacheLoader {
	return &QACacheLoader{}
}

// aliasPattern 用于匹配 Markdown 文件中的 aliases 元数据注释行，
// 格式为 <!-- aliases: ["问题1","问题2"] -->
var aliasPattern = regexp.MustCompile(`<!--\s*aliases:\s*(\[.*?\])\s*-->`)

// LoadCache 从指定目录递归加载所有 .md 文件，解析其中的 aliases 元数据，并将"问题→答案"映射写入缓存。
// 每个文件的答案内容默认去掉首行的 aliases 注释行。
func (l *QACacheLoader) LoadCache(cache *model.QACache, dirPath string) error {
	wd, _ := os.Getwd()
	qaCachePath := filepath.Join(wd, dirPath)
	// 使用 filepath.WalkDir 遍历目录，仅处理 .md 文件
	return filepath.WalkDir(qaCachePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// 跳过子目录与非 Markdown 文件
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		// 读取文件全部内容
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("读取文件失败 %s: %w", path, err)
		}

		text := string(content)
		// 提取 aliases 元数据
		matches := aliasPattern.FindStringSubmatch(text)
		if len(matches) < 2 {
			zlog.Warnw("QA cache file missing aliases metadata", "file", path)
			return nil
		}

		// 将 JSON 数组字符串解析为字符串切片
		var aliases []string
		if err := json.Unmarshal([]byte(matches[1]), &aliases); err != nil {
			return fmt.Errorf("解析 aliases 失败 in %s: %w", path, err)
		}

		// 提取 answer：去掉第一行（即注释行）
		scanner := bufio.NewScanner(strings.NewReader(text))
		var answerLines []string
		firstLine := true
		for scanner.Scan() {
			line := scanner.Text()
			if firstLine && aliasPattern.MatchString(line) {
				firstLine = false
				continue // 跳过元数据行
			}
			answerLines = append(answerLines, line)
		}
		answer := strings.Join(answerLines, "\n")

		// 存入缓存
		for _, alias := range aliases {
			alias = strings.TrimSpace(alias)
			if alias != "" {
				cache.Set(alias, answer)
			}
		}

		return nil
	})
}

func (l *QACacheLoader) LoadCacheFile(cache *model.QACache, filepath string) error {
	// 读取文件全部内容
	content, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("读取文件失败 %s: %w", filepath, err)
	}

	text := string(content)
	// 提取 aliases 元数据
	matches := aliasPattern.FindStringSubmatch(text)
	if len(matches) < 2 {
		zlog.Warnw("QA cache file missing aliases metadata", "file", filepath)
		return nil
	}

	// 将 JSON 数组字符串解析为字符串切片
	var aliases []string
	if err := json.Unmarshal([]byte(matches[1]), &aliases); err != nil {
		return fmt.Errorf("解析 aliases 失败 in %s: %w", filepath, err)
	}

	// 提取 answer：去掉第一行（即注释行）
	scanner := bufio.NewScanner(strings.NewReader(text))
	var answerLines []string
	firstLine := true
	for scanner.Scan() {
		line := scanner.Text()
		if firstLine && aliasPattern.MatchString(line) {
			firstLine = false
			continue // 跳过元数据行
		}
		answerLines = append(answerLines, line)
	}
	answer := strings.Join(answerLines, "\n")

	// 存入缓存
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias != "" {
			cache.Set(alias, answer)
		}
	}

	return nil
}

// QACacheRepository 数据库仓库，用于从PostgreSQL加载和管理QA缓存
// 注意：这是新增的结构体，不影响现有的QACacheLoader实现
type QACacheRepository struct {
	db *gorm.DB
}

// NewQACacheRepository 创建数据库仓库实例
func NewQACacheRepository(db *gorm.DB) *QACacheRepository {
	return &QACacheRepository{
		db: db,
	}
}

// LoadCacheFromDB 从数据库加载QA缓存
func (r *QACacheRepository) LoadCacheFromDB(cache *model.QACache) error {
	// 定义一个结构体用于接收查询结果
	type QACacheRecord struct {
		Content string
	}

	var records []QACacheRecord
	// 使用GORM查询所有QA缓存记录
	if err := r.db.Model(&model.QACacheItem{}).Select("content").Find(&records).Error; err != nil {
		return fmt.Errorf("查询数据库失败: %w", err)
	}

	// 遍历每一条记录
	for _, record := range records {
		content := record.Content

		// 解析content中的aliases元数据和答案（与文件加载逻辑相同）
		matches := aliasPattern.FindStringSubmatch(content)
		if len(matches) < 2 {
			zlog.Warnw("QA cache database record missing aliases metadata")
			continue
		}

		// 解析aliases
		var aliases []string
		if err := json.Unmarshal([]byte(matches[1]), &aliases); err != nil {
			return fmt.Errorf("解析aliases失败: %w", err)
		}

		// 提取answer：去掉第一行（即注释行）
		scanner := bufio.NewScanner(strings.NewReader(content))
		var answerLines []string
		firstLine := true
		for scanner.Scan() {
			line := scanner.Text()
			if firstLine && aliasPattern.MatchString(line) {
				firstLine = false
				continue // 跳过元数据行
			}
			answerLines = append(answerLines, line)
		}
		answer := strings.Join(answerLines, "\n")

		// 存入缓存
		for _, alias := range aliases {
			alias = strings.TrimSpace(alias)
			if alias != "" {
				cache.Set(alias, answer)
			}
		}
	}

	return nil
}

// SaveCacheToDB 将缓存内容保存到数据库
func (r *QACacheRepository) SaveCacheToDB(items []model.QACacheItem) error {
	// 使用GORM的事务功能
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 批量插入数据
		if err := tx.Create(&items).Error; err != nil {
			return fmt.Errorf("插入数据失败: %w", err)
		}

		return nil
	})
}

// GetAllItems 获取所有缓存项
func (r *QACacheRepository) GetAllItems() ([]model.QACacheItem, error) {
	var items []model.QACacheItem
	// 使用GORM查询所有缓存项
	if err := r.db.Find(&items).Error; err != nil {
		return nil, fmt.Errorf("查询数据库失败: %w", err)
	}

	return items, nil
}
