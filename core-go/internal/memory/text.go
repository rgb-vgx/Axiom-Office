// Package memory: ghi nho dai han (New_arch.md muc 8.5) - ban Go cua Memory/*.cs.
package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const (
	MaxFactLength = 300
	MaxQueryTerms = 12
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

// Normalize: chu thuong, bo dau, bo dau cau, gop khoang trang.
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

// Hash: SHA-256 cua text da chuan hoa (chong trung).
func Hash(text string) string {
	digest := sha256.Sum256([]byte(Normalize(text)))
	return hex.EncodeToString(digest[:])
}

// StopWords: tu dung (da chuan hoa) - bo khi dung truy van tu prompt.
var stopWords = map[string]bool{
	// tieng Viet
	"va": true, "voi": true, "cua": true, "cho": true, "la": true, "thi": true, "ma": true, "nhung": true,
	"cac": true, "mot": true, "nhieu": true, "nay": true, "do": true, "kia": true, "dang": true, "da": true,
	"se": true, "duoc": true, "bi": true, "tai": true, "trong": true, "ngoai": true, "tren": true, "duoi": true,
	"den": true, "tu": true, "ve": true, "theo": true, "nhu": true, "hay": true, "hoac": true, "neu": true,
	"khi": true, "de": true, "vao": true, "ra": true, "lai": true, "cung": true, "rat": true, "qua": true,
	"nua": true, "roi": true, "a": true, "nhe": true, "oi": true, "giup": true, "minh": true, "toi": true,
	"ban": true, "vui": true, "long": true, "xin": true, "can": true, "phai": true, "co": true, "khong": true,
	"gi": true, "nao": true, "sao": true, "the": true, "lam": true, "viet": true, "tao": true, "them": true,
	"sua": true, "bo": true, "o": true, "day": true, "ay": true, "vay": true, "thoi": true,
	// tieng Anh ("the", "a", "can" da co o phan tieng Viet)
	"an": true, "and": true, "or": true, "of": true, "to": true, "in": true, "on": true,
	"for": true, "with": true, "is": true, "are": true, "be": true, "this": true, "that": true, "it": true,
	"my": true, "me": true, "i": true, "you": true, "please": true, "make": true, "add": true,
	"from": true, "at": true, "by": true, "as": true,
}

func QueryTerms(text string) []string {
	terms := []string{}
	seen := map[string]bool{}
	for _, word := range strings.Fields(Normalize(text)) {
		if len([]rune(word)) <= 1 || stopWords[word] || seen[word] {
			continue
		}
		seen[word] = true
		terms = append(terms, word)
		if len(terms) >= MaxQueryTerms {
			break
		}
	}
	return terms
}

// FtsQuery: "w1" OR "w2"... (moi tu trong nhay kep: khong de ky tu dac biet cua FTS5 lam hong truy van).
func FtsQuery(terms []string) string {
	if len(terms) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(terms))
	for _, term := range terms {
		quoted = append(quoted, `"`+strings.ReplaceAll(term, `"`, "")+`"`)
	}
	return strings.Join(quoted, " OR ")
}

var (
	// Thong tin nhay cam khong duoc luu vao memory (muc 8.5.5): day 9-12 chu so lien (CCCD, tai khoan),
	// so the, mat khau, API key/token.
	longDigits  = regexp.MustCompile(`(?:^|\D)\d{9,12}(?:\D|$)`)
	cardNumber  = regexp.MustCompile(`(?:^|\D)(?:\d[ -]?){13,19}(?:\D|$)`)
	secretWords = regexp.MustCompile(`(?i)(mật\s*khẩu|mat\s*khau|password|passwd|pass\s*:|m[aậ]t\s*m[aã]\s*pin|\bpin\s*:|otp)`)
	apiKey      = regexp.MustCompile(`(?:^|\b)(sk-[A-Za-z0-9_-]{16,}|AIza[0-9A-Za-z_-]{20,}|AQ\.[0-9A-Za-z_-]{20,}|ghp_[A-Za-z0-9]{20,}|xox[bp]-[A-Za-z0-9-]{10,})`)
)

func IsSensitive(text string) bool {
	return longDigits.MatchString(text) || cardNumber.MatchString(text) ||
		secretWords.MatchString(text) || apiKey.MatchString(text)
}

// ValidDate: "YYYY-MM-DD" hop le, con lai -> "".
func ValidDate(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil || parsed.Format("2006-01-02") != trimmed {
		return ""
	}
	return trimmed
}

// Now: moc thoi gian luu trong CSDL (yyyy-MM-ddTHH:mm:ss.fffZ).
func Now() string { return time.Now().UTC().Format("2006-01-02T15:04:05.000Z") }
