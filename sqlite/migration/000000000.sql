-- initial migration
CREATE TABLE users (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT NOT NULL,
	email      TEXT UNIQUE,
	api_key    TEXT NOT NULL UNIQUE,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE auths (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id       INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	source        TEXT NOT NULL,
	source_id     TEXT NOT NULL,
	access_token  TEXT NOT NULL,
	refresh_token TEXT NOT NULL,
	expiry        TEXT,
	created_at    TEXT NOT NULL,
	updated_at    TEXT NOT NULL,

	UNIQUE(user_id, source),  -- one source per user
	UNIQUE(source, source_id) -- one auth per source user
);

CREATE TABLE shorts (
    key TEXT PRIMARY KEY,
    url TEXT NOT NULL,

	owner_id	INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,

    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
