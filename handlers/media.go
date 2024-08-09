package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/puregarlic/space/storage"
)

func ServeMedia(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/")

	res, err := storage.S3().GetObject(r.Context(), &s3.GetObjectInput{
		Bucket: aws.String(os.Getenv("AWS_S3_BUCKET_NAME")),
		Key:    &key,
	})
	if err != nil {
		fmt.Println("failed to get object", err)
		panic(err)
	}

	defer res.Body.Close()

	w.Header().Set("Cache-Control", "604800")

	if _, err := io.Copy(w, res.Body); err != nil {
		fmt.Println("failed to send object", err)
		panic(err)
	}
}
