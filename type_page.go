package fbbotscan

type FBPage struct {
	ID   string `json:"id"`
	Link string `json:"link"`
	Name string `json:"name"`
}

type FBPageList struct {
	Entries []FBPage `json:"data"`
}
