package db

import (
	"database/sql"
	"fmt"
	log "github.com/Sirupsen/logrus"
	_ "github.com/lib/pq"
	"github.com/moensch/fbbotscan/config"
	"strings"
)

type DB struct {
	Conn   *sql.DB
	Config *config.DBConfig
}

func New(cfg *config.DBConfig) *DB {
	db := &DB{
		Config: cfg,
	}

	if err := db.Initialize(); err != nil {
		log.Fatalf("Failed to initialize: %s", err)
	}

	return db
}

func (db *DB) Initialize() error {
	log.Debugf("DB type: %s", db.Config.Type)
	log.Debugf("DB Connection: %s", db.Config.Connstr)
	return nil
}

func (db *DB) Connect() error {
	log.Info("Connecting to Database")
	var err error
	db.Conn, err = sql.Open(db.Config.Type, db.Config.Connstr)
	if err != nil {
		return fmt.Errorf("Cannot connect to database: %s", err)
	}

	log.Info("Successfully connected to database server")

	return err
}

// Get all post and comments which haven't been checked for new comments
//  in over delaySecs seconds.
//  Exclude comments which have a parent as sub-comments cannot
//    have more comments
func (db *DB) GetSchedulerComments(delaySecs int) (*sql.Rows, error) {
	check_query := `SELECT id, objtype, last_check FROM (
				SELECT concat(page_id, '_', post_id) AS id, cast(EXTRACT(EPOCH FROM last_check) as integer) AS last_check, 'post' AS objtype FROM posts
					WHERE scheduled = 'f'
				UNION
				SELECT concat(post_id, '_', comment_id) AS id, cast(EXTRACT(EPOCH FROM last_check) as integer) AS last_check, 'comment' AS objtype FROM comments
					WHERE (parent_id IS NULL OR parent_id = '')
					AND scheduled = 'f'
			) sub1
			WHERE last_check < extract(EPOCH FROM NOW()) - %d
			OR last_check IS NULL`

	log.Debugf("Query: %s", fmt.Sprintf(check_query, delaySecs))
	rows, err := db.Conn.Query(fmt.Sprintf(check_query, delaySecs))
	if err != nil {
		return nil, fmt.Errorf("Database query failed: %s", err)
	}

	return rows, err
}

// Get all the pages which haven't been checked for new posts
//  in over delaySecs seconds (or never)
func (db *DB) GetSchedulerPosts(delaySecs int) (*sql.Rows, error) {
	check_query := `SELECT id, objtype, last_check FROM (
				SELECT page_id AS id, cast(EXTRACT(EPOCH FROM last_check) as integer) AS last_check, 'page' as objtype FROM pages
					WHERE scheduled = 'f'
			) sub1
			WHERE last_check < extract(EPOCH FROM NOW()) - %d
			OR last_check IS NULL`

	log.Debugf("Query: %s", fmt.Sprintf(check_query, delaySecs))
	rows, err := db.Conn.Query(fmt.Sprintf(check_query, delaySecs))
	if err != nil {
		return nil, fmt.Errorf("Database query failed: %s", err)
	}

	return rows, err
}

func (db *DB) UpdateLastCheck(entryType string, id string, last_check int64) error {
	if entryType != "post" && entryType != "page" && entryType != "comment" {
		return fmt.Errorf("Invalid type '%s' - must be one of 'post', 'page', or 'comment'", entryType)
	}

	var real_id string
	if strings.Contains(id, "_") {
		id_parts := strings.Split(id, "_")
		real_id = id_parts[1]
	} else {
		real_id = id
	}

	query := fmt.Sprintf("UPDATE %ss SET last_check = to_timestamp(%d), scheduled = $2 WHERE %s_id = $1",
		entryType,
		last_check,
		entryType,
	)

	log.Debugf("Running update: %s", query)

	var something int
	err := db.Conn.QueryRow(query, real_id, false).Scan(&something)

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("Cannot update database: %s", err)
	}

	return nil
}

func (db *DB) SetScheduled(entryType string, id string) error {
	if entryType != "post" && entryType != "page" && entryType != "comment" {
		return fmt.Errorf("Invalid type '%s' - must be one of 'post', 'page', or 'comment'", entryType)
	}

	var real_id string
	if strings.Contains(id, "_") {
		id_parts := strings.Split(id, "_")
		real_id = id_parts[1]
	} else {
		real_id = id
	}

	query := fmt.Sprintf("UPDATE %ss SET scheduled = $1 WHERE %s_id = '%s'",
		entryType,
		entryType,
		real_id,
	)

	log.Debugf("Running update: %s", query)

	var something int
	err := db.Conn.QueryRow(query, true).Scan(&something)

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("Cannot update database: %s", err)
	}

	return nil
}

func (db *DB) InsertComment(comment_id string, post_id, parent_id string, user_id string) error {
	log.Debugf("Storing new comment: %s / %s / %s / %s", comment_id, post_id, parent_id, user_id)

	var real_parent_id sql.NullString
	if parent_id != "" {
		parent_id_parts := strings.Split(parent_id, "_")
		real_parent_id.String = parent_id_parts[1]
		real_parent_id.Valid = true
	}

	query := `INSERT INTO comments (comment_id, post_id, parent_id, user_id)
			VALUES
			($1, $2, $3, $4)`

	var ignore int
	err := db.Conn.QueryRow(query, comment_id, post_id, real_parent_id, user_id).Scan(&ignore)

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("Cannot insert comment (%s, %s, %s, %s: %s", comment_id, post_id, real_parent_id.String, user_id, err)
	}

	return nil
}

func (db *DB) InsertPost(id string) error {
	log.Debugf("Storing new post: %s", id)
	id_parts := strings.Split(id, "_")

	query := `INSERT INTO posts (post_id, page_id)
			VALUES
			($1, $2)`

	var ignore int
	err := db.Conn.QueryRow(query, id_parts[1], id_parts[0]).Scan(&ignore)

	switch {
	case err == sql.ErrNoRows:
		return nil
	case err == nil:
		return nil
	case strings.Contains(err.Error(), "duplicate key"):
		log.Warnf("Warning - post %s already in database", id)
		return nil
	case err != nil:
		return fmt.Errorf("Cannot insert post: %s", err)
	}

	return nil
}
