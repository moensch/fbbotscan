package fbbotscan

import (
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	fb "github.com/huandu/facebook"
	"github.com/moensch/fbbotscan/config"
)

type FBApp struct {
	AppToken   string
	App        *fb.App
	Session    *fb.Session
	ConfigPath string
	Config     *config.Config
}

func New(configPath string) *FBApp {
	fbapp := &FBApp{
		ConfigPath: configPath,
	}
	if err := fbapp.Initialize(); err != nil {
		log.Fatalf("Failed to initialize: %s", err)
	}

	return fbapp
}

func (a *FBApp) LoadConfig() error {
	var err error
	a.Config, err = config.LoadFile(a.ConfigPath)
	return err
}

func (a *FBApp) Initialize() error {
	var err error
	if err := a.LoadConfig(); err != nil {
		return err
	}
	log.Infof("Application ID: %s", a.Config.FB.AppID)
	log.Infof("Application Secret: %s", a.Config.FB.AppSecret)
	a.App = fb.New(a.Config.FB.AppID, a.Config.FB.AppSecret)
	a.AppToken = a.App.AppAccessToken()
	log.Infof("Application Token: %s", a.AppToken)

	a.Session = a.App.Session(a.AppToken)
	a.Session.SetDebug(fb.DEBUG_ALL)

	return err
}

func (a *FBApp) LoadFeed(pageId string, maxEntries int, since int64) ([]FBPost, error) {
	var err error

	var posts = make([]FBPost, 0)

	log.Infof("Loading feed for %s since %d", pageId, since)
	res, err := a.Session.Get(fmt.Sprintf("/%s/feed", pageId), fb.Params{"limit": "4", "fields": "id,created_time,permalink_url,link,message,story", "since": since})
	if err != nil {
		return posts, err
	}

	p, err := res.Paging(a.Session)
	if err != nil {
		return posts, errors.New(fmt.Sprintf("Failed to call Paging: %s", err))
	}

	totalPosts := 0
	for ok := true; ok; ok = p.HasNext() {
		log.Debugf("  Loading next page for %s", pageId)
		for _, res := range p.Data() {
			var post FBPost
			err = res.Decode(&post)
			if err != nil {
				return posts, errors.New(fmt.Sprintf("Failed to decode post: %s", err))
			}
			posts = append(posts, post)
			totalPosts++
			if totalPosts >= maxEntries {
				return posts, err
			}
		}
		noMore, err := p.Next()
		if err != nil {
			return posts, errors.New(fmt.Sprintf("Error whilst calling Next() on paginaation: %s", err))
		}
		if noMore == true {
			return posts, err
		}
	}

	return posts, err
}

func (a *FBApp) LoadComments(objectId string, since int64) ([]FBComment, error) {
	var err error

	var comments = make([]FBComment, 0)

	log.Infof("Loading comments for %s since %d", objectId, since)
	res, err := a.Session.Get(fmt.Sprintf("/%s/comments", objectId), fb.Params{"limit": "20", "order": "chronological", "fields": "id,created_time,from,message,parent,comment_count,like_count,permalink_url", "since": since})
	if err != nil {
		return comments, err
	}

	p, err := res.Paging(a.Session)
	if err != nil {
		return comments, errors.New(fmt.Sprintf("Failed to call Paging: %s", err))
	}

	totalPosts := 0
	for ok := true; ok; ok = p.HasNext() {
		log.Debugf("  Loading next page for %s", objectId)
		for _, res := range p.Data() {
			var comment FBComment
			err = res.Decode(&comment)
			if err != nil {
				return comments, errors.New(fmt.Sprintf("Failed to decode comment: %s", err))
			}
			comments = append(comments, comment)
			totalPosts++
		}
		noMore, err := p.Next()
		if err != nil {
			return comments, errors.New(fmt.Sprintf("Error whilst calling Next() on paginaation: %s", err))
		}
		if noMore == true {
			return comments, err
		}
	}

	return comments, err
}
