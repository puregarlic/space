package main

import (
	"errors"
	"fmt"
	"net/http"
	urlpkg "net/url"
	"reflect"
	"strings"
	"time"

	"github.com/ostafen/clover/v2/document"
	"github.com/puregarlic/space/db"
	"github.com/puregarlic/space/types"
	"github.com/samber/lo"

	"go.hacdias.com/indielib/micropub"
)

type micropubImplementation struct {
	*server
}

func postIdFromUrlPath(path string) string {
	return strings.TrimPrefix(path, "/posts/")
}

func (s *micropubImplementation) HasScope(r *http.Request, scope string) bool {
	v := r.Context().Value(scopesContextKey)
	if scopes, ok := v.([]string); ok {
		for _, sc := range scopes {
			if sc == scope {
				return true
			}
		}
	}

	return false
}

func (s *micropubImplementation) Source(urlStr string) (map[string]any, error) {
	url, err := urlpkg.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}

	id := postIdFromUrlPath(url.Path)
	post := &types.Post{}
	doc, err := s.server.db.Docs.FindById(string(db.PostCollection), id)
	if err != nil {
		panic(err)
	} else if doc == nil {
		return nil, micropub.ErrNotFound
	}

	if err := doc.Unmarshal(post); err != nil {
		panic(err)
	}

	return map[string]any{
		"type":       []string{post.Type},
		"properties": post.Properties,
	}, nil
}

func (s *micropubImplementation) SourceMany(limit, offset int) ([]map[string]any, error) {
	return nil, micropub.ErrNotImplemented
}

func (s *micropubImplementation) Create(req *micropub.Request) (string, error) {
	post := types.Post{
		Type:       req.Type,
		Properties: req.Properties,
		CreatedAt:  time.Now().Unix(),
	}
	doc := document.NewDocumentOf(post)
	if doc == nil {
		return "", errors.New("Could not marshal post to Clover document")
	}

	id, err := s.server.db.Docs.InsertOne(string(db.PostCollection), doc)
	if err != nil {
		return "", err
	}

	return s.profileURL + "posts/" + id, nil
}

func (s *micropubImplementation) Update(req *micropub.Request) (string, error) {
	url, err := urlpkg.Parse(req.URL)
	if err != nil {
		return "", fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}

	id := postIdFromUrlPath(url.Path)

	if err := s.server.db.Docs.UpdateById(
		string(db.PostCollection),
		id,
		func(doc *document.Document) *document.Document {
			post := &types.Post{}
			if err := doc.Unmarshal(post); err != nil {
				panic(err)
			}

			props, err := updateProperties(post.Properties, req)
			if err != nil {
				panic(err)
			}

			doc.Set("properties", props)

			return doc
		},
	); err != nil {
		return "", fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}

	return s.profileURL + url.Path, nil
}

func (s *micropubImplementation) Delete(urlStr string) error {
	url, err := urlpkg.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}

	id := postIdFromUrlPath(url.Path)

	if err := s.server.db.Docs.DeleteById(string(db.PostCollection), id); err != nil {
		return fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}

	return nil
}

func (s *micropubImplementation) Undelete(url string) error {
	return micropub.ErrNotImplemented
}

// updateProperties applies the updates (additions, deletions, replacements)
// in the given [micropub.Request] to a set of existing microformats properties.
func updateProperties(properties map[string][]any, req *micropub.Request) (map[string][]any, error) {
	if req.Updates.Replace != nil {
		for key, value := range req.Updates.Replace {
			properties[key] = value
		}
	}

	if req.Updates.Add != nil {
		for key, value := range req.Updates.Add {
			switch key {
			case "name":
				return nil, errors.New("cannot add a new name")
			case "content":
				return nil, errors.New("cannot add content")
			default:
				if key == "published" {
					if _, ok := properties["published"]; ok {
						return nil, errors.New("cannot replace published through add method")
					}
				}

				if _, ok := properties[key]; !ok {
					properties[key] = []any{}
				}

				properties[key] = append(properties[key], value...)
			}
		}
	}

	if req.Updates.Delete != nil {
		if reflect.TypeOf(req.Updates.Delete).Kind() == reflect.Slice {
			toDelete, ok := req.Updates.Delete.([]any)
			if !ok {
				return nil, errors.New("invalid delete array")
			}

			for _, key := range toDelete {
				delete(properties, fmt.Sprint(key))
			}
		} else {
			toDelete, ok := req.Updates.Delete.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid delete object: expected map[string]any, got: %s", reflect.TypeOf(req.Updates.Delete))
			}

			for key, v := range toDelete {
				value, ok := v.([]any)
				if !ok {
					return nil, fmt.Errorf("invalid value: expected []any, got: %s", reflect.TypeOf(value))
				}

				if _, ok := properties[key]; !ok {
					properties[key] = []any{}
				}

				properties[key] = lo.Filter(properties[key], func(ss any, _ int) bool {
					for _, s := range value {
						if s == ss {
							return false
						}
					}
					return true
				})
			}
		}
	}

	return properties, nil
}
