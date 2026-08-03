package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
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
	return UploadAudioReader(ctx, file, fileHeader.Filename, folder)
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

// AudioDeliveryURL rebuilds the public Cloudinary delivery URL for a public ID.
func AudioDeliveryURL(publicID string) (string, error) {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return "", errors.New("Cloudinary public ID is required")
	}

	cld, err := newCloudinaryClient()
	if err != nil {
		return "", err
	}

	asset, err := cld.Video(publicID)
	if err != nil {
		return "", fmt.Errorf("build Cloudinary delivery URL: %w", err)
	}
	url, err := asset.String()
	if err != nil {
		return "", fmt.Errorf("stringify Cloudinary delivery URL: %w", err)
	}
	return url, nil
}

// DownloadAudioBytes GETs the delivery URL for publicID and returns the body.
// maxBytes caps the read (use 10 MB for sync STT).
func DownloadAudioBytes(ctx context.Context, publicID string, maxBytes int64) ([]byte, error) {
	url, err := AudioDeliveryURL(publicID)
	if err != nil {
		return nil, err
	}
	if maxBytes <= 0 {
		maxBytes = 10 << 20
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download Cloudinary audio: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download Cloudinary audio %s: %s", url, res.Status)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Cloudinary audio: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("Cloudinary audio exceeds %d byte limit", maxBytes)
	}
	return body, nil
}

// UploadAudioBytes uploads raw audio bytes (e.g. MP3 reply) to Cloudinary.
func UploadAudioBytes(ctx context.Context, data []byte, filename, folder string) (secureURL string, publicID string, err error) {
	if len(data) == 0 {
		return "", "", errors.New("audio bytes are required")
	}
	return UploadAudioReader(ctx, bytes.NewReader(data), filename, folder)
}

// UploadAudioReader uploads audio from an io.Reader to Cloudinary.
func UploadAudioReader(ctx context.Context, r io.Reader, filename, folder string) (secureURL string, publicID string, err error) {
	if r == nil {
		return "", "", errors.New("audio reader is required")
	}
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "" {
		return "", "", errors.New("audio filename is required")
	}

	extension := filepath.Ext(filename)
	publicID = strings.TrimSuffix(filename, extension)
	if publicID == "" {
		return "", "", errors.New("audio filename is invalid")
	}

	cld, err := newCloudinaryClient()
	if err != nil {
		return "", "", err
	}

	uploadResult, err := cld.Upload.Upload(ctx, r, uploader.UploadParams{
		ResourceType: audioResourceType,
		PublicID:     publicID,
		Folder:       strings.Trim(folder, "/"),
	})
	if err != nil {
		return "", "", fmt.Errorf("upload audio to Cloudinary: %w", err)
	}

	return uploadResult.SecureURL, uploadResult.PublicID, nil
}
