package vfs

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"k8s.io/klog/v2"

	"context"
)

// Filesystem is an interface that abstracts file system operations.
type Filesystem interface {
	ReadFile(path string) ([]byte, error)
	ReadDir(path string) ([]os.DirEntry, error)
	Stat(path string) (os.FileInfo, error)
	Join(elem ...string) string
}

type GcsFS struct {
	bucket       *storage.BucketHandle
	bucketName   string
	objectPrefix string
}

func NewGcsFS(baseURL string) (*GcsFS, error) {
	ctx := context.Background()

	klog.V(5).Infof("GcsFS: NewGcsFS with baseURL %s", baseURL)
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
		bucketName:   bucketName,
		objectPrefix: objectPrefix,
	}, nil
}

func (g *GcsFS) getObjectPath(p string) string {

	// Handle absolute GCS URLs eg. like "gs://bucket-name/whatever/whatnot"
	// and "gs:/bucket-name/whatever/whatnot"
	if strings.HasPrefix(p, "gs://") || strings.HasPrefix(p, "gs:/") {

		var trimmed = ""
		if strings.HasPrefix(p, "gs://") {
			trimmed = strings.TrimPrefix(p, "gs://")
		} else if strings.HasPrefix(p, "gs:/") {
			trimmed = strings.TrimPrefix(p, "gs:/")
		}

		parts := strings.SplitN(trimmed, "/", 2)
		if len(parts) < 2 {
			return ""
		}
		bucketName := parts[0]
		objectPath := parts[1]

		// if it is not the same bucket, exit
		if bucketName != g.bucketName {
			klog.Warningf("GcsFS: Accessing different bucket %s (expected %s)", bucketName, g.bucketName)
			// we should not proceed here! Likely we have a bug that we're going
			// beyond the bucket.
			os.Exit(1)
		}

		return objectPath
	}

	// Handle relative paths - join with objectPrefix
	// should covers eg. like "./whatever/foo", "whatever/foo/another"
	relativePath := strings.TrimPrefix(p, "./")
	if g.objectPrefix == "" {
		return relativePath
	}
	return path.Join(g.objectPrefix, relativePath)
}

func (g *GcsFS) ReadFile(p string) ([]byte, error) {
	klog.V(5).Infof("GcsFS: ReadFile %s", p)
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
	klog.V(5).Infof("GcsFS: ReadDir %s", p)

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
	klog.V(5).Infof("GcsFS: Stat %s", p)
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
	klog.V(5).Infof("GcsFS: Join %v", elem)
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
	klog.V(5).Infof("LocalFS: ReadFile %s", path)
	return os.ReadFile(path)
}

func (l *LocalFS) ReadDir(path string) ([]os.DirEntry, error) {
	klog.V(5).Infof("LocalFS: ReadDir %s", path)
	return os.ReadDir(path)
}

func (l *LocalFS) Stat(path string) (os.FileInfo, error) {
	klog.V(5).Infof("LocalFS: Stat %s", path)
	return os.Stat(path)
}

func (l *LocalFS) Join(elem ...string) string {
	klog.V(5).Infof("LocalFS: Join %v", elem)
	return filepath.Join(elem...)
}

// CurrentFS is the default filesystem that uses the local disk.
// It is switched to GCS when config detects.
var CurrentFS Filesystem = &LocalFS{}
