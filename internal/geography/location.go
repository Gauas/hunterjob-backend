package geography

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const HoChiMinhCity = "Ho Chi Minh City"

// IsHoChiMinh recognizes common spellings and district names used by job boards.
func IsHoChiMinh(value string) bool {
	var normalized strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(value)) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case r == 'đ':
			normalized.WriteByte('d')
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			normalized.WriteRune(r)
		default:
			normalized.WriteByte(' ')
		}
	}
	location := " " + strings.Join(strings.Fields(normalized.String()), " ") + " "
	for _, city := range []string{"ho chi minh", "hochiminh", "hcmc", "hcm", "tp hcm", "sai gon", "saigon", "thu duc", "cu chi", "tan binh", "nha be", "hiep phuoc"} {
		if strings.Contains(location, " "+city+" ") {
			return true
		}
	}
	return false
}
