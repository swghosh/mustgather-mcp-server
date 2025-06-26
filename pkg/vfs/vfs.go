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

	"golang.org/x/net/html"
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
