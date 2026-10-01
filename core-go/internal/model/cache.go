package model

// So token prompt doc tu cache la so DO, khong phai tham so.
//
// DeepSeek (proxy 9router) va OpenAI cache ngam theo tien to on dinh cua hoi thoai - khong co truong
// nao phai gui len de bat. Viec cua minh chi la: (1) giu tien to that on dinh de cache trung, va
// (2) doc so nay ra de BIET no co trung khong. Truoc 02/10/2026 khong doc, nen khong ai biet.
//
// Ba cach goi ten khac nhau, moi nha cung cap mot kieu:
//   - DeepSeek: `prompt_cache_hit_tokens` / `prompt_cache_miss_tokens`
//   - OpenAI:   `prompt_tokens_details.cached_tokens`
//   - Anthropic: `cache_read_input_tokens` (doc) / `cache_creation_input_tokens` (ghi)
//
// Lay so LON NHAT trong cac ung vien: vai gateway dien ca hai truong, va truong nao cung chi dem
// phan doc tu cache. Khong co truong nao -> 0, nghia la "khong cache hoac khong bao" - KHONG phai loi.
func cachedFrom(candidates ...int) int {
	best := 0
	for _, value := range candidates {
		if value > best {
			best = value
		}
	}
	return best
}
