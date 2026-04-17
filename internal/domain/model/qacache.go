package model

import (
	"regexp"
	"strings"
	"time"
)

type QACache struct {
	data map[string]string // map[问题]答案
}

func NewQACache() *QACache {
	return &QACache{
		data: make(map[string]string),
	}
}

func (c *QACache) Get(key string) string {
	key = c.normalize(key)
	return c.data[key]
}

var spaceRegex = regexp.MustCompile(`\s+`)

// normalize 标准化提问 key
// 1. 去除前后空白
// 2. 去除行内所有空格
// 3. 去除末尾中英文句子常见的结尾标点符号
// 考虑到一些名称，不改变大小写
func (c *QACache) normalize(key string) string {
	// 去除前后空白
	key = strings.TrimSpace(key)
	// 压缩连续空格
	key = spaceRegex.ReplaceAllString(key, "")
	// 只去除真正可能出现在句尾的结束标点
	key = strings.TrimRightFunc(key, func(r rune) bool {
		return r == '。' || r == '！' || r == '？' ||
			r == '.' || r == '!' || r == '?'
	})

	return key
}

func (c *QACache) Set(key, value string) {
	c.data[key] = value
}

func (c *QACache) Delete(key string) {
	delete(c.data, key)
}

// QACacheItem 数据库模型，用于 PostgreSQL 存储
// 注意：这是新增的结构体，不影响现有的 QACache 实现
type QACacheItem struct {
	ID        int       `json:"id" db:"id"`
	Content   string    `json:"content" db:"content"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

func (QACacheItem) TableName() string {
	return "qacache"
}
