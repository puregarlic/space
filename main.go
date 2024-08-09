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
	"github.com/puregarlic/space/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"

	"go.hacdias.com/indielib/indieauth"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	port, profileURL := validateRuntimeConfiguration()
	defer storage.CleanupAuthCache()

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(httprate.LimitByIP(100, 1*time.Minute))

	// CORS be enabled for browser-based agents to fetch `rel` elements.
	// We'll enable it just on the root route since it should be used as the profile URL
	r.With(cors.AllowAll().Handler).Get("/", handlers.ServeHomePage)

	// Content pages
	r.Get("/posts/{slug}", handlers.ServePostPage)

	// Static asset handlers
	r.Get("/media/*", handlers.ServeMedia)
	r.Get("/static/*", http.StripPrefix(
		"/static",
		http.FileServer(http.Dir("static")),
	).ServeHTTP)

	// Service handlers
	handlers.AttachIndieAuth(r, "/authorization", profileURL)
	handlers.AttachMicropub(r, "/micropub", profileURL)

	// Start it!
	log.Printf("Listening on http://localhost:%d", port)
	log.Printf("Listening on %s", profileURL)
	if err := http.ListenAndServe(":"+strconv.Itoa(port), r); err != nil {
		log.Fatal(err)
	}
}

func validateRuntimeConfiguration() (portNumber int, profileURL string) {
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

	return port, profileURL
}
