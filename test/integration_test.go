package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"photo-library-server/config"
	"photo-library-server/database"
	"photo-library-server/handlers"
	"photo-library-server/middleware"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContext holds the test environment
type TestContext struct {
	DB      *database.SQLiteDB
	Router  *gin.Engine
	TempDir string
}

// TestLibrary represents a library for testing
type TestLibrary struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Images      string    `json:"images"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TestPhoto represents a photo for testing
type TestPhoto struct {
	ID           uuid.UUID `json:"id"`
	Filename     string    `json:"filename"`
	OriginalName string    `json:"original_name"`
	FilePath     string    `json:"file_path"`
	MimeType     string    `json:"mime_type"`
	FileSize     int64     `json:"file_size"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	Rating       *int      `json:"rating"`
	LibraryID    uuid.UUID `json:"library_id"`
	UploadedAt   time.Time `json:"uploaded_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TestAlbum represents an album for testing
type TestAlbum struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	LibraryID   uuid.UUID `json:"library_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TestTag represents a tag for testing
type TestTag struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// setupTestEnvironment creates a fresh test environment with a new database
func setupTestEnvironment(t *testing.T) *TestContext {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "photo_test_*")
	require.NoError(t, err)

	// Create test database in memory
	sqliteDB, err := database.NewSQLiteDB(":memory:")
	require.NoError(t, err)

	// Run migrations
	err = sqliteDB.Migrate()
	require.NoError(t, err)

	// Create indexes
	err = sqliteDB.CreateIndexes()
	require.NoError(t, err)

	// Setup Gin in test mode
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.CORSMiddleware())

	// Setup test config
	cfg := &config.Config{
		MaxFileSize: 50 * 1024 * 1024, // 50MB
		AllowedTypes: []string{
			"image/jpeg",
			"image/png",
			"image/gif",
			"image/webp",
			"image/tiff",
			"image/bmp",
		},
	}

	// Initialize handlers
	libraryHandler := handlers.NewLibraryHandler(sqliteDB.GetDB())
	albumHandler := handlers.NewAlbumHandler(sqliteDB.GetDB())
	photoHandler := handlers.NewPhotoHandler(sqliteDB.GetDB(), cfg)
	tagHandler := handlers.NewTagHandler(sqliteDB.GetDB())

	// Setup routes
	api := router.Group("/api/v1")
	{
		// Library routes
		libraries := api.Group("/libraries")
		{
			libraries.POST("", libraryHandler.CreateLibrary)
			libraries.GET("", libraryHandler.GetLibraries)
			libraries.GET("/:id", libraryHandler.GetLibrary)
			libraries.PUT("/:id", libraryHandler.UpdateLibrary)
			libraries.DELETE("/:id", libraryHandler.DeleteLibrary)
			libraries.GET("/:id/stats", libraryHandler.GetLibraryStats)
		}

		// Album routes
		albums := api.Group("/albums")
		{
			albums.POST("", albumHandler.CreateAlbum)
			albums.GET("", albumHandler.GetAlbums)
			albums.GET("/:id", albumHandler.GetAlbum)
			albums.PUT("/:id", albumHandler.UpdateAlbum)
			albums.DELETE("/:id", albumHandler.DeleteAlbum)
			albums.POST("/:id/photos", albumHandler.AddPhotoToAlbum)
			albums.DELETE("/:id/photos/:photo_id", albumHandler.RemovePhotoFromAlbum)
			albums.PUT("/:id/photos/:photo_id/order", albumHandler.UpdatePhotoOrder)
		}

		// Photo routes
		photos := api.Group("/photos")
		{
			photos.POST("/upload", photoHandler.UploadPhoto)
			photos.GET("", photoHandler.GetPhotos)
			photos.GET("/:id", photoHandler.GetPhoto)
			photos.PUT("/:id", photoHandler.UpdatePhoto)
			photos.DELETE("/:id", photoHandler.DeletePhoto)
			photos.GET("/:id/file", photoHandler.ServePhoto)
			photos.POST("/:id/copy", photoHandler.CopyPhoto)
		}

		// Tag routes
		tags := api.Group("/tags")
		{
			tags.POST("", tagHandler.CreateTag)
			tags.GET("", tagHandler.GetTags)
			tags.GET("/:id", tagHandler.GetTag)
			tags.PUT("/:id", tagHandler.UpdateTag)
			tags.DELETE("/:id", tagHandler.DeleteTag)
			tags.POST("/:id/photos", tagHandler.AddTagToPhoto)
			tags.DELETE("/:id/photos/:photo_id", tagHandler.RemoveTagFromPhoto)
			tags.GET("/:id/stats", tagHandler.GetTagStats)
		}
	}

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "photo-library-server",
		})
	})

	return &TestContext{
		DB:      sqliteDB,
		Router:  router,
		TempDir: tempDir,
	}
}

