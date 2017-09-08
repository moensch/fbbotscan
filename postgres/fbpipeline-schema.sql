BEGIN;
\set ON_ERROR_ROLLBACK

CREATE TABLE pages (
  "page_id" character varying(50) NOT NULL,
  "name" character varying(255) NOT NULL,
  "link" character varying(2048) NOT NULL,
  "created" timestamp with time zone DEFAULT ('now'::text)::timestamp(6) with time zone,
  "last_check" timestamp with time zone NULL,
  "scheduled" boolean NOT NULL default 'f'
);

CREATE UNIQUE INDEX pages_page_id_idx ON pages(page_id);

CREATE TABLE posts (
  "post_id" character varying(50) NOT NULL,
  "page_id" character varying(50) NOT NULL REFERENCES pages(page_id) ON DELETE CASCADE,
  "created" timestamp with time zone DEFAULT ('now'::text)::timestamp(6) with time zone,
  "last_check" timestamp with time zone NULL,
  "scheduled" boolean NOT NULL default 'f'
);

CREATE UNIQUE INDEX posts_post_id_idx ON posts(post_id);
CREATE INDEX posts_page_id_idx ON posts(page_id);

CREATE TABLE comments (
  "comment_id" character varying (50) NOT NULL,
  "post_id" character varying (50) NOT NULL REFERENCES posts(post_id) ON DELETE CASCADE,
  "parent_id" character varying (50) NULL,
  "user_id" character varying (50) NOT NULL,
  "created" timestamp with time zone DEFAULT ('now'::text)::timestamp(6) with time zone,
  "last_check" timestamp with time zone NULL,
  "scheduled" boolean NOT NULL default 'f'
);


CREATE UNIQUE INDEX comments_comment_id_idx ON comments(comment_id);
CREATE INDEX comments_parent_id_idx ON comments(parent_id);
CREATE INDEX comments_post_id_idx ON comments(post_id);
ALTER TABLE comments ADD FOREIGN KEY (parent_id) REFERENCES comments(comment_id) ON DELETE CASCADE;

END;
