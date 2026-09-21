CREATE SCHEMA IF NOT EXISTS chat;

CREATE TABLE IF NOT EXISTS chat.users (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    fullname VARCHAR(255) NOT NULL,
    phone VARCHAR(50),                       
    avatar_url TEXT,                         
    photo_posted_at TIMESTAMPTZ,             
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS chat.chats (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL DEFAULT '',
    is_private BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS chat.chat_members (
    chat_id UUID NOT NULL REFERENCES chat.chats(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES chat.users(id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (chat_id, user_id)
);

CREATE TABLE IF NOT EXISTS chat.messages (
    id UUID PRIMARY KEY,
    chat_id UUID NOT NULL REFERENCES chat.chats(id) ON DELETE CASCADE,
    sender_id UUID REFERENCES chat.users(id) ON DELETE SET NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_chat_members_user_id 
ON chat.chat_members(user_id);

CREATE INDEX IF NOT EXISTS idx_messages_chat_id_created_at 
ON chat.messages(chat_id, created_at DESC);