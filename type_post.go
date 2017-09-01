package fbbotscan

type FBPost struct {
	CreatedTime  string `json:"created_time"`
	ID           string `json:"id"`
	Link         string `json:"link"`
	Message      string `json:"message"`
	Story        string `json:"story"`
	PermalinkURL string `json:"permalink_url"`
}

type FBPostList struct {
	Entries []FBPost `json:"data"`
}
