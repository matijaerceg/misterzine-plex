package plex

import (
	"encoding/xml"
	"testing"
)

func TestSortTitleMetadata(t *testing.T) {
	var x xmlItem
	if err := xml.Unmarshal([]byte(`<Video title="The 'Burbs" titleSort="'Burbs"/>`), &x); err != nil {
		t.Fatal(err)
	}
	it := x.item()
	if it.Title != "The 'Burbs" || it.SortLabel() != "'Burbs" {
		t.Fatal("sort title lost or display title changed")
	}
	it.SortTitle = ""
	if it.SortLabel() != it.Title {
		t.Fatal("missing sort title must fall back to display title")
	}
}
