-- +goose Up
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    username TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL DEFAULT '',
    avatar_url TEXT,
    role TEXT NOT NULL DEFAULT 'user',
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_username_unique ON users(username) WHERE username <> '';
CREATE UNIQUE INDEX users_email_unique ON users(email) WHERE email <> '';

CREATE TABLE titles (
    id TEXT PRIMARY KEY,
    primary_title TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL DEFAULT '',
    start_year INT NOT NULL DEFAULT 0,
    rating_aggregate DOUBLE PRECISION NOT NULL DEFAULT 0,
    vote_count INT NOT NULL DEFAULT 0,
    added_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    metadata JSONB NOT NULL
);
CREATE INDEX titles_primary_title_idx ON titles(primary_title);
CREATE INDEX titles_type_idx ON titles(type);
CREATE INDEX titles_rating_idx ON titles(rating_aggregate);

CREATE TABLE ratings (
    id TEXT PRIMARY KEY,
    title_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    note REAL NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, title_id)
);
CREATE TABLE rating_seasons (
    rating_id TEXT NOT NULL REFERENCES ratings(id) ON DELETE CASCADE,
    season TEXT NOT NULL,
    rating REAL NOT NULL DEFAULT 0,
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(rating_id, season)
);

CREATE TABLE comments (
    id TEXT PRIMARY KEY,
    title_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE comment_seasons (
    comment_id TEXT NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
    season TEXT NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(comment_id, season)
);

CREATE TABLE groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    owner_id TEXT NOT NULL,
    deleted BOOLEAN NOT NULL DEFAULT false,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX groups_owner_name_unique ON groups(owner_id, name) WHERE NOT deleted;

CREATE TABLE group_members (
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    PRIMARY KEY(group_id, user_id)
);
CREATE TABLE group_titles (
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    title_id TEXT NOT NULL,
    title_type TEXT NOT NULL DEFAULT '',
    watched BOOLEAN NOT NULL DEFAULT false,
    watched_at TIMESTAMPTZ,
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(group_id, title_id)
);
CREATE TABLE group_title_seasons (
    group_id TEXT NOT NULL,
    title_id TEXT NOT NULL,
    season TEXT NOT NULL,
    watched BOOLEAN NOT NULL DEFAULT false,
    watched_at TIMESTAMPTZ,
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(group_id, title_id, season),
    FOREIGN KEY(group_id, title_id) REFERENCES group_titles(group_id, title_id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE group_title_seasons;
DROP TABLE group_titles;
DROP TABLE group_members;
DROP TABLE groups;
DROP TABLE comment_seasons;
DROP TABLE comments;
DROP TABLE rating_seasons;
DROP TABLE ratings;
DROP TABLE titles;
DROP TABLE users;
