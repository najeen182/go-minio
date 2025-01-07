package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	minioClient *minio.Client
	bucketName  string
)

func init() {
	// Load MinIO configuration from environment variables
	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")
	bucketName = os.Getenv("MINIO_BUCKET")

	if endpoint == "" || accessKey == "" || secretKey == "" || bucketName == "" {
		log.Fatal("Missing required environment variables: MINIO_ENDPOINT, MINIO_ACCESS_KEY, MINIO_SECRET_KEY, MINIO_BUCKET")
	}

	// Initialize MinIO client
	var err error
	minioClient, err = minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: true,
	})
	if err != nil {
		log.Fatalf("Failed to initialize MinIO client: %v", err)
	}

	// Check if the bucket exists
	exists, err := minioClient.BucketExists(context.Background(), bucketName)
	if err != nil {
		log.Fatalf("Failed to check bucket existence: %v", err)
	}
	if !exists {
		log.Fatalf("Bucket %s does not exist", bucketName)
	}
}

func staticFileHandler(w http.ResponseWriter, r *http.Request) {
	objectName := r.URL.Path[1:] // Remove leading slash
	if objectName == "" {
		http.Error(w, "Bad Request: Object name is required", http.StatusBadRequest)
		return
	}

	// Get the object from MinIO
	object, err := minioClient.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		log.Printf("Failed to get object %s: %v", objectName, err)
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	defer object.Close()

	// Set caching headers
	w.Header().Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year

	// Detect content type if possible
	stat, err := object.Stat()
	if err != nil {
		log.Printf("Failed to stat object %s: %v", objectName, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", stat.ContentType)

	// Serve the content
	http.ServeContent(w, r, objectName, stat.LastModified, object)
}

func main() {
	http.HandleFunc("/", staticFileHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port
	}

	log.Printf("Starting server on :%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
