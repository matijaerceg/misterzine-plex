package plex

// SortLabel is the server's ordering title, with the display title as fallback.
func (it *Item) SortLabel() string {
	if it.SortTitle != "" {
		return it.SortTitle
	}
	return it.Title
}
