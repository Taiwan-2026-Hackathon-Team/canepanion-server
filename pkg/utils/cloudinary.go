package utils

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

const audioResourceType = "video"

func getCloudinaryURL() (string, error) {
	cloudName := os.Getenv("CLOUDINARY_CLOUD_NAME")
	apiKey := os.Getenv("CLOUDINARY_API_KEY")
	apiSecret := os.Getenv("CLOUDINARY_API_SECRET")

	if cloudName == "" || apiKey == "" || apiSecret == "" {
		return "", errors.New("Cloudinary environment variables are not configured")
	}

	return fmt.Sprintf(
		"cloudinary://%s:%s@%s",
		apiKey,
		apiSecret,
		cloudName,
	), nil
}

func newCloudinaryClient() (*cloudinary.Cloudinary, error) {
	cloudinaryURL, err := getCloudinaryURL()
	if err != nil {
		return nil, err
	}

	cld, err := cloudinary.NewFromURL(cloudinaryURL)
	if err != nil {
		return nil, fmt.Errorf("create Cloudinary client: %w", err)
	}

	return cld, nil
}

// UploadAudio uploads an audio file to Cloudinary.
func UploadAudio(
	ctx context.Context,
	file multipart.File,
	fileHeader *multipart.FileHeader,
	folder string,
) (secureURL string, publicID string, err error) {
	if file == nil {
		return "", "", errors.New("audio file is required")
	}

	if fileHeader == nil {
		return "", "", errors.New("audio file header is required")
	}

	cld, err := newCloudinaryClient()
	if err != nil {
		return "", "", err
	}

	// Remove directories and the extension from the uploaded filename
	filename := filepath.Base(fileHeader.Filename)
	extension := filepath.Ext(filename)
	publicID = strings.TrimSuffix(filename, extension)

	if publicID == "" {
		return "", "", errors.New("audio filename is invalid")
	}

	uploadResult, err := cld.Upload.Upload(ctx, file, uploader.UploadParams{
		ResourceType: audioResourceType,
		PublicID:     publicID,
		Folder:       strings.Trim(folder, "/"),
	})
	if err != nil {
		return "", "", fmt.Errorf("upload audio to Cloudinary: %w", err)
	}

	return uploadResult.SecureURL, uploadResult.PublicID, nil
}

func DeleteAudio(ctx context.Context, publicID string) error {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return errors.New("Cloudinary public ID is required")
	}

	cld, err := newCloudinaryClient()
	if err != nil {
		return err
	}

	invalidate := true
	result, err := cld.Upload.Destroy(ctx, uploader.DestroyParams{
		PublicID:     publicID,
		ResourceType: audioResourceType,
		Invalidate:   &invalidate,
	})
	if err != nil {
		return fmt.Errorf("delete audio from Cloudinary: %w", err)
	}

	if result.Result != "ok" && result.Result != "not found" {
		return fmt.Errorf("unexpected Cloudinary deletion result: %s", result.Result)
	}

	return nil
}
