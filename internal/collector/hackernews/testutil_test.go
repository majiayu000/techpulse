package hackernews

// parseItemIDFromPath parses item ID from URL path like /item/123.json
func parseItemIDFromPath(path string) int {
	var id int
	prefix := "/item/"
	if len(path) > len(prefix) {
		for i := len(prefix); i < len(path); i++ {
			c := path[i]
			if c >= '0' && c <= '9' {
				id = id*10 + int(c-'0')
			} else {
				break
			}
		}
	}
	return id
}
