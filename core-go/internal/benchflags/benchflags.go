// Package benchflags: công tắc CHỈ DÙNG CHO BENCHMARK, để so từng cải tiến trên cùng một binary.
//
// Đặt AXIOM_BENCH_VARIANT=B0..B6 khi chạy benchmark:
//
//	B0  tất cả cải tiến TẮT (hành vi cũ)
//	B1  chỉ HarnessProfile (ngân sách/chế độ theo model)
//	B2  chỉ mô tả lệnh gọn
//	B3  chỉ tách system prompt ổn định/động
//	B4  chỉ chèn checklist mỗi vòng
//	B5  chỉ giữ kết quả bị cắt + read_result
//	B6  tất cả bật (cấu hình tích hợp)
//
// Không đặt biến = chạy production như hiện tại (HarnessProfile tự quyết định compact theo cửa sổ model).
// Các nhánh B0–B5 phục vụ ablation; B6 chỉ để kiểm tra tương tác, không dùng để quy công cho một thay đổi.
package benchflags

import (
	"os"
	"strings"
)

const Env = "AXIOM_BENCH_VARIANT"

const (
	Harness   = "harness"
	Compact   = "compact"
	Prompt    = "prompt"
	Checklist = "checklist"
	Results   = "results"
)

// Variant: giá trị AXIOM_BENCH_VARIANT đã chuẩn hoá ("" = production).
func Variant() string {
	return strings.ToUpper(strings.TrimSpace(os.Getenv(Env)))
}

// Enabled: feature có bật trong nhánh benchmark hiện tại không. Không đặt biến = bật (production).
func Enabled(feature string) bool {
	switch Variant() {
	case "":
		return true
	case "B0":
		return false
	case "B1":
		return feature == Harness
	case "B2":
		return feature == Compact
	case "B3":
		return feature == Prompt
	case "B4":
		return feature == Checklist
	case "B5":
		return feature == Results
	case "B6":
		return true
	default:
		// Nhánh lạ: coi như production, để không âm thầm tắt tính năng khi gõ sai.
		return true
	}
}

// CompactAuto: production để HarnessProfile quyết định compact theo cửa sổ model; khi chạy
// benchmark thì nhánh quyết định (B1 không compact, B2 compact, B0 không).
func CompactAuto() bool {
	variant := Variant()
	return variant == "" || !knownVariant(variant)
}

func knownVariant(variant string) bool {
	switch variant {
	case "B0", "B1", "B2", "B3", "B4", "B5", "B6":
		return true
	}
	return false
}

// Summary: nhãn ngắn để ghi vào trace/run manifest.
func Summary() string {
	if variant := Variant(); variant != "" {
		return variant
	}
	return "production"
}
