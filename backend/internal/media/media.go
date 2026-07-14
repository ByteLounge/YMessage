package media

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	s3Client   *s3.Client
	bucketName string
	s3Endpoint string
)

func InitS3() {
	bucketName = os.Getenv("S3_BUCKET")
	if bucketName == "" {
		bucketName = "ymessage-media"
	}

	accessKey := os.Getenv("S3_ACCESS_KEY")
	secretKey := os.Getenv("S3_SECRET_KEY")
	s3Endpoint = os.Getenv("S3_ENDPOINT") // E.g., http://localhost:9000 for MinIO

	if accessKey == "" || secretKey == "" {
		logPrintln("Warning: S3 credentials not fully set, media upload will run in local mock mode.")
		return
	}

	// Load configuration
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		logPrintln("Failed to load S3 configurations: ", err)
		return
	}

	// If using MinIO or localstack, customize endpoint resolver
	s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		if s3Endpoint != "" {
			o.BaseEndpoint = aws.String(s3Endpoint)
			o.UsePathStyle = true
		}
	})

	logPrintln("S3 Client initialized successfully.")
}

func logPrintln(args ...interface{}) {
	fmt.Println(args...)
}

// Simple pixel-skipping image scaler to create thumbnails in pure Go
func resizeImage(src image.Image, w, h int) image.Image {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcX := bounds.Min.X + (x * bounds.Dx() / w)
			srcY := bounds.Min.Y + (y * bounds.Dy() / h)
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

// UploadAttachment handles media file upload, image compression, and thumbnail generation
func UploadAttachment(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is required"})
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	uniqueID := uuid.New().String()
	fileName := fmt.Sprintf("%s%s", uniqueID, ext)

	var fileData []byte
	var fileReader io.Reader = file

	// Read everything to memory to run thumbnail processing
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read upload file"})
		return
	}
	fileData = buf.Bytes()
	fileReader = bytes.NewReader(fileData)

	var thumbnailURL string
	contentType := header.Header.Get("Content-Type")

	// If file is an image, generate a thumbnail
	if contentType == "image/jpeg" || contentType == "image/png" {
		var img image.Image
		var decodeErr error
		if contentType == "image/jpeg" {
			img, decodeErr = jpeg.Decode(bytes.NewReader(fileData))
		} else {
			img, decodeErr = png.Decode(bytes.NewReader(fileData))
		}

		if decodeErr == nil {
			thumbImg := resizeImage(img, 150, 150)
			thumbBuf := new(bytes.Buffer)
			var encodeErr error
			if contentType == "image/jpeg" {
				encodeErr = jpeg.Encode(thumbBuf, thumbImg, &jpeg.Options{Quality: 70})
			} else {
				encodeErr = png.Encode(thumbBuf, thumbImg)
			}

			if encodeErr == nil {
				thumbFileName := fmt.Sprintf("thumb_%s%s", uniqueID, ext)
				thumbnailURL, _ = uploadToStorage(thumbFileName, thumbBuf.Bytes(), contentType)
			}
		}
	}

	// Upload main file
	fullURL, err := uploadToStorage(fileName, fileData, contentType)
	if err != nil {
		// Mock local fallback if S3 is not configured
		localPath := filepath.Join("uploads", fileName)
		os.MkdirAll("uploads", 0755)
		if err := os.WriteFile(localPath, fileData, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "S3 failed and local storage fallback failed"})
			return
		}
		fullURL = "/uploads/" + fileName
		if thumbnailURL == "" {
			thumbnailURL = fullURL
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"file_url":      fullURL,
		"thumbnail_url": thumbnailURL,
		"file_name":     header.Filename,
		"file_size":     header.Size,
		"content_type":  contentType,
	})
}

func uploadToStorage(key string, data []byte, contentType string) (string, error) {
	if s3Client == nil {
		return "", fmt.Errorf("s3 client uninitialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}

	// Return public URL
	var publicURL string
	if s3Endpoint != "" {
		publicURL = fmt.Sprintf("%s/%s/%s", s3Endpoint, bucketName, key)
	} else {
		publicURL = fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucketName, key)
	}

	return publicURL, nil
}
