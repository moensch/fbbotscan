package fbbotscan

type FBFeed struct {
	Posts  FBPostList `json:"data"`
	Paging FBPaging   `json:"paging"`
}
