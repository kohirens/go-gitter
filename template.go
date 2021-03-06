package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/kohirens/stdlib"
)

const (
	MAX_TPL_SIZE = 1e+7
)

type Client interface {
	Get(url string) (*http.Response, error)
	Head(url string) (*http.Response, error)
}

type tplVars map[string]string

var regExpTmplLocation = regexp.MustCompile(`^https?://.+$`)

// getPathType Check if the path is an HTTP or local directory URL.
func getPathType(tplPath string) (pathType string) {
	if regExpTmplLocation.MatchString(tplPath) {
		pathType = "http"
	}

	// Check if local dir also exists.
	if pathType == "" && stdlib.DirExist(tplPath) {
		pathType = "local"
	}

	return
}

// copyDir copies a source directory to another destination directory.
func copyDir(srcDir, dstDir string) (err error) {
	// TODO: Why not just use the OS to copy the files over!?
	files, err := ioutil.ReadDir(srcDir)
	if err != nil {
		return
	}

	err = os.MkdirAll(dstDir, DIR_MODE)
	if err != nil {
		return
	}

	for _, file := range files {
		srcPath := srcDir + PS + file.Name()

		if file.IsDir() {
			ferr := copyDir(srcPath, dstDir+PS+file.Name())
			if ferr != nil {
				err = ferr
				return
			}

			continue
		}

		srcR, ferr := os.Open(srcPath)
		if ferr != nil {
			err = ferr
			break
		}

		dstPath := dstDir + PS + file.Name()
		fmt.Printf("copy %q  to %q ", srcPath, dstPath)
		dstW, ferr := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY, file.Mode())
		if ferr != nil {
			err = ferr
			break
		}

		written, ferr := io.Copy(dstW, srcR)
		if ferr != nil {
			err = ferr
			break
		}

		// check file written matches the original file size.
		if written != file.Size() {
			err = fmt.Errorf("failed to copy file correctly, wrote %v, should have written %v", written, file.Size())
		}

		ferr = srcR.Close()
		if ferr != nil {
			err = fmt.Errorf("copyDir could not close the source file: %q", srcPath)
			break
		}

		ferr = dstW.Close()
		if ferr != nil {
			err = fmt.Errorf("copyDir could not close the destination file: %q", dstPath)
			break
		}
	}

	return
}

// download a template from a URL to a local directory.
func download(url, dstDir string, client Client) (zipFile string, err error) {
	// TODO: extract path from URL, after domain, and mkdirall.
	dest := path.Base(url)
	// HTTP Request
	resp, err := client.Get(url)
	if err != nil {
		return
	}

	if resp.StatusCode > 300 || resp.StatusCode < 200 {
		err = fmt.Errorf(errMsgs[0], resp.Status, resp.StatusCode)
		return
	}

	defer resp.Body.Close()

	zipFile = dstDir + PS + dest
	// make handle to the file.
	out, err := os.Create(zipFile)
	if err != nil {
		return
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)

	fmt.Printf("downloading %v to %v\n", url, dest)

	return
}

func extract(archivePath, dest string) (err error) {
	// Get resource to zip archive.
	archive, err := zip.OpenReader(archivePath)

	if err != nil {
		err = fmt.Errorf("could not open archive %q, error: %v", archivePath, err.Error())
		return
	}

	err = os.MkdirAll(dest, DIR_MODE)
	if err != nil {
		err = fmt.Errorf("could not write dest %q, error: %v", dest, err.Error())
		return
	}

	fmt.Printf("extracting %v to %v\n", archivePath, dest)
	for _, file := range archive.File {
		sourceFile, ferr := file.Open()

		if ferr != nil {
			err = fmt.Errorf("failed to extract archive %q to dest %q, error: %v", archivePath, dest, file.Name)
			break
		}

		deflateFilePath := filepath.Join(dest, file.Name)

		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(deflateFilePath, filepath.Clean(dest)+PS) {
			err = fmt.Errorf("illegal file path: %s", deflateFilePath)
			return
		}

		if file.FileInfo().IsDir() {
			ferr := os.MkdirAll(deflateFilePath, file.Mode())
			if ferr != nil {
				err = ferr
				return
			}
		} else {
			dh, ferr := os.OpenFile(deflateFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())

			if ferr != nil {
				err = ferr
				return
			}

			_, ferr = io.Copy(dh, sourceFile)
			if ferr != nil {
				err = ferr
				return
			}

			ferr = dh.Close()
			if ferr != nil {
				panic(ferr)
			}
		}

		ferr = sourceFile.Close()
		if ferr != nil {
			err = fmt.Errorf("unsuccessful extracting archive %q, error: %v", archivePath, ferr.Error())
		}
	}

	archive.Close()

	return
}

// parse a a file as a Go template.
func parse(tplFile, dstDir string, vars tplVars) (err error) {

	parser, err := template.ParseFiles(tplFile)

	if err != nil {
		return
	}

	fileStats, err := os.Stat(tplFile)

	if err != nil {
		return
	}

	dstFile := dstDir + PS + fileStats.Name()
	file, err := os.OpenFile(dstFile, os.O_CREATE|os.O_WRONLY, fileStats.Mode())

	if err != nil {
		return
	}

	err = parser.Execute(file, vars)

	return
}

// parseDir Recursively walk a directory parsing all files along the way as Go templates.
func parseDir(tplDir, outDir string, vars tplVars, fec *stdlib.FileExtChecker) (err error) {
	// Recursively walk the template directory.
	err = filepath.Walk(tplDir, func(sourcePath string, fi os.FileInfo, wErr error) (rErr error) {
		if wErr != nil {
			rErr = wErr
			return
		}

		// Do not parse directories.
		if fi.IsDir() {
			return
		}

		// Stop processing files if a template file is too big.
		if fi.Size() > MAX_TPL_SIZE {
			rErr = fmt.Errorf("Template file too big to parse, must be less thatn %v bytes.", MAX_TPL_SIZE)
			return
		}

		// Skip non-text files.
		if !fec.IsValid(sourcePath) { // Use an exclude list, include every file by default.
			rErr = fmt.Errorf("could not detect file type for %v", sourcePath)
			return
		}
		// TODO: Update outDir to append any subdirectories we are walking from tplDir.
		partial := strings.ReplaceAll(sourcePath, tplDir, "")
		partial = strings.ReplaceAll(partial, PS, "/")
		saveDir := path.Clean(outDir + path.Dir(partial))

		// TODO: Make the subdirectories in the new savePath.
		err = os.MkdirAll(saveDir, DIR_MODE)
		if err != nil {
			return
		}

		rErr = parse(sourcePath, saveDir, vars)

		return
	})

	return
}
