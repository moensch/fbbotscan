package fbbotscan

type FBComment struct {
	CreatedTime string `json:"created_time"`
	From        FBUser `json:"from"`
	ID          string `json:"id"`
	Message     string `json:"message"`
	Parent      struct {
		CreatedTime string `json:"created_time"`
		From        FBUser `json:"from"`
		ID          string `json:"id"`
		Message     string `json:"message"`
	} `json:"parent"`
	PermalinkURL string `json:"permalink_url"`
	CommentCount int32  `json:"comment_count"`
	LikeCount    int32  `json:"like_count"`
}

type FBCommentList struct {
	Entries []FBComment `json:"data"`
}

const CommentMapping = `
{
    "settings": {
        "number_of_replicas": 0,
        "number_of_shards": 1
    },
    "mappings": {
        "fbcomment": {
            "properties": {
                "comment_count": {
                    "type": "long"
                },
                "created_time": {
                    "type": "date"
                },
                "from": {
                    "properties": {
                        "first_name": {
                            "type": "text"
                        },
                        "id": {
                            "type": "text"
                        },
                        "last_name": {
                            "type": "text"
                        },
                        "is_verified": {
                            "type": "boolean"
                        },
                        "name": {
                            "type": "text"
                        },
                        "name_format": {
                            "type": "text"
                        },
                        "short_name": {
                            "type": "text"
                        }
                    }
                },
                "id": {
                    "store": true,
                    "type": "keyword"
                },
                "like_count": {
                    "type": "long"
                },
                "message": {
                    "fielddata": true,
                    "store": true,
                    "type": "text",
                    "term_vector": "yes"
                },
                "parent": {
                    "properties": {
                        "id": {
                            "type": "keyword"
                        }
                    }
                },
                "permalink_url": {
                    "fielddata": true,
                    "store": true,
                    "type": "text"
                }
            }
        }
    }
}`
