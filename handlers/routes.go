package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/puregarlic/space/models"
	"github.com/puregarlic/space/storage"

	"github.com/puregarlic/space/html/layouts"
	"github.com/puregarlic/space/html/pages"
)

func ServeHomePage(w http.ResponseWriter, r *http.Request) {
	posts := make([]*models.Post, 0)
	result := storage.GORM().Limit(10).Order("created_at DESC").Find(&posts)
	if result.Error != nil {
		panic(result.Error)
	}

	layouts.RenderDefault("", pages.Home(posts)).ServeHTTP(w, r)
}

func ServePostPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "slug")
	post := &models.Post{}

	result := storage.GORM().First(post, "id = ?", id)

	if result.RowsAffected == 0 {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	layouts.RenderDefault(string(post.MicroformatType), pages.Post(post)).ServeHTTP(w, r)
}
