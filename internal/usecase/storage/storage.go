package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/dh-kam/kakao-bot/pkg/kakao"
)

// UploadRequest holds parameters for uploading image to Kakao CDN.
type UploadRequest struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	TokenPath    string
	FilePath     string
	Out          io.Writer
}

// UploadUseCase uploads an image file to Kakao CDN.
type UploadUseCase struct{}

func NewUploadUseCase() *UploadUseCase {
	return &UploadUseCase{}
}

func (uc *UploadUseCase) Execute(ctx context.Context, req UploadRequest) error {
	out := req.Out
	if out == nil {
		out = os.Stdout
	}

	if req.FilePath == "" {
		return fmt.Errorf("file path is required")
	}

	file, err := os.Open(req.FilePath)
	if err != nil {
		return fmt.Errorf("open file %q: %w", req.FilePath, err)
	}
	defer file.Close()

	client := kakao.NewClient(kakao.ClientConfig{
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		RedirectURI:  req.RedirectURI,
		TokenStore:   kakao.NewFileTokenStore(req.TokenPath),
	})

	filename := filepath.Base(req.FilePath)
	info, err := client.Storage().UploadImage(ctx, file, filename)
	if err != nil {
		return fmt.Errorf("upload image: %w", err)
	}

	fmt.Fprintln(out, "Image uploaded successfully to Kakao CDN:")
	fmt.Fprintf(out, "  URL:     %s\n", info.Infos.Original.URL)
	fmt.Fprintf(out, "  Size:    %d x %d (%d bytes)\n", info.Infos.Original.Width, info.Infos.Original.Height, info.Infos.Original.Length)
	if info.Infos.Original.Expires > 0 {
		expireTime := time.Unix(info.Infos.Original.Expires, 0)
		fmt.Fprintf(out, "  Expires: %s\n", expireTime.Format(time.RFC3339))
	}

	return nil
}

// DeleteRequest holds parameters for deleting an image from Kakao CDN.
type DeleteRequest struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	TokenPath    string
	ImageURL     string
	Out          io.Writer
}

// DeleteUseCase deletes an uploaded image from Kakao CDN.
type DeleteUseCase struct{}

func NewDeleteUseCase() *DeleteUseCase {
	return &DeleteUseCase{}
}

func (uc *DeleteUseCase) Execute(ctx context.Context, req DeleteRequest) error {
	out := req.Out
	if out == nil {
		out = os.Stdout
	}

	if req.ImageURL == "" {
		return fmt.Errorf("image URL is required")
	}

	client := kakao.NewClient(kakao.ClientConfig{
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		RedirectURI:  req.RedirectURI,
		TokenStore:   kakao.NewFileTokenStore(req.TokenPath),
	})

	if err := client.Storage().DeleteImage(ctx, req.ImageURL); err != nil {
		return fmt.Errorf("delete image: %w", err)
	}

	fmt.Fprintln(out, "Image deleted successfully from Kakao CDN.")
	return nil
}
