package use

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	pathlib "path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gmeghnag/omc/pkg/vfs"
	"github.com/spf13/cobra"
	"github.com/ulikunitz/xz"
)

const (
	fileTypeTar     string = "tar"
	fileTypeTarGzip string = "tar.gz"
	fileTypeXZ      string = "tar.xz"
	fileTypeZip     string = "zip"
)

func humanizeBytes(bytes int64) string {
	var human string
	if float64(bytes) < math.Pow(2, 10) {
		human = fmt.Sprintf("%.0f B", float64(bytes))
	} else if float64(bytes) < math.Pow(2, 20) {
		human = fmt.Sprintf("%.1f K", float64(bytes)/math.Pow(2, 10))
	} else {
		human = fmt.Sprintf("%.1f M", float64(bytes)/math.Pow(2, 20))
	}
	return human
}

type WriteCounter struct {
	length     string
	downloaded int64
	lastShown  time.Time
	cmd        *cobra.Command
}

func NewWriteCounter(cmd *cobra.Command, total int64) *WriteCounter {
	length := ""
	if total != -1 {
		length = humanizeBytes(total)
	} else {
		length = "?"
	}
	counter := &WriteCounter{
		length:     length,
		downloaded: 0,
		lastShown:  time.Now(),
		cmd:        cmd,
	}
	return counter
}

func (counter *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	counter.downloaded += int64(n)
	counter.ShowProgress()
	return n, nil
}

func (counter *WriteCounter) Downloaded() string {
	return humanizeBytes(counter.downloaded)
}

func (counter *WriteCounter) ShowProgress() {
	// rate limit
	throttleDuration, _ := time.ParseDuration("100ms")
	if time.Since(counter.lastShown).Nanoseconds() < throttleDuration.Nanoseconds() {
		return
	}

	fmt.Fprintf(counter.cmd.OutOrStdout(), "\r%s", strings.Repeat(" ", 78))
	fmt.Fprintf(counter.cmd.OutOrStdout(), "\rDownloading... %s / %s", counter.Downloaded(), counter.length)

	counter.lastShown = time.Now()
}

func GetHeaderFile(cmd *cobra.Command, path string) (string, error) {
	file, err := vfs.CurrentFS.ReadFile(path)
	if err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "error: cannot open "+path+": "+err.Error())
		return "", err
	}

	buff := make([]byte, 512)
	if len(file) < 512 {
		buff = make([]byte, len(file))
	}

	copy(buff, file)

	filetype := http.DetectContentType(buff)

	return filetype, nil
}

func isTarFile(cmd *cobra.Command, path string) (bool, error) {
	file, err := vfs.CurrentFS.ReadFile(path)
	if err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "error: cannot open "+path+": "+err.Error())
		return false, err
	}
	tarReader := tar.NewReader(bytes.NewReader(file))
	_, err = tarReader.Next()
	if err != nil {
		return false, fmt.Errorf("unable to read tarbal file: %w", err)
	}

	return true, nil
}

func isZip(cmd *cobra.Command, path string) (bool, error) {
	header, err := GetHeaderFile(cmd, path)
	if err == nil {
		return header == "application/zip", nil
	}
	return false, err
}

func isGzip(cmd *cobra.Command, path string) (bool, error) {
	header, err := GetHeaderFile(cmd, path)
	if err == nil {
		return header == "application/x-gzip", nil
	}
	return false, err
}

func isXZ(path string) (bool, error) {
	file, err := vfs.CurrentFS.ReadFile(path)
	if err != nil {
		return false, err
	}

	_, err = xz.NewReader(bytes.NewReader(file))
	if err != nil {
		return false, err
	}
	return true, nil
}

func IsCompressedFile(cmd *cobra.Command, path string) (bool, string, error) {
	result, err := isGzip(cmd, path)
	if err != nil {
		return false, "", err
	} else if result {
		return result, fileTypeTarGzip, nil
	}

	result, err = isZip(cmd, path)
	if err != nil {
		return false, "", err
	} else if result {
		return result, fileTypeZip, nil
	}

	result, err = isXZ(path)
	if err != nil {
		return false, "", err
	} else if result {
		return result, fileTypeXZ, nil
	}

	result, err = isTarFile(cmd, path)
	if err != nil {
		return false, "", err
	}

	return result, fileTypeTar, nil
}

func IsRemoteFile(path string) bool {
	parsedURL, err := url.Parse(path)
	return err == nil && parsedURL.Scheme != "" && parsedURL.Host != ""
}

