package vfs

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/net/html"
	"google.golang.org/api/iterator"

	"context"
)

// Filesystem is an interface that abstracts file system operations.
type Filesystem interface {
	ReadFile(path string) ([]byte, error)
	ReadDir(path string) ([]os.DirEntry, error)
	Stat(path string) (os.FileInfo, error)
	Join(elem ...string) string
}

type HttpFS struct {
	baseURL string
	client  *http.Client
}

func NewHttpFS(baseURL string) *HttpFS {
	return &HttpFS{
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (h *HttpFS) url(filePath string) (string, error) {
	u, err := url.Parse(h.baseURL)
	if err != nil {
		return "", err
	}
	u.Path = path.Join(u.Path, filePath)
	return u.String(), nil
}

func (h *HttpFS) ReadFile(filePath string) ([]byte, error) {
	url, err := h.url(filePath)
	if err != nil {
		return nil, err
	}
	resp, err := h.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}
	return ioutil.ReadAll(resp.Body)
}

func (h *HttpFS) ReadDir(dirPath string) ([]os.DirEntry, error) {
	url, err := h.url(dirPath)
	if err != nil {
		return nil, err
	}
	resp, err := h.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	tokenizer := html.NewTokenizer(resp.Body)
	var entries []os.DirEntry
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			break
		}
		if tokenType == html.StartTagToken {
			token := tokenizer.Token()
			if token.Data == "a" {
				for _, attr := range token.Attr {
					if attr.Key == "href" {
						// Simple parsing, assumes href is a file or directory name
						// and skips parent directory links.
						if !strings.HasPrefix(attr.Val, ".") && !strings.HasPrefix(attr.Val, "/") {
							entries = append(entries, &httpDirEntry{name: attr.Val, isDir: strings.HasSuffix(attr.Val, "/")})
						}
					}
				}
			}
		}
	}
	return entries, nil
}

type httpDirEntry struct {
	name  string
	isDir bool
}

func (e *httpDirEntry) Name() string {
	return e.name
}

func (e *httpDirEntry) IsDir() bool {
	return e.isDir
}

func (e *httpDirEntry) Type() os.FileMode {
	if e.isDir {
		return os.ModeDir
	}
	return 0
}

func (e *httpDirEntry) Info() (os.FileInfo, error) {
	return &httpFileInfo{name: e.name, isDir: e.isDir}, nil
}

type httpFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (i *httpFileInfo) Name() string {
	return i.name
}
func (i *httpFileInfo) Size() int64 {
	return i.size
}
func (i *httpFileInfo) Mode() os.FileMode {
	if i.isDir {
		return os.ModeDir
	}
	return 0
}
func (i *httpFileInfo) ModTime() time.Time {
	return time.Time{} // Not available from basic HTTP
}
func (i *httpFileInfo) IsDir() bool {
	return i.isDir
}
func (i *httpFileInfo) Sys() interface{} {
	return nil
}

func (h *HttpFS) Stat(filePath string) (os.FileInfo, error) {
	url, err := h.url(filePath)
	if err != nil {
		return nil, err
	}
	resp, err := h.client.Head(url)
	if err != nil {
		// Try a GET, maybe the server doesn't support HEAD
		getResp, getErr := h.client.Get(url)
		if getErr != nil {
			return nil, err // return original HEAD error
		}
		defer getResp.Body.Close()
		if getResp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("bad status: %s", getResp.Status)
		}
		contentType := getResp.Header.Get("Content-Type")
		isDir := strings.Contains(contentType, "text/html")
		return &httpFileInfo{name: filepath.Base(filePath), size: getResp.ContentLength, isDir: isDir}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	contentType := resp.Header.Get("Content-Type")
	isDir := strings.Contains(contentType, "text/html")

	return &httpFileInfo{name: filepath.Base(filePath), size: resp.ContentLength, isDir: isDir}, nil
}

func (h *HttpFS) Join(elem ...string) string {
	return path.Join(elem...)
}

type GcsFS struct {
	bucket       *storage.BucketHandle
	objectPrefix string
}

