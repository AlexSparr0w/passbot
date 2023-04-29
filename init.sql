CREATE DATABASE passbot;
CREATE USER bot;
GRANT ALL PRIVILEGES ON DATABASE passbot TO bot;
ALTER USER bot WITH PASSWORD 'password';
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE passwords (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL,
  resource TEXT NOT NULL,
  login TEXT NOT NULL,
  password BYTEA NOT NULL
);