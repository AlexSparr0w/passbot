CREATE USER bot123;
GRANT ALL PRIVILEGES ON DATABASE passbot TO bot123;
ALTER USER bot123 WITH PASSWORD 'password';
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE passwords (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL,
  resource TEXT NOT NULL,
  login TEXT NOT NULL,
  password BYTEA NOT NULL
);

INSERT INTO passwords (user_id, resource, password)
VALUES (1, 'example.com', pgp_sym_encrypt('password123', 'my_secret_key'));

SELECT user_id, resource, pgp_sym_decrypt(password, 'my_secret_key') AS password
FROM passwords;	