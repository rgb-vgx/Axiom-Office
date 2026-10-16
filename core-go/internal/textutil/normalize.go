package textutil

import (
	"strings"
	"unicode"
)

// Bang bo dau tieng Viet tuong minh. Ban .NET phai lam viec nay vi chay InvariantGlobalization (khong ICU):
// string.Normalize(FormD) khong tach duoc dau. Go cung khong co bang phan ra Unicode trong stdlib, nen dung
// cung cach: phu het nguyen am tieng Viet + 'đ'. (Chu Latin co dau khac chi duoc bo dau neu van ban da o
// dang phan ra san - khac ban .NET mot chut, khong anh huong tieng Viet.)
var vietnameseBase = buildVietnameseMap()

func buildVietnameseMap() map[rune]rune {
	groups := []struct {
		base     rune
		accented string
	}{
		{'a', "àáảãạăằắẳẵặâầấẩẫậ"},
		{'e', "èéẻẽẹêềếểễệ"},
		{'i', "ìíỉĩị"},
		{'o', "òóỏõọôồốổỗộơờớởỡợ"},
		{'u', "ùúủũụưừứửữự"},
		{'y', "ỳýỷỹỵ"},
		{'d', "đ"},
	}
	table := map[rune]rune{}
	for _, group := range groups {
		for _, accented := range group.accented {
			table[accented] = group.base
			table[unicode.ToUpper(accented)] = group.base
		}
	}
	return table
}

// Normalize: chu thuong, bo dau (tieng Viet + dau ket hop), bo dau cau, gop khoang trang.
// Dung chung cho memory (tim kiem) va kho nguon web (cot search_text cua FTS).
func Normalize(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	var builder strings.Builder
	space := false
	for _, char := range text {
		if plain, ok := vietnameseBase[char]; ok {
			builder.WriteRune(plain)
			space = false
			continue
		}
		switch {
		case unicode.Is(unicode.Mn, char):
			// Dau ket hop (van ban da phan ra san): bo.
		case unicode.IsLetter(char) || unicode.IsDigit(char):
			builder.WriteRune(unicode.ToLower(char))
			space = false
		case !space && builder.Len() > 0:
			builder.WriteRune(' ')
			space = true
		}
	}
	return strings.TrimSpace(builder.String())
}
