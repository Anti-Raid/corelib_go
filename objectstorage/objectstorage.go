package objectstorage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/Anti-Raid/corelib_go/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/signer"
)

// A simple abstraction for object storage
type ObjectStorage struct {
	c *config.ObjectStorageConfig

	// If s3-like
	minio *minio.Client
}

func New(c *config.ObjectStorageConfig) (o *ObjectStorage, err error) {
	o = &ObjectStorage{
		c: c,
	}

	switch c.Type {
	case "s3-like":
		o.minio, err = minio.New(c.Endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(c.AccessKey, c.SecretKey, ""),
			Secure: c.Secure,
			Region: "us-east-1",
		})

		if err != nil {
			return nil, err
		}
	case "local":
		err = os.MkdirAll(c.Path, 0755)

		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("invalid object storage type")
	}

	return o, nil
}

func (o *ObjectStorage) manuallyCreateCdnPresignedUrlNoNetRequest(path string, expiry time.Duration) *url.URL {
	// This is a workaround for the fact that presigning a URL requires a network request
	// This is not ideal, but it works for now
	// It is guaranteed to always succeed
	expirySecs := int64(expiry.Seconds())

	req := signer.PreSignV4(
		http.Request{
			Method: http.MethodGet,
			URL: &url.URL{
				Scheme: func() string {
					if o.c.CdnSecure {
						return "https"
					}
					return "http"
				}(),
				Host:    o.c.CdnEndpoint,
				Path:    path,
				RawPath: path,
			},
		},
		o.c.AccessKey,
		o.c.SecretKey,
		"",
		"us-east-1",
		expirySecs,
	)

	return req.URL
}

func (o *ObjectStorage) createBucketIfNotExists(ctx context.Context) error {
	if o.c.Type != "s3-like" {
		return nil
	}

	exists, err := o.minio.BucketExists(ctx, o.c.Path)

	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	err = o.minio.MakeBucket(ctx, o.c.Path, minio.MakeBucketOptions{})

	if err != nil {
		return fmt.Errorf("failed to create bucket: %v", err)
	}

	return nil
}

// Saves a file to the object storage
//
// Note that 'expiry' is not supported for local storage
func (o *ObjectStorage) Save(ctx context.Context, dir, filename string, data *bytes.Buffer, expiry time.Duration) error {
	switch o.c.Type {
	case "local":
		err := os.MkdirAll(filepath.Join(o.c.Path, dir), 0755)

		if err != nil {
			return err
		}

		f, err := os.Create(filepath.Join(o.c.Path, dir, filename))

		if err != nil {
			return err
		}

		_, err = io.Copy(f, data)

		if err != nil {
			return err
		}

		return nil
	case "s3-like":
		err := o.createBucketIfNotExists(ctx)
		if err != nil {
			return err
		}

		p := minio.PutObjectOptions{}

		if expiry != 0 {
			p.Expires = time.Now().Add(expiry)
		}
		_, err = o.minio.PutObject(ctx, o.c.Path, dir+"/"+filename, data, int64(data.Len()), p)

		if err != nil {
			return err
		}

		return nil
	default:
		return fmt.Errorf("operation not supported for object storage type %s", o.c.Type)
	}
}

// Returns the url to the file
func (o *ObjectStorage) GetUrl(ctx context.Context, dir, filename string, urlExpiry time.Duration) (*url.URL, error) {
	switch o.c.Type {
	case "local":
		var path string

		if filename == "" {
			path = filepath.Join(o.c.Path, dir)
		} else {
			path = filepath.Join(o.c.Path, dir, filename)
		}

		return &url.URL{
			Scheme: "file",
			Path:   path,
		}, nil
	case "s3-like":
		var path string

		if filename == "" {
			path = dir
		} else {
			path = dir + "/" + filename
		}

		p := o.manuallyCreateCdnPresignedUrlNoNetRequest(path, urlExpiry)

		return p, nil
	default:
		return nil, fmt.Errorf("operation not supported for object storage type %s", o.c.Type)
	}
}

// Deletes a file
func (o *ObjectStorage) Delete(ctx context.Context, dir, filename string) error {
	switch o.c.Type {
	case "local":
		return os.Remove(filepath.Join(o.c.Path, dir, filename))
	case "s3-like":
		return o.minio.RemoveObject(ctx, o.c.Path, dir+"/"+filename, minio.RemoveObjectOptions{})
	default:
		return fmt.Errorf("operation not supported for object storage type %s", o.c.Type)
	}
}
