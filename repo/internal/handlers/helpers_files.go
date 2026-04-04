package handlers

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func saveAvatarFile(fh *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		return "", errors.New("avatar must be jpg or png")
	}
	if err := validateUploadedImageSignature(fh); err != nil {
		return "", err
	}
	if fh.Size > 2*1024*1024 {
		return "", errors.New("avatar must be <= 2MB")
	}
	dir := filepath.Join(".", "static", "uploads", "avatars")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := strconv.FormatInt(time.Now().UnixNano(), 10) + "_" + filepath.Base(fh.Filename)
	target := filepath.Join(dir, name)
	src, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()
	out, err := os.Create(target)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := out.ReadFrom(src); err != nil {
		return "", err
	}
	return "/static/uploads/avatars/" + name, nil
}

func saveMemberImportErrorReport(contents string) (string, error) {
	dir := filepath.Join(".", "static", "uploads", "reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := "member_import_errors_" + strconv.FormatInt(time.Now().UnixNano(), 10) + ".csv"
	target := filepath.Join(dir, name)
	if err := os.WriteFile(target, []byte(contents), 0o644); err != nil {
		return "", err
	}
	return "/static/uploads/reports/" + name, nil
}

func validateUploadedImageSignature(fh *multipart.FileHeader) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	buf := make([]byte, 512)
	n, err := src.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	contentType := http.DetectContentType(buf[:n])
	if contentType != "image/jpeg" && contentType != "image/png" {
		return errors.New("avatar must be valid image content")
	}
	return nil
}
