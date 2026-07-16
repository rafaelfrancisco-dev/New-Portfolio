package views

import (
	"fmt"

	"go-port/internal/db"
)

type AdminPostForm struct {
	ID            int64
	Title         string
	Slug          string
	Category      string
	Teaser        string
	Lede          string
	BodyMD        string
	StoreURL      string
	Status        string
	PublishedDate string
	UpdatedAt     string
	Error         string
}

func NewAdminPostForm(post db.Post) AdminPostForm {
	form := AdminPostForm{
		ID:        post.ID,
		Title:     post.Title,
		Slug:      post.Slug,
		Category:  post.Category,
		Teaser:    post.Teaser,
		Lede:      post.Lede,
		BodyMD:    post.BodyMd,
		StoreURL:  post.StoreUrl,
		Status:    post.Status,
		UpdatedAt: post.UpdatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"),
	}
	if post.PublishedAt.Valid {
		form.PublishedDate = post.PublishedAt.Time.Format("2006-01-02")
	}
	return form
}

func adminPostAction(form AdminPostForm) string {
	if form.ID == 0 {
		return "/admin/posts"
	}
	return fmt.Sprintf("/admin/posts/%d", form.ID)
}

func adminPostHeading(form AdminPostForm) string {
	if form.ID == 0 {
		return "New post"
	}
	return "Edit post"
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	if size < unit*unit {
		return fmt.Sprintf("%.1f KiB", float64(size)/unit)
	}
	return fmt.Sprintf("%.1f MiB", float64(size)/(unit*unit))
}
