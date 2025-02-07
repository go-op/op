CREATE TABLE IF NOT EXISTS users (
    username TEXT PRIMARY KEY NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    full_name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    encrypted_password TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users_recipes_favorites (
    username TEXT NOT NULL,
    recipe_id TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (username, recipe_id),
    FOREIGN KEY (username) REFERENCES users (id) ON DELETE CASCADE,
    FOREIGN KEY (recipe_id) REFERENCES recipes (id) ON DELETE CASCADE
);
