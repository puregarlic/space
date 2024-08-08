package handlers

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/puregarlic/space/models"
	"github.com/puregarlic/space/pages"
	"github.com/puregarlic/space/storage"
)

func ServeHomePage(w http.ResponseWriter, r *http.Request) {
	posts := make([]*models.Post, 0)
	result := storage.GORM().Limit(10).Find(&posts)
	if result.Error != nil {
		panic(result.Error)
	}

	templ.Handler(pages.Home(posts)).ServeHTTP(w, r)
}

func ServePostPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "slug")
	post := &models.Post{}

	result := storage.GORM().First(post, "id = ?", id)

	if result.RowsAffected == 0 {
		SendHttpError(w, http.StatusNotFound)
		return
	}

	templ.Handler(pages.Post(post)).ServeHTTP(w, r)
}
