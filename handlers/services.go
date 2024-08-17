package handlers

import (
	"net/http"

	"github.com/puregarlic/space/services"
	"github.com/puregarlic/space/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"go.hacdias.com/indielib/indieauth"
	"go.hacdias.com/indielib/micropub"
)

func AttachIndieAuth(r chi.Router, path string, profileUrl string) {
	svc := &services.IndieAuth{
		ProfileURL: profileUrl,
		Server:     indieauth.NewServer(true, nil),
	}

	r.Route(path, func(r chi.Router) {
		r.Post("/", svc.HandleAuthPOST)
		r.Post("/token", svc.HandleToken)
		r.Post("/accept", svc.HandleAuthApproval)

		// User authentication portal
		r.With(services.MustBasicAuth).Get("/", svc.HandleAuthGET)
	})

	storage.AddRel("authorization_endpoint", path)
	storage.AddRel("token_endpoint", path+"/token")
}

func AttachMicropub(r chi.Router, path string, profileURL string) {
	mp := &services.Micropub{
		ProfileURL: profileURL,
	}

	mpHandler := micropub.NewHandler(
		mp,
		micropub.WithMediaEndpoint(profileURL+"micropub/media"),
	)

	r.Route(path, func(r chi.Router) {
		// Enable CORS for browser-based clients
		r.Use(cors.Handler(
			cors.Options{
				AllowedHeaders: []string{
					"Accept",
					"Authorization",
					"Content-Type",
				},
			},
		))

		// Require access tokens for all Micropub routes
		r.Use(services.MustAuth)

		r.Get("/", mpHandler.ServeHTTP)
		r.Post("/", mpHandler.ServeHTTP)
		r.Post("/media", micropub.NewMediaHandler(
			mp.HandleMediaUpload,
			func(r *http.Request, scope string) bool {
				// IndieKit checks for a `media` scope, not commonly requested
				hasMediaScope := mp.HasScope(r, scope)
				hasCreateScope := mp.HasScope(r, "create")

				return hasMediaScope || hasCreateScope
			},
		).ServeHTTP)
	})

	storage.AddRel("micropub", path)
}
