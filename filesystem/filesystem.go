package fileserver

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const uploadDirectory = "./uploads"

// HandleFileUpload 处理文件上传
func HandleFileUpload(w http.ResponseWriter, r *http.Request) {
	// 限制上传文件的大小
	r.ParseMultipartForm(10 << 20) // 10 MB

	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving the file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 创建上传目录
	if err := os.MkdirAll(uploadDirectory, os.ModePerm); err != nil {
		http.Error(w, "Error creating the upload directory", http.StatusInternalServerError)
		return
	}

	// 生成上传文件的路径
	filePath := filepath.Join(uploadDirectory, handler.Filename)
	dst, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Error creating the file on the server", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	// 将上传的文件内容拷贝到服务器上的文件中
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Error copying file to server", http.StatusInternalServerError)
		return
	}

	// 返回上传成功的消息
	fmt.Fprintf(w, "File %s uploaded successfully", handler.Filename)
}

// HandleFileDownload 处理文件下载
func HandleFileDownload(w http.ResponseWriter, r *http.Request) {
	// 获取要下载的文件名
	fileName := filepath.Base(r.URL.Path)

	// 拼接文件的完整路径
	filePath := filepath.Join(uploadDirectory, fileName)

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	// 设置响应头，告知浏览器文件的类型
	w.Header().Set("Content-Type", "application/octet-stream")

	// 将文件内容复制到响应体中，实现文件下载
	_, err = io.Copy(w, file)
	if err != nil {
		http.Error(w, "Error copying file to response", http.StatusInternalServerError)
		return
	}
}
