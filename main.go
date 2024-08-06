package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"log"
	"net/http"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/puregarlic/space/db"
	"github.com/puregarlic/space/models"
	"github.com/puregarlic/space/pages"

	"github.com/a-h/templ"
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

	// Setup storage handlers
	store := db.NewStorage()
	defer store.Cleanup()

	// Create a new client.
	s := &server{
		profileURL: profileURL,
		ias:        indieauth.NewServer(true, nil),
		db:         store,
	}

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(httprate.LimitByIP(100, 1*time.Minute))

	// Static resources
	r.Get("/static/*", http.StripPrefix("/static", http.FileServer(http.Dir("static"))).ServeHTTP)

	// Pages
	r.Get("/", s.serveHomeTemplate)
	r.Get("/posts/{slug}", s.servePostTemplate)
	r.Get("/media/*", s.serveMedia)

	// IndieAuth handlers
	r.Group(func(r chi.Router) {
		r.Post("/token", s.tokenHandler)
		r.Post("/authorization", s.authorizationPostHandler)
		r.Post("/authorization/accept", s.authorizationAcceptHandler)

		// User authentication portal
		r.With(s.mustBasicAuth).Get("/authorization", s.authorizationGetHandler)
	})

	// Micropub handler
	r.Route("/micropub", func(r chi.Router) {
		// Enable CORS for browser-based clients
		r.Use(cors.Handler(
			cors.Options{
				AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
			},
		))
		r.Use(s.mustAuth)

		mp := &micropubImplementation{s}
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
			micropub.WithMediaEndpoint(s.profileURL+"micropub/media"),
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

type server struct {
	profileURL string
	ias        *indieauth.Server
	db         *db.Storage
}

func (s *server) serveHomeTemplate(w http.ResponseWriter, r *http.Request) {
	posts := make([]*models.Post, 0)
	result := s.db.Db.Limit(10).Find(&posts)
	if result.Error != nil {
		panic(result.Error)
	}

	templ.Handler(pages.Home(s.profileURL, posts)).ServeHTTP(w, r)
}

func (s *server) servePostTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "slug")
	post := &models.Post{}

	result := s.db.Db.First(post, "id = ?", id)

	if result.RowsAffected == 0 {
		httpError(w, http.StatusNotFound)
		return
	}

	templ.Handler(pages.Post(post)).ServeHTTP(w, r)
}

func (s *server) serveMedia(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/")

	res, err := s.db.Media.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(os.Getenv("AWS_S3_BUCKET_NAME")),
		Key:    &key,
	})
	if err != nil {
		fmt.Println("failed to get object", err)
		httpError(w, http.StatusInternalServerError)
		return
	}

	defer res.Body.Close()

	w.Header().Set("Cache-Control", "604800")

	if _, err := io.Copy(w, res.Body); err != nil {
		fmt.Println("failed to send object", err)
		httpError(w, http.StatusInternalServerError)
		return
	}
}

func httpError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

func serveJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}

func serveErrorJSON(w http.ResponseWriter, code int, err, errDescription string) {
	serveJSON(w, code, map[string]string{
		"error":             err,
		"error_description": errDescription,
	})
}
