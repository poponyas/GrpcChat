DROP INDEX chat.messages_history_cursor;
ALTER TABLE chat.chats DROP COLUMN direct_key;
ALTER TABLE chat.chat_members DROP COLUMN role;
DROP TABLE chat.sessions;
DROP INDEX chat.users_login_name;
ALTER TABLE chat.users DROP COLUMN password_hash;
