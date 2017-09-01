package fbbotscan

type FBUser struct {
	FirstName  string `json:"first_name"`
	ID         string `json:"id"`
	IsVerified bool   `json:"is_verified"`
	LastName   string `json:"last_name"`
	Name       string `json:"name"`
	NameFormat string `json:"name_format"`
	ShortName  string `json:"short_name"`
}

type FBUserList struct {
	Entries []FBUser `json:"data"`
}
