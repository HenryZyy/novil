package natsort

import (
	"slices"
	"testing"
)

func TestCompare(t *testing.T) {
	got := []string{"page10.jpg", "Page2.jpg", "page1.jpg", "page01b.jpg", "cover.jpg", "page002.jpg"}
	slices.SortFunc(got, Compare)
	want := []string{"cover.jpg", "page1.jpg", "page01b.jpg", "Page2.jpg", "page002.jpg", "page10.jpg"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
