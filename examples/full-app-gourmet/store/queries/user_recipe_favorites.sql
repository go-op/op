-- name: GetFavoritesByUser :many
SELECT sqlc.embed(recipe) FROM users_recipes_favorites
JOIN recipe ON users_recipes_favorites.recipe_id = recipe.id
WHERE username = ?;

-- name: AddFavorite :one
INSERT INTO users_recipes_favorites (username, recipe_id) VALUES (?, ?) RETURNING *;

-- name: RemoveFavorite :exec
DELETE FROM users_recipes_favorites WHERE username = ? AND recipe_id = ?;
