package model

/*
MockStream 模拟一个流式响应，用于响应预置高频问题缓存
*/

type MockStream struct {
	chunks []string
	index  int
	length int
}

func NewMockStream(aiResponse string) *MockStream {
	chunks := splitIntoChunks(aiResponse, 12)
	return &MockStream{
		chunks: chunks,
		index:  0,
		length: len(chunks),
	}
}

func splitIntoChunks(str string, chunkSize int) []string {
	chunks := make([]string, 0, len(str)/chunkSize+1)
	runes := []rune(str)
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}

func (m *MockStream) Next() bool {
	return m.index < m.length
}

func (m *MockStream) Current() string {
	if m.index >= m.length {
		return ""
	}
	m.index++
	return m.chunks[m.index-1]
}

func (m *MockStream) Err() error {
	return nil
}