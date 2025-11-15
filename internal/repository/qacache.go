package repository

import (
	"ai/internal/domain/model"
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	wd,_ := os.Getwd()
	qaCachePath := filepath.Join(wd, dirPath)
	fmt.Println("qaCachePath: ", qaCachePath)
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
		// fmt.Printf("%+v\n", matches)
		if len(matches) < 2 {
			// 没有找到 aliases，跳过或记录警告
			fmt.Printf("警告：文件 %s 缺少 aliases 元数据\n", path)
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
