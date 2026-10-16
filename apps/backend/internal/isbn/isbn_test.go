package isbn

import (
	"slices"
	"testing"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"978-0-06-294150-3": "9780062941503",
		"9780062941503":     "9780062941503",
		"0-441-01359-7":     "9780441013593",
		"080442957X":        "9780804429573",
		" 080442957x ":      "9780804429573",
	} {
		if got, err := Normalize(in); err != nil || got != want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "9780062941504", "0441013598", "12345", "978006294150X", "1234567890123", "abcdefghij"} {
		if got, err := Normalize(in); err == nil {
			t.Errorf("Normalize(%q) = %q, want an error", in, got)
		}
	}
}

func TestAddAuthor(t *testing.T) {
	var names []string
	for _, n := range []string{"Kate Elizabeth Russell", "Grace Gummer", "Russell  Kate Elizab", "kate russell", "Kate  Morton"} {
		names = addAuthor(names, n)
	}
	if want := []string{"Kate Elizabeth Russell", "Grace Gummer", "Kate Morton"}; !slices.Equal(names, want) {
		t.Errorf("got %q, want %q", names, want)
	}
}
