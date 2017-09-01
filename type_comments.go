package fbbotscan

type FBComments struct {
	Comments FBCommentList `json:"data"`
	Paging   FBPaging      `json:"paging"`
}