// cleanup cleans up the test environment
func (tc *TestContext) cleanup() {
	tc.DB.Close()
	os.RemoveAll(tc.TempDir)
}

// makeRequest makes an HTTP request and returns the response
func (tc *TestContext) makeRequest(method, url string, body interface{}) *httptest.ResponseRecorder {
	var req *http.Request
	var err error

	if body != nil {
		jsonBody, _ := json.Marshal(body)
		req, err = http.NewRequest(method, url, bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		panic(err)
	}

	w := httptest.NewRecorder()
	tc.Router.ServeHTTP(w, req)
	return w
}

// makeMultipartRequest makes a multipart form request for file uploads
func (tc *TestContext) makeMultipartRequest(url string, fields map[string]string, files map[string][]byte) *httptest.ResponseRecorder {
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)

	// Add form fields
	for key, value := range fields {
		writer.WriteField(key, value)
	}

	// Add files with proper MIME type
	for fieldName, fileData := range files {
		// Create form file with proper headers
		h := make(map[string][]string)
		h["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="%s"; filename="test.jpg"`, fieldName)}
		h["Content-Type"] = []string{"image/jpeg"}

		part, err := writer.CreatePart(h)
		if err != nil {
			panic(err)
		}
		part.Write(fileData)
	}

	writer.Close()

	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	tc.Router.ServeHTTP(w, req)
	return w
}

// createTestLibrary creates a test library and returns its details
func (tc *TestContext) createTestLibrary(name, description string) TestLibrary {
	imagePath := filepath.Join(tc.TempDir, "library_"+name)

	payload := map[string]interface{}{
		"name":        name,
		"description": description,
		"images":      imagePath,
	}

	resp := tc.makeRequest("POST", "/api/v1/libraries", payload)
	if resp.Code != http.StatusCreated {
		panic(fmt.Sprintf("Failed to create test library: %d - %s", resp.Code, resp.Body.String()))
	}

	var library TestLibrary
	json.Unmarshal(resp.Body.Bytes(), &library)
	return library
}

// createTestTag creates a test tag and returns its details
func (tc *TestContext) createTestTag(name, color string) TestTag {
	payload := map[string]interface{}{
		"name":  name,
		"color": color,
	}

	resp := tc.makeRequest("POST", "/api/v1/tags", payload)
	if resp.Code != http.StatusCreated {
		panic(fmt.Sprintf("Failed to create test tag: %d - %s", resp.Code, resp.Body.String()))
	}

	var tag TestTag
	json.Unmarshal(resp.Body.Bytes(), &tag)
	return tag
}

// createTestAlbum creates a test album and returns its details
func (tc *TestContext) createTestAlbum(name, description string, libraryID uuid.UUID) TestAlbum {
	payload := map[string]interface{}{
		"name":        name,
		"description": description,
		"library_id":  libraryID,
	}

	resp := tc.makeRequest("POST", "/api/v1/albums", payload)
	if resp.Code != http.StatusCreated {
		panic(fmt.Sprintf("Failed to create test album: %d - %s", resp.Code, resp.Body.String()))
	}

	var album TestAlbum
	json.Unmarshal(resp.Body.Bytes(), &album)
	return album
}

// createTestImage creates a valid JPEG image programmatically for testing
func createTestImage() []byte {
	// Create a simple 1x1 pixel image in memory and encode it as JPEG
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255}) // Red pixel

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
	if err != nil {
		panic("Failed to create test image: " + err.Error())
	}

	return buf.Bytes()
}

// uploadTestPhoto uploads a test photo and returns its details
func (tc *TestContext) uploadTestPhoto(libraryID uuid.UUID, filename string, rating *int, tags string) TestPhoto {
	fields := map[string]string{
		"library_id": libraryID.String(),
	}

	if rating != nil {
		fields["rating"] = fmt.Sprintf("%d", *rating)
	}

	if tags != "" {
		fields["tags"] = tags
	}

	files := map[string][]byte{
		"photo": createTestImage(),
	}

	resp := tc.makeMultipartRequest("/api/v1/photos/upload", fields, files)
	if resp.Code != http.StatusCreated {
		panic(fmt.Sprintf("Failed to upload test photo: %d - %s", resp.Code, resp.Body.String()))
	}

	var photo TestPhoto
	json.Unmarshal(resp.Body.Bytes(), &photo)
	return photo
}

// TestHealthEndpoint tests the health check endpoint
func TestHealthEndpoint(t *testing.T) {
	tc := setupTestEnvironment(t)
	defer tc.cleanup()

	resp := tc.makeRequest("GET", "/health", nil)

	assert.Equal(t, http.StatusOK, resp.Code)

	var response map[string]interface{}
	json.Unmarshal(resp.Body.Bytes(), &response)

	assert.Equal(t, "healthy", response["status"])
	assert.Equal(t, "photo-library-server", response["service"])
}
