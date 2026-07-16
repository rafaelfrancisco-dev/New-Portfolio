package views

import "go-port/internal/db"

func formatPostDate(post db.Post) string {
	if !post.PublishedAt.Valid {
		return ""
	}
	return post.PublishedAt.Time.Format("2006-01-02")
}
