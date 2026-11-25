-- name: DeleteArticle :execrows
delete from articles
where article_id = $1 and version = sqlc.arg(expected_version);
