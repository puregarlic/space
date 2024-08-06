package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	urlpkg "net/url"
	"os"
	"reflect"
	"strings"

	"github.com/puregarlic/space/models"

	"github.com/aidarkhanov/nanoid"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/h2non/filetype"
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
	post := &models.Post{}

	res := s.server.db.Db.Find(post, "id = ?", id)
	if res.Error != nil {
		panic(res.Error)
	} else if res.RowsAffected == 0 {
		return nil, micropub.ErrNotFound
	}

	return map[string]any{
		"type":       []string{post.Type},
		"properties": post.Properties,
	}, nil
}

func (s *micropubImplementation) SourceMany(limit, offset int) ([]map[string]any, error) {
	return nil, micropub.ErrNotImplemented
}

func (s *micropubImplementation) HandleMediaUpload(file multipart.File, header *multipart.FileHeader) (string, error) {
	defer file.Close()

	kind, err := filetype.MatchReader(file)
	if _, err := file.Seek(0, 0); err != nil {
		return "", fmt.Errorf("%w: %w", errors.New("failed to reset cursor"), err)
	}

	if err != nil {
		return "", fmt.Errorf("%w: %w", errors.New("failed to upload"), err)
	}

	key := fmt.Sprintf("media/%s.%s", nanoid.New(), kind.Extension)
	_, err = s.db.Media.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(os.Getenv("AWS_S3_BUCKET_NAME")),
		Key:    &key,
		Body:   file,
	})
	if err != nil {
		return "", fmt.Errorf("%w: %w", errors.New("failed to upload"), err)
	}

	return s.profileURL + key, nil
}

func (s *micropubImplementation) Create(req *micropub.Request) (string, error) {
	props, err := json.Marshal(req.Properties)
	if err != nil {
		return "", err
	}

	post := &models.Post{
		ID:         models.NewULID(),
		Type:       req.Type,
		Properties: props,
	}

	res := s.server.db.Db.Create(post)
	if res.Error != nil {
		return "", res.Error
	}

	return s.profileURL + "posts/" + post.ID.String(), nil
}

func (s *micropubImplementation) Update(req *micropub.Request) (string, error) {
	url, err := urlpkg.Parse(req.URL)
	if err != nil {
		return "", fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}

	id := postIdFromUrlPath(url.Path)
	post := &models.Post{}

	res := s.server.db.Db.Find(post, "id = ?", id)
	if res.Error != nil {
		panic(res.Error)
	} else if res.RowsAffected != 1 {
		return "", micropub.ErrNotFound
	}

	newProps, err := updateProperties(json.RawMessage(post.Properties), req)
	if err != nil {
		panic(err)
	}

	post.Properties = newProps

	s.server.db.Db.Save(post)

	return s.profileURL + url.Path, nil
}

func (s *micropubImplementation) Delete(urlStr string) error {
	url, err := urlpkg.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}

	id := postIdFromUrlPath(url.Path)

	res := s.server.db.Db.Delete(&models.Post{}, "id = ?", id)
	if res.Error != nil {
		panic(res.Error)
	} else if res.RowsAffected == 0 {
		return fmt.Errorf("%w: %w", micropub.ErrNotFound, err)
	}

	return nil
}

func (s *micropubImplementation) Undelete(urlStr string) error {
	url, err := urlpkg.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}

	id := postIdFromUrlPath(url.Path)
	res := s.server.db.Db.Unscoped().Model(&models.Post{}).Where("id = ?", id).Update("deleted_at", nil)
	if res.Error != nil {
		return res.Error
	} else if res.RowsAffected != 1 {
		return micropub.ErrNotFound
	}

	return nil
}

// updateProperties applies the updates (additions, deletions, replacements)
// in the given [micropub.Request] to a set of existing microformats properties.
func updateProperties(props json.RawMessage, req *micropub.Request) ([]byte, error) {
	properties := make(map[string][]any)
	if err := json.Unmarshal(props, &properties); err != nil {
		panic(err)
	}

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

	propJson, err := json.Marshal(&properties)
	if err != nil {
		panic(err)
	}

	return propJson, nil
}
