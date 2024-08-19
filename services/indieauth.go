package services

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/puregarlic/space/html/layouts"
	"github.com/puregarlic/space/html/pages"
	"github.com/puregarlic/space/storage"

	"github.com/aidarkhanov/nanoid"
	"github.com/golang-jwt/jwt/v5"
	"go.hacdias.com/indielib/indieauth"
)

type IndieAuth struct {
	ProfileURL string
	Server     *indieauth.Server
}

func (i *IndieAuth) storeAuthorization(req *indieauth.AuthenticationRequest) string {
	code := nanoid.New()

	storage.AuthCache().Set(code, req, 0)

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

func (i *IndieAuth) HandleAuthGET(w http.ResponseWriter, r *http.Request) {
	req, err := i.Server.ParseAuthorization(r)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	app, _ := i.Server.DiscoverApplicationMetadata(r.Context(), req.ClientID)

	nonceId, nonce := nanoid.New(), nanoid.New()
	storage.NonceCache().Set(nonceId, nonce, 0)

	layouts.RenderDefault("authorize", pages.Auth(req, app, nonceId, nonce)).ServeHTTP(w, r)
}

func (i *IndieAuth) HandleAuthPOST(w http.ResponseWriter, r *http.Request) {
	i.authorizationCodeExchange(w, r, false)
}

func (i *IndieAuth) HandleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if r.Form.Get("grant_type") == "refresh_token" {
		// NOTE: this server does not implement refresh tokens.
		// https://indieauth.spec.indieweb.org/#refresh-tokens
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	i.authorizationCodeExchange(w, r, true)
}

type tokenResponse struct {
	Me          string `json:"me"`
	AccessToken string `json:"access_token,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	Scope       string `json:"scope,omitempty"`
	ExpiresIn   int64  `json:"expires_in,omitempty"`
}

func (i *IndieAuth) authorizationCodeExchange(w http.ResponseWriter, r *http.Request, withToken bool) {
	if err := r.ParseForm(); err != nil {
		SendErrorJSON(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	// t := s.getAuthorization(r.Form.Get("code"))
	req, present := storage.AuthCache().GetAndDelete(r.Form.Get("code"))
	if !present {
		SendErrorJSON(w, http.StatusBadRequest, "invalid_request", "invalid authorization")
		return
	}
	authRequest := req.Value()

	err := i.Server.ValidateTokenExchange(authRequest, r)
	if err != nil {
		SendErrorJSON(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	response := &tokenResponse{
		Me: i.ProfileURL,
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
	SendJSON(w, http.StatusOK, response)
}

func (i *IndieAuth) HandleAuthApproval(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("nonce_id")
	nonce := r.FormValue("nonce")

	stored, ok := storage.NonceCache().GetAndDelete(id)
	if !ok {
		SendErrorJSON(w, http.StatusBadRequest, "bad_request", "nonce does not match")
	} else if stored.Value() != nonce {
		SendErrorJSON(w, http.StatusBadRequest, "bad_request", "nonce does not match")
	}

	req, err := i.Server.ParseAuthorization(r)
	if err != nil {
		SendErrorJSON(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	code := i.storeAuthorization(req)

	// Redirect to client callback.
	query := url.Values{}
	query.Set("code", code)
	query.Set("state", req.State)
	http.Redirect(w, r, req.RedirectURI+"?"+query.Encode(), http.StatusFound)
}

func MustAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.Header.Get("Authorization")
		tokenStr = strings.TrimPrefix(tokenStr, "Bearer")
		tokenStr = strings.TrimSpace(tokenStr)

		if len(tokenStr) <= 0 {
			SendErrorJSON(w, http.StatusUnauthorized, "invalid_request", "no credentials")
			return
		}

		token, err := jwt.ParseWithClaims(tokenStr, &CustomTokenClaims{}, func(t *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil {
			SendErrorJSON(w, http.StatusUnauthorized, "invalid_request", "invalid token")
			return
		} else if claims, ok := token.Claims.(*CustomTokenClaims); ok {
			ctx := context.WithValue(r.Context(), scopesContextKey, claims.Scopes)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		} else {
			SendErrorJSON(w, http.StatusUnauthorized, "invalid_request", "malformed claims")
			return
		}
	})
}

func MustBasicAuth(next http.Handler) http.Handler {
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

func SendJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}

func SendErrorJSON(w http.ResponseWriter, code int, err, errDescription string) {
	SendJSON(w, code, map[string]string{
		"error":             err,
		"error_description": errDescription,
	})
}
