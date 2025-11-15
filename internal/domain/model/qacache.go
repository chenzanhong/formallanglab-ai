package model

import (
	"fmt"
	"regexp"
	"strings"
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
	fmt.Println("规格化后的提问：", key)
	return c.data[key]
}

var spaceRegex = regexp.MustCompile(`\s+`)

// 标准化提问key
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
