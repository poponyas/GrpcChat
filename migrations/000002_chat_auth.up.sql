ALTER TABLE chat.users ADD COLUMN password_hash TEXT;
CREATE UNIQUE INDEX users_login_name ON chat.users (lower(name)) WHERE password_hash IS NOT NULL;

CREATE TABLE chat.sessions (
    token_hash TEXT PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES chat.users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX sessions_expiry ON chat.sessions(expires_at);

ALTER TABLE chat.chat_members ADD COLUMN role TEXT NOT NULL DEFAULT 'participant'
    CHECK (role IN ('owner', 'admin', 'participant'));
UPDATE chat.chat_members m SET role = 'owner'
WHERE m.user_id::text = (SELECT min(m2.user_id::text) FROM chat.chat_members m2 WHERE m2.chat_id = m.chat_id)
AND NOT (SELECT is_private FROM chat.chats WHERE id = m.chat_id);

ALTER TABLE chat.chats ADD COLUMN direct_key TEXT UNIQUE;
CREATE INDEX messages_history_cursor ON chat.messages(chat_id, created_at DESC, id DESC);