func DownloadFile(cmd *cobra.Command, path string) (string, error) {
	tmpdir, err := os.MkdirTemp("", "omc-*")
	if err != nil {
		return "", err
	}

	resp, err := http.Get(path)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Use a sensible filename
	var filename string
	// First, try to extract filename from headers
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			filename = params["filename"]
		}
	}
	// If that fails, resort to parsing the path
	if filename == "" {
		if parsedURL, err := url.Parse(path); err == nil {
			filename = pathlib.Base(parsedURL.Path)
		}
	}

	outpath := filepath.Join(tmpdir, filename)
	fmt.Fprintln(cmd.OutOrStdout(), "downloading file "+path+" in "+outpath)

	out, err := os.Create(outpath)
	if err != nil {
		return "", err
	}

	counter := NewWriteCounter(cmd, resp.ContentLength)
	if _, err = io.Copy(out, io.TeeReader(resp.Body, counter)); err != nil {
		out.Close()
		return "", err
	}

	out.Close()
	fmt.Fprintln(cmd.OutOrStdout())

	return out.Name(), nil
}

func CopyFile(cmd *cobra.Command, path string, destinationfile string) error {
	source, err := vfs.CurrentFS.ReadFile(path)
	if err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "error opening file "+path+": "+err.Error())
		return err
	}
	dest, err := os.Create(destinationfile)
	if err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "error creating file "+destinationfile+": "+err.Error())
		return err
	}
	defer dest.Close()
	_, err = dest.Write(source)
	if err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "error copying file "+path+" to "+destinationfile+": "+err.Error())
	}
	return err
}

func DecompressFile(cmd *cobra.Command, path string, outpath string, fileType string) (string, error) {
	fmt.Fprintln(cmd.OutOrStdout(), "decompressing file "+path+" in "+outpath)
	var err error
	var mgRootDir string = ""

	switch fileType {
	case fileTypeTar:
		mgRootDir, err = ExtractTar(cmd, path, outpath)
	case fileTypeTarGzip:
		mgRootDir, err = ExtractTarGz(cmd, path, outpath)
	case fileTypeXZ:
		mgRootDir, err = extractTarXZ(cmd, path, outpath)
	case fileTypeZip:
		mgRootDir, err = ExtractZip(cmd, path, outpath)
	default:
		return "", fmt.Errorf("unable to decompress file: unknown file type %s", fileType)
	}

	return mgRootDir, err
}

func ExtractTarStream(cmd *cobra.Command, st io.Reader, destinationdir string) (string, error) {
	firstDirectory := false
	var mgRootDir string = ""
	tarReader := tar.NewReader(st)

	for true {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), "cannot extract tar: "+err.Error())
			return "", err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if !firstDirectory {
				firstDirectory = true
				mgRootDir = destinationdir + "/" + header.Name
			}
			directory := filepath.Join(destinationdir, header.Name)
			if _, err := os.Stat(directory); os.IsNotExist(err) {
				if err := os.Mkdir(directory, 0755); err != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), "mkdir failed extracting tar: "+err.Error())
					return "", err
				}
			}
		case tar.TypeReg:
			// Root dir is not part of the archive
			if mgRootDir == "" {
				mgRootDir = filepath.Join(destinationdir, filepath.Dir(header.Name))
				firstDirectory = true
				err := os.MkdirAll(mgRootDir, os.ModePerm)
				if err != nil && !os.IsExist(err) {
					return "", err
				}
			}
			outpath := filepath.Join(destinationdir, header.Name)
			if _, err := os.Stat(outpath); !os.IsNotExist(err) {
				fmt.Fprintln(cmd.ErrOrStderr(), "create file failed extracting tar: file already exists")
			}
			outFile, err := os.Create(outpath)
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "create file failed extracting tar: "+err.Error())
				return "", err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "copy file failed extracting tar: "+err.Error())
				return "", err
			}
			outFile.Close()
		default:
			fmt.Fprintf(cmd.ErrOrStderr(), "unknown type(%s) in %s: "+err.Error(), header.Typeflag, header.Name)
			return "", err
		}
	}
	return mgRootDir, nil
}

func ExtractTar(cmd *cobra.Command, tarfile string, destinationdir string) (string, error) {
	tarStream, err := vfs.CurrentFS.ReadFile(tarfile)
	var mgRootDir string
	if err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "error: cannot open "+tarfile+": "+err.Error())
		return "", err
	}

	var fileReader io.Reader = bytes.NewReader(tarStream)
	mgRootDir, err = ExtractTarStream(cmd, fileReader, destinationdir)

	return mgRootDir, err
}

