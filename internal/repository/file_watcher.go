package repository

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/chenzanhong/formallanglab-ai/internal/domain/model"
	"github.com/chenzanhong/zlog"
	"github.com/fsnotify/fsnotify"
)

// QACacheWatcher 文件系统监控器，用于自动更新QA缓存
type QACacheWatcher struct {
	watcher       *fsnotify.Watcher
	watchedDir    string
	qaCache       model.QACache
	qaCacheLoader *QACacheLoader
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// NewQACacheWatcher 创建新的文件系统监控器
func NewQACacheWatcher(watchedDir string, qaCache model.QACache, qaCacheLoader *QACacheLoader) (*QACacheWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &QACacheWatcher{
		watcher:       watcher,
		watchedDir:    watchedDir,
		qaCache:       qaCache,
		qaCacheLoader: qaCacheLoader,
		stopChan:      make(chan struct{}),
	}, nil
}

// Start 启动文件系统监控
func (w *QACacheWatcher) Start() error {
	// 确保目录存在
	if _, err := os.Stat(w.watchedDir); os.IsNotExist(err) {
		zlog.Warnw("Watched directory does not exist, creating", "dir", w.watchedDir)
		if err := os.MkdirAll(w.watchedDir, 0o755); err != nil {
			return err
		}
	}

	// 添加目录到监控
	if err := w.watcher.Add(w.watchedDir); err != nil {
		return err
	}

	zlog.Infow("Started watching QA cache directory", "dir", w.watchedDir)

	// 启动监控协程
	w.wg.Add(1)
	go w.watch()

	return nil
}

// watch 监控文件变化
func (w *QACacheWatcher) watch() {
	defer w.wg.Done()

	// 用于防抖
	debounceTimer := make(map[string]*time.Timer)
	debounceMu := sync.Mutex{}
	debounceDuration := 2 * time.Second

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// 只处理Markdown文件
			if filepath.Ext(event.Name) != ".md" {
				continue
			}

			// 防抖处理，避免频繁更新
			debounceMu.Lock()
			if timer, exists := debounceTimer[event.Name]; exists {
				timer.Stop()
			}
			debounceTimer[event.Name] = time.AfterFunc(debounceDuration, func() {
				w.handleFileChange(event)
				debounceMu.Lock()
				delete(debounceTimer, event.Name)
				debounceMu.Unlock()
			})
			debounceMu.Unlock()

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			zlog.Errorw("Error watching file system", "error", err)

		case <-w.stopChan:
			return
		}
	}
}

// handleFileChange 处理文件变化
func (w *QACacheWatcher) handleFileChange(event fsnotify.Event) {
	filePath := event.Name
	fileName := filepath.Base(filePath)

	zlog.Infow("Detected file change", "file", fileName, "event", event.Op.String())

	switch {
	case event.Op&(fsnotify.Write|fsnotify.Create) != 0:
		// 文件创建或修改，重新加载该文件
		w.reloadSingleFile(filePath)

	case event.Op&fsnotify.Remove != 0:
		// 文件删除，从缓存中移除相关条目
		w.removeFileFromCache(filePath)

	case event.Op&fsnotify.Rename != 0:
		// 文件重命名，先移除旧条目，然后如果新文件存在则加载
		w.removeFileFromCache(filePath)
	}
}

// reloadSingleFile 重新加载单个文件到缓存
func (w *QACacheWatcher) reloadSingleFile(filePath string) {
	// 解析文件并更新缓存
	w.qaCacheLoader.LoadCacheFile(&w.qaCache, filePath)

	// 保存布隆过滤器状态到Redis
	// if redisCache, ok := w.qaCache.(*model.RedisQACache); ok {
	// 	if err := redisCache.SaveBloomFilterToRedis(); err != nil {
	// 		zlog.Warnw("Failed to save bloom filter to Redis", "error", err)
	// 	}
	// }

	zlog.Infow("Successfully reloaded file into cache", "file", filePath)
}

// removeFileFromCache 从缓存中移除与文件相关的条目
func (w *QACacheWatcher) removeFileFromCache(filePath string) {
	// 这里简化处理，实际上应该记录每个问题来自哪个文件
	// 为了简化，可以在文件中添加特殊标记或维护一个映射表
	// 目前的实现是重新扫描整个目录，只保留存在的文件中的问题

	// 重新加载所有文件
	if err := w.qaCacheLoader.LoadCache(&w.qaCache, w.watchedDir); err != nil {
		zlog.Errorw("Failed to reload cache after file deletion", "error", err)
	}
}

// Stop 停止文件系统监控
func (w *QACacheWatcher) Stop() {
	close(w.stopChan)
	w.watcher.Close()
	w.wg.Wait()
	zlog.Info("Stopped file system watcher")
}
