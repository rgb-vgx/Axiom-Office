package tools

import (
	"fmt"
	"sync"
)

// ResultStore: giữ kết quả tool NGUYÊN VĂN khi lớp truncate chỉ trả về mẫu đầu/đuôi. Model đọc lại
// từng phần bằng tool read_result thay vì gọi lại bridge (tốn thêm một lượt) hoặc tự đoán phần bị cắt.
//
// Học từ Deep Agents (_message_eviction.py): kết quả quá lớn được ghi vào large_tool_results/{id},
// thay bằng preview + hướng dẫn đọc lại theo offset/limit. Axiom giữ trong bộ nhớ của lượt chạy (không
// ghi đĩa) nên nội dung tài liệu không nằm lại sau khi lượt kết thúc.
const (
	maxStoredResultBytes = 4 << 20  // 4 MB mỗi kết quả
	maxStoredTotalBytes  = 16 << 20 // 16 MB mỗi lượt
	ReadResultDefaultMax = 20_000   // ký tự mỗi lần đọc
	ReadResultMaxLimit   = 40_000
)

type ResultStore struct {
	mu    sync.Mutex
	items map[string]string
	order []string
	total int
}

func NewResultStore() *ResultStore {
	return &ResultStore{items: map[string]string{}}
}

// Put: lưu kết quả, trả id ("r1", "r2"...). Trả "" nếu quá lớn hoặc đã hết ngân sách của lượt.
func (s *ResultStore) Put(content string) string {
	if s == nil || content == "" || len(content) > maxStoredResultBytes {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.total+len(content) > maxStoredTotalBytes {
		return ""
	}
	id := fmt.Sprintf("r%d", len(s.order)+1)
	s.items[id] = content
	s.order = append(s.order, id)
	s.total += len(content)
	return id
}

// Get: nội dung đầy đủ của một kết quả đã lưu.
func (s *ResultStore) Get(id string) (string, bool) {
	if s == nil {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	content, ok := s.items[id]
	return content, ok
}

// Len: số kết quả đã lưu (test/dọn dẹp).
func (s *ResultStore) Len() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.order)
}
