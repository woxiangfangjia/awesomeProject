package fileserver

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
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

func CopyAndRenameFile(filePath, newFilePath string) error {
	// 打开源文件
	srcFile, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 创建目标文件
	dstFile, err := os.Create(newFilePath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// 将源文件内容拷贝到目标文件中
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	// 重命名副本 这什么垃圾Rename啊，天天爆32错误
	//err = os.Rename(newFilePath, filePath)
	//if err != nil {
	//	return err
	//}

	return nil
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
		//return err
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			return
		}
	}(file)

	// 创建上传目录
	if err := os.MkdirAll(uploadDirectory, os.ModePerm); err != nil {
		http.Error(w, "Error creating the upload directory", http.StatusInternalServerError)
		//return err
	}

	// 生成上传文件的路径
	filePath := filepath.Join(uploadDirectory, handler.Filename)
	dst, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Error creating the file on the server", http.StatusInternalServerError)
		//return err
	}
	defer func(dst *os.File) {
		err := dst.Close()
		if err != nil {
			return
		}
	}(dst)

	// 将上传的文件内容拷贝到服务器上的文件中
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Error copying file to server", http.StatusInternalServerError)
		//return err
	}

	file, err = os.Open(filePath)
	if err != nil {
		http.Error(w, "Error reopening the file", http.StatusInternalServerError)
		return
	}
	defer file.Close()
	// 计算文件的SHA256哈希值
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		http.Error(w, "Error calculating file hash", http.StatusInternalServerError)
		return
	}

	// 获取SHA256哈希值的十六进制表示
	//hash := hex.EncodeToString(hasher.Sum(nil))
	hashInBytes := hasher.Sum(nil)[:]
	sha256Hash := fmt.Sprintf("%x", hashInBytes)

	// 构建新的文件名
	newFileName := sha256Hash
	newFilePath := filepath.Join(uploadDirectory, newFileName)

	//重命名
	//err = os.Rename(filePath, newFilePath)
	//if err != nil {
	//	http.Error(w, "Error Rename", http.StatusInternalServerError)
	//	return
	//}
	file.Close()
	if CopyAndRenameFile(filePath, newFilePath) != nil {
		http.Error(w, "Error Rename", http.StatusInternalServerError)
	}

	// 返回上传成功的消息
	_, err = fmt.Fprintf(w, "File %s uploaded successfully", newFileName)
	if err != nil {
		//return err
	}
	//return nil
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
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)

	// 设置响应头，告知浏览器文件的类型
	w.Header().Set("Content-Type", "application/octet-stream")

	// 将文件内容复制到响应体中，实现文件下载
	_, err = io.Copy(w, file)
	if err != nil {
		http.Error(w, "Error copying file to response", http.StatusInternalServerError)
		return
	}
}
