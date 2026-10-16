// Package isbn checks ISBNs and looks up the books they name.
package isbn

import (
	"errors"
	"strings"
)

var ErrInvalid = errors.New("not a valid ISBN")

// Normalize returns the ISBN-13 of an ISBN-10 or ISBN-13, written with or
// without hyphens and spaces. Typos almost always break the check digit,
// so they are ErrInvalid.
func Normalize(s string) (string, error) {
	s = strings.NewReplacer("-", "", " ", "").Replace(strings.ToUpper(strings.TrimSpace(s)))
	switch len(s) {
	case 10:
		sum := 0
		for i, c := range s {
			d := int(c - '0')
			if c == 'X' && i == 9 {
				d = 10
			} else if c < '0' || c > '9' {
				return "", ErrInvalid
			}
			sum += d * (10 - i)
		}
		if sum%11 != 0 {
			return "", ErrInvalid
		}
		return withCheckDigit("978" + s[:9]), nil
	case 13:
		if strings.Trim(s, "0123456789") != "" || (!strings.HasPrefix(s, "978") && !strings.HasPrefix(s, "979")) {
			return "", ErrInvalid
		}
		if withCheckDigit(s[:12]) != s {
			return "", ErrInvalid
		}
		return s, nil
	}
	return "", ErrInvalid
}

// withCheckDigit appends the ISBN-13 check digit to its first 12 digits.
func withCheckDigit(first12 string) string {
	sum := 0
	for i, c := range first12 {
		d := int(c - '0')
		if i%2 == 1 {
			d *= 3
		}
		sum += d
	}
	return first12 + string(rune('0'+(10-sum%10)%10))
}
