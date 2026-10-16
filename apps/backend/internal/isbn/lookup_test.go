package isbn

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// Calls the real services, so only with NOVIL_ONLINE_TESTS=1.
func TestOnline(t *testing.T) {
	if os.Getenv("NOVIL_ONLINE_TESTS") == "" {
		t.Skip("set NOVIL_ONLINE_TESTS=1 to look up real ISBNs")
	}
	info, err := Online(os.Getenv("NOVIL_GOOGLE_BOOKS_KEY"))(context.Background(), "9780062941503")
	if err != nil || !strings.EqualFold(info.Title, "My Dark Vanessa") || info.Authors == "" {
		t.Errorf("My Dark Vanessa: %+v, %v", info, err)
	}
	t.Logf("%+v", info)
	if _, err := Online("")(context.Background(), "9790000000001"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown ISBN: %v", err)
	}
}
