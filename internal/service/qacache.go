package service

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/chenzanhong/formallanglab-ai/internal/domain/model"
	"github.com/chenzanhong/formallanglab-ai/internal/repository"
)

// QACacheService 提供 QA 缓存服务接口
type QACacheService struct {
	cache        *model.QACache
	fileLoader   *repository.QACacheLoader
	dbRepository *repository.QACacheRepository
}

// NewQACacheService 创建新的缓存服务实例
func NewQACacheService(db *gorm.DB, dirPath string) *QACacheService {
	cache := model.NewQACache()
	fileLoader := repository.NewQACacheLoader()
	var dbRepository *repository.QACacheRepository

	// 如果提供了数据库连接，则初始化数据库仓库
	if db != nil {
		dbRepository = repository.NewQACacheRepository(db)
	}

	service := &QACacheService{
		cache:        cache,
		fileLoader:   fileLoader,
		dbRepository: dbRepository,
	}

	// 初始化时根据配置加载缓存
	service.InitializeCache(dirPath)

	return service
}

// InitializeCache 根据配置初始化缓存
func (s *QACacheService) InitializeCache(dirPath string) error {
	// 从文件系统加载缓存
	if dirPath != "" {
		if err := s.LoadCacheFromFile(dirPath); err != nil {
			return fmt.Errorf("从文件加载缓存失败: %w", err)
		}
	}
	// 从数据库加载缓存
	if s.dbRepository != nil {
		if err := s.LoadCacheFromDB(); err != nil {
			return fmt.Errorf("从数据库加载缓存失败: %w", err)
		}
	}

	return nil
}

// LoadCacheFromDB 从数据库加载缓存
func (s *QACacheService) LoadCacheFromDB() error {
	if s.dbRepository == nil {
		return fmt.Errorf("数据库仓库未初始化")
	}

	return s.dbRepository.LoadCacheFromDB(s.cache)
}

// LoadCacheFromFile 从文件系统加载缓存
func (s *QACacheService) LoadCacheFromFile(dirPath string) error {
	return s.fileLoader.LoadCache(s.cache, dirPath)
}

// LoadSingleCacheFile 加载单个缓存文件
func (s *QACacheService) LoadSingleCacheFile(filePath string) error {
	return s.fileLoader.LoadCacheFile(s.cache, filePath)
}

// GetAnswer 获取问题的答案
func (s *QACacheService) GetAnswer(question string) string {
	return s.cache.Get(question)
}

// AddQA 手动添加问答对到内存缓存
func (s *QACacheService) AddQA(question, answer string) {
	s.cache.Set(question, answer)
}

// RemoveQA 从内存缓存中移除问答对
func (s *QACacheService) RemoveQA(question string) {
	s.cache.Delete(question)
}

// SaveToDB 将缓存保存到数据库
func (s *QACacheService) SaveToDB(items []model.QACacheItem) error {
	if s.dbRepository == nil {
		return fmt.Errorf("数据库仓库未初始化")
	}

	return s.dbRepository.SaveCacheToDB(items)
}

// GetAllDBItems 获取数据库中所有缓存项
func (s *QACacheService) GetAllDBItems() ([]model.QACacheItem, error) {
	if s.dbRepository == nil {
		return nil, fmt.Errorf("数据库仓库未初始化")
	}

	return s.dbRepository.GetAllItems()
}
