package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/puregarlic/space/pages"

	"github.com/a-h/templ"
	"github.com/aidarkhanov/nanoid"
	"github.com/golang-jwt/jwt/v5"
	"go.hacdias.com/indielib/indieauth"
)

// storeAuthorization stores the authorization request and returns a code for it.
// Something such as JWT tokens could be used in a production environment.
func (s *server) storeAuthorization(req *indieauth.AuthenticationRequest) string {
	code := nanoid.New()

	s.db.Authorization.Set(code, req, 0)

	return code
}

type CustomTokenClaims struct {
	Scopes []string `json:"scopes"`
	jwt.RegisteredClaims
}

type contextKey string

const (
	scopesContextKey contextKey = "scopes"
)

// authorizationGetHandler handles the GET method for the authorization endpoint.
func (s *server) authorizationGetHandler(w http.ResponseWriter, r *http.Request) {
	// In a production server, this page would usually be protected. In order for
	// the user to authorize this request, they must be authenticated. This could
	// be done in different ways: username/password, passkeys, etc.

	// Parse the authorization request.
	req, err := s.ias.ParseAuthorization(r)
	if err != nil {
		serveErrorJSON(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	// Do a best effort attempt at fetching more information about the application
	// that we can show to the user. Not all applications provide this sort of
	// information.
	app, _ := s.ias.DiscoverApplicationMetadata(r.Context(), req.ClientID)

	// Here, we just display a small HTML document where the user has to press
	// to authorize this request. Please note that this template contains a form
	// where we dump all the request information. This makes it possible to reuse
	// [indieauth.Server.ParseAuthorization] when the user authorizes the request.
	templ.Handler(pages.Auth(req, app)).ServeHTTP(w, r)
}

// authorizationPostHandler handles the POST method for the authorization endpoint.
func (s *server) authorizationPostHandler(w http.ResponseWriter, r *http.Request) {
	s.authorizationCodeExchange(w, r, false)
}

// tokenHandler handles the token endpoint. In our case, we only accept the default
// type which is exchanging an authorization code for a token.
func (s *server) tokenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpError(w, http.StatusMethodNotAllowed)
		return
	}

	if r.Form.Get("grant_type") == "refresh_token" {
		// NOTE: this server does not implement refresh tokens.
		// https://indieauth.spec.indieweb.org/#refresh-tokens
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	s.authorizationCodeExchange(w, r, true)
}

type tokenResponse struct {
	Me          string `json:"me"`
	AccessToken string `json:"access_token,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	Scope       string `json:"scope,omitempty"`
	ExpiresIn   int64  `json:"expires_in,omitempty"`
}

// authorizationCodeExchange handles the authorization code exchange. It is used by
// both the authorization handler to exchange the code for the user's profile URL,
// and by the token endpoint, to exchange the code by a token.
func (s *server) authorizationCodeExchange(w http.ResponseWriter, r *http.Request, withToken bool) {
	if err := r.ParseForm(); err != nil {
		serveErrorJSON(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	// t := s.getAuthorization(r.Form.Get("code"))
	req, present := s.db.Authorization.GetAndDelete(r.Form.Get("code"))
	if !present {
		serveErrorJSON(w, http.StatusBadRequest, "invalid_request", "invalid authorization")
		return
	}
	authRequest := req.Value()

	err := s.ias.ValidateTokenExchange(authRequest, r)
	if err != nil {
		serveErrorJSON(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	response := &tokenResponse{
		Me: s.profileURL,
	}

	scopes := authRequest.Scopes

	if withToken {
		now := time.Now()
		expiresAt := now.Add(15 * time.Minute)
		claims := CustomTokenClaims{
			scopes,
			jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(expiresAt),
				IssuedAt:  jwt.NewNumericDate(now),
				NotBefore: jwt.NewNumericDate(now),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

		secret := os.Getenv("JWT_SECRET")
		jwt, err := token.SignedString([]byte(secret))
		if err != nil {
			panic(err)
		}

		response.AccessToken = jwt
		response.TokenType = "Bearer"
		response.ExpiresIn = int64(time.Until(expiresAt).Seconds())
		response.Scope = strings.Join(scopes, " ")
	}

	// An actual server may want to include the "profile" in the response if the
	// scope "profile" is included.
	serveJSON(w, http.StatusOK, response)
}

func (s *server) authorizationAcceptHandler(w http.ResponseWriter, r *http.Request) {
	// Parse authorization information. This only works because our authorization page
	// includes all the required information. This can be done in other ways: database,
	// whether temporary or not, cookies, etc.
	req, err := s.ias.ParseAuthorization(r)
	if err != nil {
		serveErrorJSON(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	// Generate a random code and persist the information associated to that code.
	// You could do this in other ways: database, or JWT tokens, or both, for example.
	code := s.storeAuthorization(req)

	// Redirect to client callback.
	query := url.Values{}
	query.Set("code", code)
	query.Set("state", req.State)
	http.Redirect(w, r, req.RedirectURI+"?"+query.Encode(), http.StatusFound)
}

// mustAuth is a middleware to ensure that the request is authorized. The way this
// works depends on the implementation. It then stores the scopes in the context.
func (s *server) mustAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.Header.Get("Authorization")
		tokenStr = strings.TrimPrefix(tokenStr, "Bearer")
		tokenStr = strings.TrimSpace(tokenStr)

		if len(tokenStr) <= 0 {
			serveErrorJSON(w, http.StatusUnauthorized, "invalid_request", "no credentials")
			return
		}

		token, err := jwt.ParseWithClaims(tokenStr, &CustomTokenClaims{}, func(t *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil {
			serveErrorJSON(w, http.StatusUnauthorized, "invalid_request", "invalid token")
			return
		} else if claims, ok := token.Claims.(*CustomTokenClaims); ok {
			ctx := context.WithValue(r.Context(), scopesContextKey, claims.Scopes)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		} else {
			serveErrorJSON(w, http.StatusUnauthorized, "invalid_request", "malformed claims")
			return
		}
	})
}

func (s *server) mustBasicAuth(next http.Handler) http.Handler {
	user, ok := os.LookupEnv("ADMIN_USERNAME")
	if !ok {
		panic(errors.New("ADMIN_USERNAME is not set, cannot start"))
	}

	pass, ok := os.LookupEnv("ADMIN_PASSWORD")
	if !ok {
		panic(errors.New("ADMIN_PASSWORD is not set, cannot start"))
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok {
			usernameHash := sha256.Sum256([]byte(username))
			passwordHash := sha256.Sum256([]byte(password))
			expectedUsernameHash := sha256.Sum256([]byte(user))
			expectedPasswordHash := sha256.Sum256([]byte(pass))

			usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1)
			passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], expectedPasswordHash[:]) == 1)

			if usernameMatch && passwordMatch {
				next.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}
