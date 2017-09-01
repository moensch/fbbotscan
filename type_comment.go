package fbbotscan

type FBComment struct {
	CreatedTime string     `json:"created_time"`
	From        FBUser     `json:"from"`
	ID          string     `json:"id"`
	Message     string     `json:"message"`
	Parent      *FBComment `json:"parent"`
}

type FBCommentList struct {
	Entries []FBComment `json:"data"`
}
