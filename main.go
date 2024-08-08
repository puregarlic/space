package main

//go:generate templ generate
//go:generate deno run --allow-all npm:tailwindcss -o static/styles.css -c config/tailwind.config.js --minify

import (
	"os"
	"time"

	"log"
	"net/http"
	"strconv"

	"github.com/puregarlic/space/handlers"
	"github.com/puregarlic/space/services"
	"github.com/puregarlic/space/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"

	"go.hacdias.com/indielib/indieauth"
	"go.hacdias.com/indielib/microformats"
	"go.hacdias.com/indielib/micropub"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	var port int
	if portStr, ok := os.LookupEnv("PORT"); !ok {
		port = 80
	} else {
		portInt, err := strconv.Atoi(portStr)
		if err != nil {
			log.Fatal(err)
		}

		port = portInt
	}

	profileURL, ok := os.LookupEnv("PROFILE_URL")
	if !ok {
		profileURL = "http://localhost/"
	}

	// Validate the given Client ID before starting the HTTP server.
	err := indieauth.IsValidProfileURL(profileURL)
	if err != nil {
		log.Fatal(err)
	}

	defer storage.CleanupAuthCache()

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(httprate.LimitByIP(100, 1*time.Minute))

	// Static resources
	r.Get("/static/*", http.StripPrefix("/static", http.FileServer(http.Dir("static"))).ServeHTTP)

	// Pages
	r.Get("/", handlers.ServeHomePage)
	r.Get("/posts/{slug}", handlers.ServePostPage)
	r.Get("/media/*", handlers.ServeMedia)

	// IndieAuth handlers
	r.Group(func(r chi.Router) {
		ias := &services.IndieAuth{
			ProfileURL: profileURL,
			Server:     indieauth.NewServer(true, nil),
		}

		r.Post("/token", ias.HandleToken)
		r.Post("/authorization", ias.HandleAuthPOST)
		r.Post("/authorization/accept", ias.HandleAuthApproval)

		// User authentication portal
		r.With(services.MustBasicAuth).Get("/authorization", ias.HandleAuthGET)
	})

	// Micropub handler
	r.Route("/micropub", func(r chi.Router) {
		// Enable CORS for browser-based clients
		r.Use(cors.Handler(
			cors.Options{
				AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
			},
		))
		r.Use(services.MustAuth)

		mp := &services.Micropub{
			ProfileURL: profileURL,
		}
		mpHandler := micropub.NewHandler(
			mp,
			micropub.WithGetPostTypes(func() []micropub.PostType {
				return []micropub.PostType{
					{
						Name: "Post",
						Type: string(microformats.TypeNote),
					},
					{
						Name: "Photo",
						Type: string(microformats.TypePhoto),
					},
				}
			}),
			micropub.WithMediaEndpoint(profileURL+"micropub/media"),
		)

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

	// Start it!
	log.Printf("Listening on http://localhost:%d", port)
	log.Printf("Listening on %s", profileURL)
	if err := http.ListenAndServe(":"+strconv.Itoa(port), r); err != nil {
		log.Fatal(err)
	}
}
