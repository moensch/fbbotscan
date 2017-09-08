package fbbotscan

type QueueEntry struct {
	ObjectID    string `json:"object_id"`
	LastChecked int64  `json:"last_checked"`
	ObjectType  string `json:"type"`
}
