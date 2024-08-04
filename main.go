package main

import (
	"encoding/json"
	"flag"
	"time"

	"log"
	"net/http"
	"strconv"

	"github.com/ostafen/clover/v2/query"

	"github.com/puregarlic/space/db"
	"github.com/puregarlic/space/pages"
	"github.com/puregarlic/space/types"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"

	"go.hacdias.com/indielib/indieauth"
	"go.hacdias.com/indielib/microformats"
	"go.hacdias.com/indielib/micropub"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	// Setup flags.
	portPtr := flag.Int("port", 80, "port to listen on")
	addressPtr := flag.String("profile", "http://localhost/", "client URL and front facing address to listen on")
	flag.Parse()

	profileURL := *addressPtr

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
		r.Use(s.mustAuth)
		r.Get("/", s.serveMicropub)
		r.Post("/", s.serveMicropub)
	})

	// Start it!
	log.Printf("Listening on http://localhost:%d", *portPtr)
	log.Printf("Listening on %s", profileURL)
	if err := http.ListenAndServe(":"+strconv.Itoa(*portPtr), r); err != nil {
		log.Fatal(err)
	}
}

type server struct {
	profileURL string
	ias        *indieauth.Server
	db         *db.Storage
}

func (s *server) serveHomeTemplate(w http.ResponseWriter, r *http.Request) {
	q := query.NewQuery(
		string(db.PostCollection),
	).Sort(query.SortOption{
		Field:     "createdAt",
		Direction: -1,
	}).Limit(10)

	docs, err := s.db.Docs.FindAll(q)
	if err != nil {
		httpError(w, http.StatusInternalServerError)
		panic(err)
	}

	posts := make([]*types.Post, len(docs))
	for i, doc := range docs {
		id := doc.ObjectId()
		post := &types.Post{
			ID: id,
		}

		if err := doc.Unmarshal(post); err != nil {
			httpError(w, http.StatusInternalServerError)
			panic(err)
		}

		post.ID = id

		posts[i] = post
	}

	templ.Handler(pages.Home(s.profileURL, posts)).ServeHTTP(w, r)
}

func (s *server) servePostTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "slug")
	post := &types.Post{}

	doc, err := s.db.Docs.FindById(string(db.PostCollection), id)
	if err != nil {
		httpError(w, http.StatusInternalServerError)
		return
	} else if doc == nil {
		httpError(w, http.StatusNotFound)
		return
	}

	if err := doc.Unmarshal(post); err != nil {
		httpError(w, http.StatusInternalServerError)
		return
	}

	templ.Handler(pages.Post(post)).ServeHTTP(w, r)
}

func (s *server) serveMicropub(w http.ResponseWriter, r *http.Request) {
	micropub.NewHandler(
		&micropubImplementation{s},
		micropub.WithGetPostTypes(func() []micropub.PostType {
			return []micropub.PostType{
				{
					Name: "Post",
					Type: string(microformats.TypeNote),
				},
			}
		}),
	).ServeHTTP(w, r)
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
