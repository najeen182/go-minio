package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	minioClient *minio.Client
	bucketName  string
)

// CacheItem represents a cached file
type CacheItem struct {
	Content      []byte
	ContentType  string
	LastModified time.Time
	ExpiresAt    time.Time
}

var (
	cache    sync.Map          // In-memory cache
	cacheTTL = 5 * time.Minute // Cache expiration time
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

	object, err := minioClient.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		log.Printf("Failed to get object %s: %v", objectName, err)
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	defer object.Close()

	// Fetch metadata and content
	stat, err := object.Stat()
	if err != nil {
		log.Printf("Failed to stat object %s: %v", objectName, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	// Set caching headers
	w.Header().Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year

	w.Header().Set("Content-Type", stat.ContentType)
	w.Header().Set("X-Powered-By", "S3-WEB")
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	// Serve the content
	http.ServeContent(w, r, objectName, stat.LastModified, object)

}

func main() {
	http.HandleFunc("/", staticFileHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port
	}
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      http.DefaultServeMux,
		ReadTimeout:  15 * time.Second, // Limit reading request headers
		WriteTimeout: 15 * time.Second, // Limit response write time
		IdleTimeout:  60 * time.Second, // Limit idle keep-alive connections
	}
	log.Printf("Starting server on :%s", port)
	log.Fatal(server.ListenAndServe())

}