func ExtractZip(cmd *cobra.Command, zipfile string, destinationdir string) (string, error) {

	firstDirectory := false
	var mgRootDir string = ""
	archive, err := zip.OpenReader(zipfile)
	if err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "error: cannot uncompress zip "+zipfile+": "+err.Error())
		return "", err
	}
	defer archive.Close()

	for _, f := range archive.File {
		filePath := filepath.Join(destinationdir, f.Name)

		// Root dir is not part of the archive
		if !f.FileInfo().IsDir() && mgRootDir == "" {
			mgRootDir = filepath.Dir(filePath)
			firstDirectory = true
			err := os.MkdirAll(mgRootDir, os.ModePerm)
			if err != nil && !os.IsExist(err) {
				return "", err
			}
		}

		if f.FileInfo().IsDir() {
			if !firstDirectory {
				firstDirectory = true
				mgRootDir = filePath
			}
			err = os.MkdirAll(filePath, os.ModePerm)
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "error: cannot create directory "+filePath+": "+err.Error())
				return "", err
			}
		} else {
			dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "error: cannot create file "+filePath+": "+err.Error())
				return "", err
			}
			defer dstFile.Close()

			fileInArchive, err := f.Open()
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "error: cannot open file "+f.Name+": "+err.Error())
				return "", err
			}
			defer fileInArchive.Close()

			if _, err := io.Copy(dstFile, fileInArchive); err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "error: cannot copy file to "+dstFile.Name()+": "+err.Error())
				return "", err
			}
		}
	}

	return mgRootDir, err
}

func ExtractTarGz(cmd *cobra.Command, gzipfile string, destinationdir string) (string, error) {
	gzipStream, err := vfs.CurrentFS.ReadFile(gzipfile)
	if err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "error: cannot open "+gzipfile+": "+err.Error())
		return "", err
	}
	uncompressedStream, err := gzip.NewReader(bytes.NewReader(gzipStream))
	if err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "error: cannot uncompress gzip "+gzipfile+": "+err.Error())
		return "", err
	}
	return ExtractTarStream(cmd, uncompressedStream, destinationdir)
}

func extractTarXZ(cmd *cobra.Command, xzFile string, destinationdir string) (string, error) {
	stream, err := vfs.CurrentFS.ReadFile(xzFile)
	if err != nil {
		return "", fmt.Errorf("error: cannot open %q: %w", xzFile, err)
	}

	xzReader, err := xz.NewReader(bytes.NewReader(stream))
	if err != nil {
		return "", fmt.Errorf("error: cannot uncompress xz file %q: %w", xzFile, err)
	}
	return ExtractTarStream(cmd, xzReader, destinationdir)
}

func extractClientVersion(cmd *cobra.Command, mustGatherLogsFilePath string) string {
	filePath := mustGatherLogsFilePath
	clientVersion := ""
	// Open the file
	file, err := vfs.CurrentFS.ReadFile(filePath)
	if err != nil {
		return ""
	}

	// Initialize a scanner to read the file line by line
	scanner := bufio.NewScanner(bytes.NewReader(file))

	// Variable to store the matching line
	var clientVersionLine string

	// Counter for the first 20 lines
	lineCount := 0

	// Read the file line by line
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		// Check if the line starts with "ClientVersion: "
		if strings.HasPrefix(line, "ClientVersion: ") {
			clientVersionLine = line
			break // Exit the loop as we found the line
		}

		// Stop checking after 20 lines as it should be at line 4
		if lineCount >= 20 {
			break
		}
	}

	// Handle potential scanning error
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "Error reading file:", err)
		return ""
	}

	// Check if we found the line and print the result
	if clientVersionLine != "" {
		parts := strings.Split(clientVersionLine, ":")
		if len(parts) == 2 {
			// Trim spaces and get the version part
			clientVersion = strings.TrimSpace(parts[1])
			return clientVersion
		}
	}
	return ""
}

const (
	ciArtifactHttpPrefix  = "http://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/"
	ciArtifactHttpsPrefix = "https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/"
	gcsUrlPrefix          = "gs://"
)

func isCIArtifactPath(url string) bool {
	return strings.HasPrefix(url, ciArtifactHttpPrefix) || strings.HasPrefix(url, ciArtifactHttpsPrefix)
}

func SanitizeCIArtifactPath(url string) string {
	if strings.HasPrefix(url, ciArtifactHttpPrefix) {
		return gcsUrlPrefix + strings.TrimPrefix(url, ciArtifactHttpPrefix)
	}

	if strings.HasPrefix(url, ciArtifactHttpsPrefix) {
		return gcsUrlPrefix + strings.TrimPrefix(url, ciArtifactHttpsPrefix)
	}

	return ""
}

func IsGCSPath(path string) bool {
	return strings.HasPrefix(path, "gs://")
}