func NewGcsFS(ctx context.Context, baseURL string) (*GcsFS, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	trimmedBaseURL := strings.TrimPrefix(baseURL, "gs://")
	bucketName := strings.Split(trimmedBaseURL, "/")[0]
	objectPrefix := strings.TrimPrefix(trimmedBaseURL, bucketName)
	objectPrefix = strings.TrimPrefix(objectPrefix, "/")

	return &GcsFS{
		bucket:       client.Bucket(bucketName),
		objectPrefix: objectPrefix,
	}, nil
}

func (g *GcsFS) getObjectPath(p string) string {
	return path.Join(g.objectPrefix, p)
}

func (g *GcsFS) ReadFile(p string) ([]byte, error) {
	ctx := context.Background()
	objPath := g.getObjectPath(p)
	rc, err := g.bucket.Object(objPath).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return ioutil.ReadAll(rc)
}

func (g *GcsFS) ReadDir(p string) ([]os.DirEntry, error) {
	ctx := context.Background()
	objPath := g.getObjectPath(p)
	if objPath != "" && !strings.HasSuffix(objPath, "/") {
		objPath += "/"
	}

	it := g.bucket.Objects(ctx, &storage.Query{
		Prefix:    objPath,
		Delimiter: "/",
	})

	var entries []os.DirEntry
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var entryName string
		var isDir bool

		if attrs.Prefix != "" { // It's a directory
			isDir = true
			entryName = strings.TrimPrefix(attrs.Prefix, objPath)
			entryName = strings.TrimSuffix(entryName, "/")
		} else { // It's a file
			isDir = false
			entryName = strings.TrimPrefix(attrs.Name, objPath)
		}

		if entryName == "" {
			continue
		}

		entries = append(entries, &gcsDirEntry{
			name:  entryName,
			isDir: isDir,
			attrs: attrs,
		})
	}
	return entries, nil
}

func (g *GcsFS) Stat(p string) (os.FileInfo, error) {
	ctx := context.Background()
	objPath := g.getObjectPath(p)
	attrs, err := g.bucket.Object(objPath).Attrs(ctx)
	if err != nil {
		// It might be a directory-like prefix
		it := g.bucket.Objects(ctx, &storage.Query{Prefix: objPath, Delimiter: "/"})
		_, err := it.Next()
		if err == iterator.Done {
			return nil, os.ErrNotExist
		}
		if err != nil {
			return nil, err
		}
		return &gcsFileInfo{
			name:  path.Base(p),
			isDir: true,
		}, nil
	}
	return &gcsFileInfo{
		name:  path.Base(attrs.Name),
		size:  attrs.Size,
		isDir: false,
		mod:   attrs.Updated,
	}, nil
}

func (g *GcsFS) Join(elem ...string) string {
	return path.Join(elem...)
}

type gcsDirEntry struct {
	name  string
	isDir bool
	attrs *storage.ObjectAttrs
}

func (e *gcsDirEntry) Name() string {
	return e.name
}

func (e *gcsDirEntry) IsDir() bool {
	return e.isDir
}

func (e *gcsDirEntry) Type() os.FileMode {
	if e.isDir {
		return os.ModeDir
	}
	return 0
}

func (e *gcsDirEntry) Info() (os.FileInfo, error) {
	return &gcsFileInfo{
		name:  e.name,
		isDir: e.isDir,
		size:  e.attrs.Size,
		mod:   e.attrs.Updated,
	}, nil
}

type gcsFileInfo struct {
	name  string
	size  int64
	isDir bool
	mod   time.Time
}

func (i *gcsFileInfo) Name() string {
	return i.name
}
func (i *gcsFileInfo) Size() int64 {
	return i.size
}
func (i *gcsFileInfo) Mode() os.FileMode {
	if i.isDir {
		return os.ModeDir
	}
	return 0
}
func (i *gcsFileInfo) ModTime() time.Time {
	return i.mod
}
func (i *gcsFileInfo) IsDir() bool {
	return i.isDir
}
func (i *gcsFileInfo) Sys() interface{} {
	return nil
}

type LocalFS struct{}

func (l *LocalFS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (l *LocalFS) ReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

func (l *LocalFS) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (l *LocalFS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

// OS is the default filesystem that uses the local disk.
var OS Filesystem = &LocalFS{}
