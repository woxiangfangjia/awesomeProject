package fileserver

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const uploadDirectory = "./uploads"

// getFileHash returns the SHA-256 hash of the given file name.
func getFileHash(fileName string) (string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// HandleFileUpload 处理文件上传
func HandleFileUpload(w http.ResponseWriter, r *http.Request) /*error*/ {
	// 限制上传文件的大小
	//err := r.ParseMultipartForm(10 << 20)
	//if err != nil {
	//	return
	//} // 10 MB

	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving the file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Calculate SHA-256 hash of the original file name
	fileNameHash, err := getFileHash(handler.Filename)
	if err != nil {
		http.Error(w, "Error calculating file name hash", http.StatusInternalServerError)
		return
	}

	// Create upload directory if it doesn't exist
	if err := os.MkdirAll(uploadDirectory, os.ModePerm); err != nil {
		http.Error(w, "Error creating the upload directory", http.StatusInternalServerError)
		return
	}

	// Generate the new file name using the hash
	filePath := filepath.Join(uploadDirectory, fileNameHash)
	dst, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Error creating the file on the server", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	// Copy the uploaded file content to the server-side file
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Error copying file to server", http.StatusInternalServerError)
		return
	}

	// Return success message with the new file name
	fmt.Fprintf(w, "File %s uploaded successfully", fileNameHash)
}

// HandleFileDownload 处理文件下载
func HandleFileDownload(w http.ResponseWriter, r *http.Request) {
	// Get the requested file name
	fileName := filepath.Base(r.URL.Path)

	// Calculate SHA-256 hash of the requested file name
	fileNameHash, err := getFileHash(fileName)
	if err != nil {
		http.Error(w, "Error calculating file name hash", http.StatusInternalServerError)
		return
	}

	// Concatenate the hash with the upload directory to get the full path
	filePath := filepath.Join(uploadDirectory, fileNameHash)

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	// Set response header to indicate file download
	w.Header().Set("Content-Disposition", "attachment; filename="+fileName)

	// Copy file content to the response body
	_, err = io.Copy(w, file)
	if err != nil {
		http.Error(w, "Error copying file to response", http.StatusInternalServerError)
		return
	}
}
