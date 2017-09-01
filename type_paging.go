package fbbotscan

type FBPaging struct {
	Cursors struct {
		After  string `json:"after"`
		Before string `json:"before"`
	} `json:"cursors"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
}
