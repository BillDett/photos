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

// TestLibraryEndpoints tests all library-related endpoints
func TestLibraryEndpoints(t *testing.T) {
	tc := setupTestEnvironment(t)
	defer tc.cleanup()

	t.Run("Create Library - Success", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":        "Test Library",
			"description": "A test library",
			"images":      filepath.Join(tc.TempDir, "test_library"),
		}

		resp := tc.makeRequest("POST", "/api/v1/libraries", payload)
		assert.Equal(t, http.StatusCreated, resp.Code)

		var library TestLibrary
		json.Unmarshal(resp.Body.Bytes(), &library)

		assert.NotEqual(t, uuid.Nil, library.ID)
		assert.Equal(t, "Test Library", library.Name)
		assert.Equal(t, "A test library", library.Description)
		assert.Equal(t, filepath.Join(tc.TempDir, "test_library"), library.Images)
		assert.False(t, library.CreatedAt.IsZero())
		assert.False(t, library.UpdatedAt.IsZero())
	})

	t.Run("Create Library - Duplicate Name", func(t *testing.T) {
		// First library
		tc.createTestLibrary("Duplicate Name", "First library")

		// Try to create another with same name
		payload := map[string]interface{}{
			"name":        "Duplicate Name",
			"description": "Second library",
			"images":      filepath.Join(tc.TempDir, "different_path"),
		}

		resp := tc.makeRequest("POST", "/api/v1/libraries", payload)
		assert.Equal(t, http.StatusConflict, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "already exists")
	})

	t.Run("Create Library - Duplicate Images Path", func(t *testing.T) {
		// First library
		imagePath := filepath.Join(tc.TempDir, "same_image_path")

		// Create first library with specific path
		payload := map[string]interface{}{
			"name":        "First Library",
			"description": "First",
			"images":      imagePath,
		}
		resp := tc.makeRequest("POST", "/api/v1/libraries", payload)
		assert.Equal(t, http.StatusCreated, resp.Code)

		// Try to create another with same images path
		payload = map[string]interface{}{
			"name":        "Second Library",
			"description": "Second library",
			"images":      imagePath,
		}

		resp = tc.makeRequest("POST", "/api/v1/libraries", payload)
		assert.Equal(t, http.StatusConflict, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "images path already exists")
	})

	t.Run("Create Library - Validation Errors", func(t *testing.T) {
		// Test empty name
		payload := map[string]interface{}{
			"name":        "",
			"description": "Test",
			"images":      "/test/path",
		}
		resp := tc.makeRequest("POST", "/api/v1/libraries", payload)
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		// Test missing required fields
		payload = map[string]interface{}{
			"description": "Test",
		}
		resp = tc.makeRequest("POST", "/api/v1/libraries", payload)
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		// Test name too long
		payload = map[string]interface{}{
			"name":        string(make([]byte, 101)), // 101 characters
			"description": "Test",
			"images":      "/test/path",
		}
		resp = tc.makeRequest("POST", "/api/v1/libraries", payload)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
	})

	t.Run("Get Libraries", func(t *testing.T) {
		// Create test libraries
		testLib1 := tc.createTestLibrary("Library 1", "First library")
		testLib2 := tc.createTestLibrary("Library 2", "Second library")

		resp := tc.makeRequest("GET", "/api/v1/libraries", nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var libraries []TestLibrary
		json.Unmarshal(resp.Body.Bytes(), &libraries)

		assert.GreaterOrEqual(t, len(libraries), 2)

		// Find our libraries
		found1, found2 := false, false
		for _, lib := range libraries {
			if lib.ID == testLib1.ID {
				found1 = true
				assert.Equal(t, "Library 1", lib.Name)
			}
			if lib.ID == testLib2.ID {
				found2 = true
				assert.Equal(t, "Library 2", lib.Name)
			}
		}
		assert.True(t, found1, "Library 1 not found")
		assert.True(t, found2, "Library 2 not found")
	})

	t.Run("Get Library by ID", func(t *testing.T) {
		library := tc.createTestLibrary("Single Library", "Test library")

		resp := tc.makeRequest("GET", fmt.Sprintf("/api/v1/libraries/%s", library.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var retrievedLibrary TestLibrary
		json.Unmarshal(resp.Body.Bytes(), &retrievedLibrary)

		assert.Equal(t, library.ID, retrievedLibrary.ID)
		assert.Equal(t, library.Name, retrievedLibrary.Name)
		assert.Equal(t, library.Description, retrievedLibrary.Description)
		assert.Equal(t, library.Images, retrievedLibrary.Images)
	})

	t.Run("Get Library by ID - Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		resp := tc.makeRequest("GET", fmt.Sprintf("/api/v1/libraries/%s", nonExistentID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Library not found", response["error"])
	})

	t.Run("Get Library by ID - Invalid UUID", func(t *testing.T) {
		resp := tc.makeRequest("GET", "/api/v1/libraries/invalid-uuid", nil)
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Invalid library ID", response["error"])
	})

	t.Run("Update Library", func(t *testing.T) {
		library := tc.createTestLibrary("Original Name", "Original description")

		payload := map[string]interface{}{
			"name":        "Updated Name",
			"description": "Updated description",
		}

		resp := tc.makeRequest("PUT", fmt.Sprintf("/api/v1/libraries/%s", library.ID), payload)
		assert.Equal(t, http.StatusOK, resp.Code)

		var updatedLibrary TestLibrary
		json.Unmarshal(resp.Body.Bytes(), &updatedLibrary)

		assert.Equal(t, library.ID, updatedLibrary.ID)
		assert.Equal(t, "Updated Name", updatedLibrary.Name)
		assert.Equal(t, "Updated description", updatedLibrary.Description)
		assert.Equal(t, library.Images, updatedLibrary.Images) // Should remain unchanged
	})

	t.Run("Update Library - Path Change", func(t *testing.T) {
		library := tc.createTestLibrary("Path Test", "Test path change")
		newPath := filepath.Join(tc.TempDir, "new_path")

		payload := map[string]interface{}{
			"images": newPath,
		}

		resp := tc.makeRequest("PUT", fmt.Sprintf("/api/v1/libraries/%s", library.ID), payload)
		assert.Equal(t, http.StatusOK, resp.Code)

		var updatedLibrary TestLibrary
		json.Unmarshal(resp.Body.Bytes(), &updatedLibrary)

		assert.Equal(t, newPath, updatedLibrary.Images)

		// Verify directory was created
		_, err := os.Stat(newPath)
		assert.NoError(t, err, "New directory should be created")
	})

	t.Run("Update Library - Conflicting Name", func(t *testing.T) {
		tc.createTestLibrary("Library One", "First")
		conflictLib := tc.createTestLibrary("Library Two", "Second")

		// Try to update conflictLib to have the same name as the first library
		payload := map[string]interface{}{
			"name": "Library One",
		}

		resp := tc.makeRequest("PUT", fmt.Sprintf("/api/v1/libraries/%s", conflictLib.ID), payload)
		assert.Equal(t, http.StatusConflict, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "already exists")
	})

	t.Run("Update Library - Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		payload := map[string]interface{}{
			"name": "New Name",
		}

		resp := tc.makeRequest("PUT", fmt.Sprintf("/api/v1/libraries/%s", nonExistentID), payload)
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("Delete Library", func(t *testing.T) {
		library := tc.createTestLibrary("To Delete", "This will be deleted")

		resp := tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/libraries/%s", library.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Library deleted successfully", response["message"])

		// Verify library is gone
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/libraries/%s", library.ID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		// Verify directory was removed
		_, err := os.Stat(library.Images)
		assert.True(t, os.IsNotExist(err), "Library directory should be removed")
	})

	t.Run("Delete Library - Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		resp := tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/libraries/%s", nonExistentID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("Get Library Stats", func(t *testing.T) {
		library := tc.createTestLibrary("Stats Library", "For testing stats")

		resp := tc.makeRequest("GET", fmt.Sprintf("/api/v1/libraries/%s/stats", library.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var stats map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &stats)

		assert.Equal(t, library.ID.String(), stats["library_id"])
		assert.Equal(t, library.Name, stats["library_name"])
		assert.Equal(t, float64(0), stats["photo_count"])
		assert.Equal(t, float64(0), stats["album_count"])
		assert.Equal(t, float64(0), stats["tag_count"])
		assert.Equal(t, float64(0), stats["total_size_bytes"])
	})

	t.Run("Get Library Stats - Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		resp := tc.makeRequest("GET", fmt.Sprintf("/api/v1/libraries/%s/stats", nonExistentID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})
}

// TestPhotoEndpoints tests all photo-related endpoints
func TestPhotoEndpoints(t *testing.T) {
	tc := setupTestEnvironment(t)
	defer tc.cleanup()

	// Setup test data
	library := tc.createTestLibrary("Photo Library", "For photo tests")

	t.Run("Upload Photo - Success", func(t *testing.T) {
		rating := 4
		photo := tc.uploadTestPhoto(library.ID, "test.jpg", &rating, "nature,landscape")

		assert.NotEqual(t, uuid.Nil, photo.ID)
		assert.Equal(t, "test.jpg", photo.OriginalName)
		assert.Equal(t, "image/jpeg", photo.MimeType)
		assert.Equal(t, library.ID, photo.LibraryID)
		assert.Equal(t, 4, *photo.Rating)
		assert.False(t, photo.UploadedAt.IsZero())
		assert.False(t, photo.CreatedAt.IsZero())
		assert.True(t, photo.FileSize > 0)
		assert.True(t, photo.Width > 0)
		assert.True(t, photo.Height > 0)

		// Verify file was actually created
		_, err := os.Stat(photo.FilePath)
		assert.NoError(t, err, "Photo file should exist")
	})

	t.Run("Upload Photo - Library Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		fields := map[string]string{
			"library_id": nonExistentID.String(),
		}
		files := map[string][]byte{
			"photo": createTestImage(),
		}

		resp := tc.makeMultipartRequest("/api/v1/photos/upload", fields, files)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Library not found", response["error"])
	})

	t.Run("Upload Photo - Invalid Library ID", func(t *testing.T) {
		fields := map[string]string{
			"library_id": "invalid-uuid",
		}
		files := map[string][]byte{
			"photo": createTestImage(),
		}

		resp := tc.makeMultipartRequest("/api/v1/photos/upload", fields, files)
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Invalid library ID", response["error"])
	})

	t.Run("Upload Photo - Missing Library ID", func(t *testing.T) {
		fields := map[string]string{}
		files := map[string][]byte{
			"photo": createTestImage(),
		}

		resp := tc.makeMultipartRequest("/api/v1/photos/upload", fields, files)
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "library_id is required", response["error"])
	})

	t.Run("Upload Photo - Missing File", func(t *testing.T) {
		fields := map[string]string{
			"library_id": library.ID.String(),
		}
		files := map[string][]byte{}

		resp := tc.makeMultipartRequest("/api/v1/photos/upload", fields, files)
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "No photo file provided", response["error"])
	})

	t.Run("Upload Photo - Invalid Rating", func(t *testing.T) {
		fields := map[string]string{
			"library_id": library.ID.String(),
			"rating":     "10", // Invalid rating (max is 5)
		}
		files := map[string][]byte{
			"photo": createTestImage(),
		}

		resp := tc.makeMultipartRequest("/api/v1/photos/upload", fields, files)
		// Should succeed but ignore invalid rating
		assert.Equal(t, http.StatusCreated, resp.Code)

		var photo TestPhoto
		json.Unmarshal(resp.Body.Bytes(), &photo)
		assert.Nil(t, photo.Rating, "Invalid rating should be ignored")
	})

	t.Run("Get Photos", func(t *testing.T) {
		// Upload test photos
		rating3 := 3
		rating5 := 5
		photo1 := tc.uploadTestPhoto(library.ID, "photo1.jpg", &rating3, "nature")
		photo2 := tc.uploadTestPhoto(library.ID, "photo2.jpg", &rating5, "portrait")

		resp := tc.makeRequest("GET", "/api/v1/photos", nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)

		photos := response["photos"].([]interface{})
		pagination := response["pagination"].(map[string]interface{})

		assert.GreaterOrEqual(t, len(photos), 2)
		assert.Equal(t, float64(1), pagination["page"])
		assert.Equal(t, float64(50), pagination["limit"])
		assert.GreaterOrEqual(t, pagination["total"], float64(2))

		// Find our photos
		found1, found2 := false, false
		for _, p := range photos {
			photoMap := p.(map[string]interface{})
			if photoMap["id"].(string) == photo1.ID.String() {
				found1 = true
			}
			if photoMap["id"].(string) == photo2.ID.String() {
				found2 = true
			}
		}
		assert.True(t, found1, "Photo 1 not found")
		assert.True(t, found2, "Photo 2 not found")
	})

	t.Run("Get Photos - Filter by Library", func(t *testing.T) {
		// Create another library and photo
		otherLibrary := tc.createTestLibrary("Other Library", "Another library")
		tc.uploadTestPhoto(otherLibrary.ID, "other.jpg", nil, "")

		// Filter by original library
		resp := tc.makeRequest("GET", fmt.Sprintf("/api/v1/photos?library_id=%s", library.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)

		photos := response["photos"].([]interface{})
		for _, p := range photos {
			photoMap := p.(map[string]interface{})
			assert.Equal(t, library.ID.String(), photoMap["library_id"])
		}
	})

	t.Run("Get Photos - Filter by Rating", func(t *testing.T) {
		resp := tc.makeRequest("GET", "/api/v1/photos?rating=5", nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)

		photos := response["photos"].([]interface{})
		for _, p := range photos {
			photoMap := p.(map[string]interface{})
			if photoMap["rating"] != nil {
				assert.Equal(t, float64(5), photoMap["rating"])
			}
		}
	})

	t.Run("Get Photo by ID", func(t *testing.T) {
		rating := 2
		uploadedPhoto := tc.uploadTestPhoto(library.ID, "single.jpg", &rating, "test")

		resp := tc.makeRequest("GET", fmt.Sprintf("/api/v1/photos/%s", uploadedPhoto.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var photo TestPhoto
		json.Unmarshal(resp.Body.Bytes(), &photo)

		assert.Equal(t, uploadedPhoto.ID, photo.ID)
		assert.Equal(t, uploadedPhoto.Filename, photo.Filename)
		assert.Equal(t, uploadedPhoto.OriginalName, photo.OriginalName)
		assert.Equal(t, uploadedPhoto.LibraryID, photo.LibraryID)
		assert.Equal(t, *uploadedPhoto.Rating, *photo.Rating)
	})

	t.Run("Get Photo by ID - Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		resp := tc.makeRequest("GET", fmt.Sprintf("/api/v1/photos/%s", nonExistentID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Photo not found", response["error"])
	})

	t.Run("Update Photo Rating", func(t *testing.T) {
		uploadedPhoto := tc.uploadTestPhoto(library.ID, "update.jpg", nil, "")

		payload := map[string]interface{}{
			"rating": 4,
		}

		resp := tc.makeRequest("PUT", fmt.Sprintf("/api/v1/photos/%s", uploadedPhoto.ID), payload)
		assert.Equal(t, http.StatusOK, resp.Code)

		var updatedPhoto TestPhoto
		json.Unmarshal(resp.Body.Bytes(), &updatedPhoto)

		assert.Equal(t, uploadedPhoto.ID, updatedPhoto.ID)
		assert.Equal(t, 4, *updatedPhoto.Rating)
	})

	t.Run("Update Photo - Invalid Rating", func(t *testing.T) {
		uploadedPhoto := tc.uploadTestPhoto(library.ID, "invalid_rating.jpg", nil, "")

		payload := map[string]interface{}{
			"rating": 10, // Invalid rating
		}

		resp := tc.makeRequest("PUT", fmt.Sprintf("/api/v1/photos/%s", uploadedPhoto.ID), payload)
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Contains(t, response["error"].(string), "rating")
	})

	t.Run("Serve Photo File", func(t *testing.T) {
		uploadedPhoto := tc.uploadTestPhoto(library.ID, "serve.jpg", nil, "")

		resp := tc.makeRequest("GET", fmt.Sprintf("/api/v1/photos/%s/file", uploadedPhoto.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		// Verify headers
		assert.Equal(t, uploadedPhoto.MimeType, resp.Header().Get("Content-Type"))
		assert.Contains(t, resp.Header().Get("Content-Disposition"), uploadedPhoto.OriginalName)

		// Verify we got actual image data
		assert.True(t, resp.Body.Len() > 0)
	})

	t.Run("Serve Photo File - Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		resp := tc.makeRequest("GET", fmt.Sprintf("/api/v1/photos/%s/file", nonExistentID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("Copy Photo - Same Library", func(t *testing.T) {
		originalPhoto := tc.uploadTestPhoto(library.ID, "original.jpg", nil, "original,tag")

		payload := map[string]interface{}{
			"library_id": library.ID,
		}

		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/photos/%s/copy", originalPhoto.ID), payload)
		assert.Equal(t, http.StatusCreated, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)

		assert.Equal(t, "Photo copied successfully", response["message"])
		assert.Equal(t, originalPhoto.ID.String(), response["original_id"])

		copiedPhotoData := response["copied_photo"].(map[string]interface{})
		assert.NotEqual(t, originalPhoto.ID.String(), copiedPhotoData["id"])
		assert.Equal(t, originalPhoto.OriginalName, copiedPhotoData["original_name"])
		assert.Equal(t, originalPhoto.MimeType, copiedPhotoData["mime_type"])
		assert.Equal(t, originalPhoto.LibraryID.String(), copiedPhotoData["library_id"])
		assert.NotEqual(t, originalPhoto.Filename, copiedPhotoData["filename"]) // Should have unique filename

		// Verify files exist
		_, err := os.Stat(originalPhoto.FilePath)
		assert.NoError(t, err, "Original file should still exist")

		copiedFilePath := copiedPhotoData["file_path"].(string)
		_, err = os.Stat(copiedFilePath)
		assert.NoError(t, err, "Copied file should exist")
	})

	t.Run("Copy Photo - Different Library", func(t *testing.T) {
		targetLibrary := tc.createTestLibrary("Target Library", "Copy destination")
		originalPhoto := tc.uploadTestPhoto(library.ID, "cross_library.jpg", nil, "tag1,tag2")

		payload := map[string]interface{}{
			"library_id": targetLibrary.ID,
		}

		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/photos/%s/copy", originalPhoto.ID), payload)
		assert.Equal(t, http.StatusCreated, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)

		copiedPhotoData := response["copied_photo"].(map[string]interface{})
		assert.Equal(t, targetLibrary.ID.String(), copiedPhotoData["library_id"])

		// Verify files exist in different directories
		_, err := os.Stat(originalPhoto.FilePath)
		assert.NoError(t, err, "Original file should still exist")

		copiedFilePath := copiedPhotoData["file_path"].(string)
		_, err = os.Stat(copiedFilePath)
		assert.NoError(t, err, "Copied file should exist")

		// Verify they're in different directories
		assert.Contains(t, copiedFilePath, targetLibrary.Images)
		assert.NotContains(t, copiedFilePath, library.Images)
	})

	t.Run("Copy Photo - Source Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		payload := map[string]interface{}{
			"library_id": library.ID,
		}

		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/photos/%s/copy", nonExistentID), payload)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Source photo not found", response["error"])
	})

	t.Run("Copy Photo - Target Library Not Found", func(t *testing.T) {
		originalPhoto := tc.uploadTestPhoto(library.ID, "no_target.jpg", nil, "")
		nonExistentID := uuid.New()

		payload := map[string]interface{}{
			"library_id": nonExistentID,
		}

		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/photos/%s/copy", originalPhoto.ID), payload)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Target library not found", response["error"])
	})

	t.Run("Delete Photo", func(t *testing.T) {
		photoToDelete := tc.uploadTestPhoto(library.ID, "delete_me.jpg", nil, "")

		// Verify file exists before deletion
		_, err := os.Stat(photoToDelete.FilePath)
		assert.NoError(t, err, "Photo file should exist before deletion")

		resp := tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/photos/%s", photoToDelete.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Photo deleted successfully", response["message"])

		// Verify photo is gone from database
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/photos/%s", photoToDelete.ID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		// Verify file is removed
		_, err = os.Stat(photoToDelete.FilePath)
		assert.True(t, os.IsNotExist(err), "Photo file should be deleted")
	})

	t.Run("Delete Photo - Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		resp := tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/photos/%s", nonExistentID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})
}

// TestAlbumEndpoints tests all album-related endpoints
func TestAlbumEndpoints(t *testing.T) {
	tc := setupTestEnvironment(t)
	defer tc.cleanup()

	// Setup test data
	library := tc.createTestLibrary("Album Library", "For album tests")
	otherLibrary := tc.createTestLibrary("Other Library", "Different library for cross-library tests")

	t.Run("Create Album - Success", func(t *testing.T) {
		album := tc.createTestAlbum("Test Album", "A test album", library.ID)

		assert.NotEqual(t, uuid.Nil, album.ID)
		assert.Equal(t, "Test Album", album.Name)
		assert.Equal(t, "A test album", album.Description)
		assert.Equal(t, library.ID, album.LibraryID)
		assert.False(t, album.CreatedAt.IsZero())
		assert.False(t, album.UpdatedAt.IsZero())
	})

	t.Run("Create Album - Library Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		payload := map[string]interface{}{
			"name":        "Invalid Album",
			"description": "This should fail",
			"library_id":  nonExistentID,
		}

		resp := tc.makeRequest("POST", "/api/v1/albums", payload)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Library not found", response["error"])
	})

	t.Run("Create Album - Validation Errors", func(t *testing.T) {
		// Test empty name
		payload := map[string]interface{}{
			"name":        "",
			"description": "Test",
			"library_id":  library.ID,
		}
		resp := tc.makeRequest("POST", "/api/v1/albums", payload)
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		// Test missing required fields
		payload = map[string]interface{}{
			"description": "Test",
		}
		resp = tc.makeRequest("POST", "/api/v1/albums", payload)
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		// Test name too long
		payload = map[string]interface{}{
			"name":        string(make([]byte, 101)), // 101 characters
			"description": "Test",
			"library_id":  library.ID,
		}
		resp = tc.makeRequest("POST", "/api/v1/albums", payload)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
	})

	t.Run("Get Albums", func(t *testing.T) {
		// Create test albums
		album1 := tc.createTestAlbum("Album 1", "First album", library.ID)
		album2 := tc.createTestAlbum("Album 2", "Second album", library.ID)

		resp := tc.makeRequest("GET", "/api/v1/albums", nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var albums []TestAlbum
		json.Unmarshal(resp.Body.Bytes(), &albums)

		assert.GreaterOrEqual(t, len(albums), 2)

		// Find our albums
		found1, found2 := false, false
		for _, album := range albums {
			if album.ID == album1.ID {
				found1 = true
				assert.Equal(t, "Album 1", album.Name)
			}
			if album.ID == album2.ID {
				found2 = true
				assert.Equal(t, "Album 2", album.Name)
			}
		}
		assert.True(t, found1, "Album 1 not found")
		assert.True(t, found2, "Album 2 not found")
	})

	t.Run("Get Albums - Filter by Library", func(t *testing.T) {
		// Create albums in different libraries
		tc.createTestAlbum("Library Album", "In main library", library.ID)
		tc.createTestAlbum("Other Album", "In other library", otherLibrary.ID)

		// Filter by library
		resp := tc.makeRequest("GET", fmt.Sprintf("/api/v1/albums?library_id=%s", library.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var albums []TestAlbum
		json.Unmarshal(resp.Body.Bytes(), &albums)

		// All albums should be from the requested library
		for _, album := range albums {
			assert.Equal(t, library.ID, album.LibraryID)
		}
	})

	t.Run("Get Album by ID", func(t *testing.T) {
		createdAlbum := tc.createTestAlbum("Single Album", "Test album", library.ID)

		resp := tc.makeRequest("GET", fmt.Sprintf("/api/v1/albums/%s", createdAlbum.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var album TestAlbum
		json.Unmarshal(resp.Body.Bytes(), &album)

		assert.Equal(t, createdAlbum.ID, album.ID)
		assert.Equal(t, createdAlbum.Name, album.Name)
		assert.Equal(t, createdAlbum.Description, album.Description)
		assert.Equal(t, createdAlbum.LibraryID, album.LibraryID)
	})

	t.Run("Get Album by ID - Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		resp := tc.makeRequest("GET", fmt.Sprintf("/api/v1/albums/%s", nonExistentID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Album not found", response["error"])
	})

	t.Run("Update Album", func(t *testing.T) {
		album := tc.createTestAlbum("Original Album", "Original description", library.ID)

		payload := map[string]interface{}{
			"name":        "Updated Album",
			"description": "Updated description",
		}

		resp := tc.makeRequest("PUT", fmt.Sprintf("/api/v1/albums/%s", album.ID), payload)
		assert.Equal(t, http.StatusOK, resp.Code)

		var updatedAlbum TestAlbum
		json.Unmarshal(resp.Body.Bytes(), &updatedAlbum)

		assert.Equal(t, album.ID, updatedAlbum.ID)
		assert.Equal(t, "Updated Album", updatedAlbum.Name)
		assert.Equal(t, "Updated description", updatedAlbum.Description)
		assert.Equal(t, album.LibraryID, updatedAlbum.LibraryID) // Should remain unchanged
	})

	t.Run("Update Album - Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		payload := map[string]interface{}{
			"name": "New Name",
		}

		resp := tc.makeRequest("PUT", fmt.Sprintf("/api/v1/albums/%s", nonExistentID), payload)
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("Delete Album", func(t *testing.T) {
		albumToDelete := tc.createTestAlbum("Delete Me", "This will be deleted", library.ID)

		resp := tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/albums/%s", albumToDelete.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Album deleted successfully", response["message"])

		// Verify album is gone
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/albums/%s", albumToDelete.ID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("Delete Album - Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		resp := tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/albums/%s", nonExistentID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("Add Photo to Album - Success", func(t *testing.T) {
		album := tc.createTestAlbum("Photo Album", "For testing photos", library.ID)
		photo := tc.uploadTestPhoto(library.ID, "album_photo.jpg", nil, "")

		payload := map[string]interface{}{
			"photo_id": photo.ID,
			"order":    1,
		}

		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", album.ID), payload)
		assert.Equal(t, http.StatusCreated, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Photo added to album successfully", response["message"])
	})

	t.Run("Add Photo to Album - Album Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		photo := tc.uploadTestPhoto(library.ID, "orphan_photo.jpg", nil, "")

		payload := map[string]interface{}{
			"photo_id": photo.ID,
		}

		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", nonExistentID), payload)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Album not found", response["error"])
	})

	t.Run("Add Photo to Album - Photo Not Found", func(t *testing.T) {
		album := tc.createTestAlbum("Empty Album", "No photos", library.ID)
		nonExistentID := uuid.New()

		payload := map[string]interface{}{
			"photo_id": nonExistentID,
		}

		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", album.ID), payload)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Photo not found", response["error"])
	})

	t.Run("Add Photo to Album - Different Libraries", func(t *testing.T) {
		albumInLibrary1 := tc.createTestAlbum("Album in Library 1", "First library", library.ID)
		photoInLibrary2 := tc.uploadTestPhoto(otherLibrary.ID, "wrong_library.jpg", nil, "")

		payload := map[string]interface{}{
			"photo_id": photoInLibrary2.ID,
		}

		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", albumInLibrary1.ID), payload)
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Photo and album must be in the same library", response["error"])
	})

	t.Run("Add Photo to Album - Duplicate", func(t *testing.T) {
		album := tc.createTestAlbum("Duplicate Test", "Testing duplicates", library.ID)
		photo := tc.uploadTestPhoto(library.ID, "duplicate_test.jpg", nil, "")

		// Add photo first time
		payload := map[string]interface{}{
			"photo_id": photo.ID,
		}
		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", album.ID), payload)
		assert.Equal(t, http.StatusCreated, resp.Code)

		// Try to add same photo again
		resp = tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", album.ID), payload)
		assert.Equal(t, http.StatusConflict, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Photo is already in this album", response["error"])
	})

	t.Run("Remove Photo from Album - Success", func(t *testing.T) {
		album := tc.createTestAlbum("Remove Test", "Testing removal", library.ID)
		photo := tc.uploadTestPhoto(library.ID, "remove_test.jpg", nil, "")

		// Add photo to album
		payload := map[string]interface{}{
			"photo_id": photo.ID,
		}
		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", album.ID), payload)
		assert.Equal(t, http.StatusCreated, resp.Code)

		// Remove photo from album
		resp = tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/albums/%s/photos/%s", album.ID, photo.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Photo removed from album successfully", response["message"])
	})

	t.Run("Remove Photo from Album - Not in Album", func(t *testing.T) {
		album := tc.createTestAlbum("Empty Album", "No photos", library.ID)
		photo := tc.uploadTestPhoto(library.ID, "not_in_album.jpg", nil, "")

		resp := tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/albums/%s/photos/%s", album.ID, photo.ID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Photo not found in album", response["error"])
	})

	t.Run("Update Photo Order in Album", func(t *testing.T) {
		album := tc.createTestAlbum("Order Test", "Testing order", library.ID)
		photo := tc.uploadTestPhoto(library.ID, "order_test.jpg", nil, "")

		// Add photo to album
		addPayload := map[string]interface{}{
			"photo_id": photo.ID,
			"order":    1,
		}
		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", album.ID), addPayload)
		assert.Equal(t, http.StatusCreated, resp.Code)

		// Update photo order
		updatePayload := map[string]interface{}{
			"order": 5,
		}
		resp = tc.makeRequest("PUT", fmt.Sprintf("/api/v1/albums/%s/photos/%s/order", album.ID, photo.ID), updatePayload)
		assert.Equal(t, http.StatusOK, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Photo order updated successfully", response["message"])
	})

	t.Run("Update Photo Order - Not in Album", func(t *testing.T) {
		album := tc.createTestAlbum("No Photo Album", "Empty", library.ID)
		photo := tc.uploadTestPhoto(library.ID, "not_added.jpg", nil, "")

		payload := map[string]interface{}{
			"order": 3,
		}

		resp := tc.makeRequest("PUT", fmt.Sprintf("/api/v1/albums/%s/photos/%s/order", album.ID, photo.ID), payload)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Photo not found in album", response["error"])
	})

	t.Run("Album with Photos Integration", func(t *testing.T) {
		album := tc.createTestAlbum("Integration Album", "Full test", library.ID)
		photo1 := tc.uploadTestPhoto(library.ID, "integration1.jpg", nil, "")
		photo2 := tc.uploadTestPhoto(library.ID, "integration2.jpg", nil, "")

		// Add photos to album with different orders
		payload1 := map[string]interface{}{
			"photo_id": photo1.ID,
			"order":    2,
		}
		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", album.ID), payload1)
		assert.Equal(t, http.StatusCreated, resp.Code)

		payload2 := map[string]interface{}{
			"photo_id": photo2.ID,
			"order":    1,
		}
		resp = tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", album.ID), payload2)
		assert.Equal(t, http.StatusCreated, resp.Code)

		// Get album with photos included
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/albums/%s?include_photos=true", album.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var albumWithPhotos map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &albumWithPhotos)

		// Verify album has photos
		photos := albumWithPhotos["photos"].([]interface{})
		assert.Equal(t, 2, len(photos))

		// Remove one photo
		resp = tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/albums/%s/photos/%s", album.ID, photo1.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		// Verify photo was removed
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/albums/%s?include_photos=true", album.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		json.Unmarshal(resp.Body.Bytes(), &albumWithPhotos)
		photos = albumWithPhotos["photos"].([]interface{})
		assert.Equal(t, 1, len(photos))

		// Verify the remaining photo is photo2
		remainingPhoto := photos[0].(map[string]interface{})
		assert.Equal(t, photo2.ID.String(), remainingPhoto["id"])
	})
}

// TestTagEndpoints tests all tag-related endpoints
func TestTagEndpoints(t *testing.T) {
	tc := setupTestEnvironment(t)
	defer tc.cleanup()

	// Setup test data
	library := tc.createTestLibrary("Tag Library", "For tag tests")

	t.Run("Create Tag - Success", func(t *testing.T) {
		tag := tc.createTestTag("nature", "#00FF00")

		assert.NotEqual(t, uuid.Nil, tag.ID)
		assert.Equal(t, "nature", tag.Name)
		assert.Equal(t, "#00FF00", tag.Color)
		assert.False(t, tag.CreatedAt.IsZero())
		assert.False(t, tag.UpdatedAt.IsZero())
	})

	t.Run("Create Tag - Without Color", func(t *testing.T) {
		payload := map[string]interface{}{
			"name": "portrait",
		}

		resp := tc.makeRequest("POST", "/api/v1/tags", payload)
		assert.Equal(t, http.StatusCreated, resp.Code)

		var tag TestTag
		json.Unmarshal(resp.Body.Bytes(), &tag)

		assert.NotEqual(t, uuid.Nil, tag.ID)
		assert.Equal(t, "portrait", tag.Name)
		assert.Empty(t, tag.Color)
	})

	t.Run("Create Tag - Duplicate Name", func(t *testing.T) {
		// Create first tag
		tc.createTestTag("duplicate", "#FF0000")

		// Try to create another with same name
		payload := map[string]interface{}{
			"name":  "duplicate",
			"color": "#0000FF",
		}

		resp := tc.makeRequest("POST", "/api/v1/tags", payload)
		assert.Equal(t, http.StatusConflict, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "already exists")
	})

	t.Run("Create Tag - Validation Errors", func(t *testing.T) {
		// Test empty name
		payload := map[string]interface{}{
			"name": "",
		}
		resp := tc.makeRequest("POST", "/api/v1/tags", payload)
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		// Test name too long
		payload = map[string]interface{}{
			"name": string(make([]byte, 51)), // 51 characters
		}
		resp = tc.makeRequest("POST", "/api/v1/tags", payload)
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		// Test invalid color format
		payload = map[string]interface{}{
			"name":  "invalid-color",
			"color": "#GGGGGG", // Invalid hex
		}
		resp = tc.makeRequest("POST", "/api/v1/tags", payload)
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		// Test color wrong length
		payload = map[string]interface{}{
			"name":  "wrong-length",
			"color": "#FF00", // Too short
		}
		resp = tc.makeRequest("POST", "/api/v1/tags", payload)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
	})

	t.Run("Get Tags", func(t *testing.T) {
		// Create test tags
		tag1 := tc.createTestTag("landscape", "#00FF00")
		tag2 := tc.createTestTag("wildlife", "#FF0000")

		resp := tc.makeRequest("GET", "/api/v1/tags", nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var tags []TestTag
		json.Unmarshal(resp.Body.Bytes(), &tags)

		assert.GreaterOrEqual(t, len(tags), 2)

		// Find our tags
		found1, found2 := false, false
		for _, tag := range tags {
			if tag.ID == tag1.ID {
				found1 = true
				assert.Equal(t, "landscape", tag.Name)
				assert.Equal(t, "#00FF00", tag.Color)
			}
			if tag.ID == tag2.ID {
				found2 = true
				assert.Equal(t, "wildlife", tag.Name)
				assert.Equal(t, "#FF0000", tag.Color)
			}
		}
		assert.True(t, found1, "Tag 1 not found")
		assert.True(t, found2, "Tag 2 not found")
	})

	t.Run("Get Tag by ID", func(t *testing.T) {
		createdTag := tc.createTestTag("architecture", "#0000FF")

		resp := tc.makeRequest("GET", fmt.Sprintf("/api/v1/tags/%s", createdTag.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var tag TestTag
		json.Unmarshal(resp.Body.Bytes(), &tag)

		assert.Equal(t, createdTag.ID, tag.ID)
		assert.Equal(t, createdTag.Name, tag.Name)
		assert.Equal(t, createdTag.Color, tag.Color)
	})

	t.Run("Get Tag by ID - Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		resp := tc.makeRequest("GET", fmt.Sprintf("/api/v1/tags/%s", nonExistentID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Tag not found", response["error"])
	})

	t.Run("Update Tag", func(t *testing.T) {
		tag := tc.createTestTag("old-name", "#FFFFFF")

		payload := map[string]interface{}{
			"name":  "new-name",
			"color": "#000000",
		}

		resp := tc.makeRequest("PUT", fmt.Sprintf("/api/v1/tags/%s", tag.ID), payload)
		assert.Equal(t, http.StatusOK, resp.Code)

		var updatedTag TestTag
		json.Unmarshal(resp.Body.Bytes(), &updatedTag)

		assert.Equal(t, tag.ID, updatedTag.ID)
		assert.Equal(t, "new-name", updatedTag.Name)
		assert.Equal(t, "#000000", updatedTag.Color)
	})

	t.Run("Update Tag - Duplicate Name", func(t *testing.T) {
		tc.createTestTag("first-tag", "#FF0000")
		tag2 := tc.createTestTag("second-tag", "#00FF00")

		// Try to update tag2 to have the same name as the first tag
		payload := map[string]interface{}{
			"name": "first-tag",
		}

		resp := tc.makeRequest("PUT", fmt.Sprintf("/api/v1/tags/%s", tag2.ID), payload)
		assert.Equal(t, http.StatusConflict, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Contains(t, response["error"], "already exists")
	})

	t.Run("Update Tag - Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		payload := map[string]interface{}{
			"name": "new-name",
		}

		resp := tc.makeRequest("PUT", fmt.Sprintf("/api/v1/tags/%s", nonExistentID), payload)
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("Delete Tag", func(t *testing.T) {
		tagToDelete := tc.createTestTag("delete-me", "#CCCCCC")

		resp := tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/tags/%s", tagToDelete.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Tag deleted successfully", response["message"])

		// Verify tag is gone
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/tags/%s", tagToDelete.ID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("Delete Tag - Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		resp := tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/tags/%s", nonExistentID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("Add Tag to Photo - Success", func(t *testing.T) {
		tag := tc.createTestTag("photo-tag", "#AAAAAA")
		photo := tc.uploadTestPhoto(library.ID, "tagged_photo.jpg", nil, "")

		payload := map[string]interface{}{
			"photo_id": photo.ID.String(),
		}

		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/tags/%s/photos", tag.ID), payload)
		assert.Equal(t, http.StatusOK, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Tag added to photo successfully", response["message"])
	})

	t.Run("Add Tag to Photo - Tag Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		photo := tc.uploadTestPhoto(library.ID, "orphan_photo.jpg", nil, "")

		payload := map[string]interface{}{
			"photo_id": photo.ID.String(),
		}

		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/tags/%s/photos", nonExistentID), payload)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Tag not found", response["error"])
	})

	t.Run("Add Tag to Photo - Photo Not Found", func(t *testing.T) {
		tag := tc.createTestTag("orphan-tag", "#BBBBBB")
		nonExistentID := uuid.New()

		payload := map[string]interface{}{
			"photo_id": nonExistentID.String(),
		}

		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/tags/%s/photos", tag.ID), payload)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Photo not found", response["error"])
	})

	t.Run("Add Tag to Photo - Duplicate", func(t *testing.T) {
		tag := tc.createTestTag("duplicate-tag", "#DDDDDD")
		photo := tc.uploadTestPhoto(library.ID, "duplicate_tag_photo.jpg", nil, "")

		payload := map[string]interface{}{
			"photo_id": photo.ID.String(),
		}

		// Add tag first time
		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/tags/%s/photos", tag.ID), payload)
		assert.Equal(t, http.StatusOK, resp.Code)

		// Try to add same tag again
		resp = tc.makeRequest("POST", fmt.Sprintf("/api/v1/tags/%s/photos", tag.ID), payload)
		assert.Equal(t, http.StatusConflict, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Tag already associated with this photo", response["error"])
	})

	t.Run("Add Tag to Photo - Invalid Photo ID", func(t *testing.T) {
		tag := tc.createTestTag("invalid-photo-tag", "#EEEEEE")

		payload := map[string]interface{}{
			"photo_id": "invalid-uuid",
		}

		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/tags/%s/photos", tag.ID), payload)
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Invalid photo_id", response["error"])
	})

	t.Run("Remove Tag from Photo - Success", func(t *testing.T) {
		tag := tc.createTestTag("remove-tag", "#333333")
		photo := tc.uploadTestPhoto(library.ID, "remove_tag_photo.jpg", nil, "")

		// Add tag to photo first
		addPayload := map[string]interface{}{
			"photo_id": photo.ID.String(),
		}
		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/tags/%s/photos", tag.ID), addPayload)
		assert.Equal(t, http.StatusOK, resp.Code)

		// Remove tag from photo
		resp = tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/tags/%s/photos/%s", tag.ID, photo.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Tag removed from photo successfully", response["message"])
	})

	t.Run("Remove Tag from Photo - Not Associated", func(t *testing.T) {
		tag := tc.createTestTag("unassociated-tag", "#444444")
		photo := tc.uploadTestPhoto(library.ID, "unassociated_photo.jpg", nil, "")

		resp := tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/tags/%s/photos/%s", tag.ID, photo.ID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Tag not found on photo", response["error"])
	})

	t.Run("Get Tag Stats", func(t *testing.T) {
		tag := tc.createTestTag("stats-tag", "#555555")

		// Initially should have no photos
		resp := tc.makeRequest("GET", fmt.Sprintf("/api/v1/tags/%s/stats", tag.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var stats map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &stats)

		assert.Equal(t, tag.ID.String(), stats["tag_id"])
		assert.Equal(t, tag.Name, stats["tag_name"])
		assert.Equal(t, float64(0), stats["photo_count"])
		assert.Equal(t, 0, len(stats["libraries"].([]interface{})))

		// Add some photos with the tag
		photo1 := tc.uploadTestPhoto(library.ID, "stats1.jpg", nil, "")
		photo2 := tc.uploadTestPhoto(library.ID, "stats2.jpg", nil, "")

		// Add tag to photos
		addPayload1 := map[string]interface{}{
			"photo_id": photo1.ID.String(),
		}
		resp = tc.makeRequest("POST", fmt.Sprintf("/api/v1/tags/%s/photos", tag.ID), addPayload1)
		assert.Equal(t, http.StatusOK, resp.Code)

		addPayload2 := map[string]interface{}{
			"photo_id": photo2.ID.String(),
		}
		resp = tc.makeRequest("POST", fmt.Sprintf("/api/v1/tags/%s/photos", tag.ID), addPayload2)
		assert.Equal(t, http.StatusOK, resp.Code)

		// Check stats again
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/tags/%s/stats", tag.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		json.Unmarshal(resp.Body.Bytes(), &stats)

		assert.Equal(t, tag.ID.String(), stats["tag_id"])
		assert.Equal(t, tag.Name, stats["tag_name"])
		assert.Equal(t, float64(2), stats["photo_count"])

		libraries := stats["libraries"].([]interface{})
		assert.Equal(t, 1, len(libraries))

		libraryStats := libraries[0].(map[string]interface{})
		assert.Equal(t, library.ID.String(), libraryStats["library_id"])
		assert.Equal(t, library.Name, libraryStats["library_name"])
		assert.Equal(t, float64(2), libraryStats["photo_count"])
	})

	t.Run("Get Tag Stats - Multi-Library", func(t *testing.T) {
		otherLibrary := tc.createTestLibrary("Other Tag Library", "Another library")
		tag := tc.createTestTag("multi-lib-tag", "#666666")

		// Add photos in different libraries
		photo1 := tc.uploadTestPhoto(library.ID, "multilib1.jpg", nil, "")
		photo2 := tc.uploadTestPhoto(otherLibrary.ID, "multilib2.jpg", nil, "")

		// Add tag to both photos
		addPayload1 := map[string]interface{}{
			"photo_id": photo1.ID.String(),
		}
		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/tags/%s/photos", tag.ID), addPayload1)
		assert.Equal(t, http.StatusOK, resp.Code)

		addPayload2 := map[string]interface{}{
			"photo_id": photo2.ID.String(),
		}
		resp = tc.makeRequest("POST", fmt.Sprintf("/api/v1/tags/%s/photos", tag.ID), addPayload2)
		assert.Equal(t, http.StatusOK, resp.Code)

		// Check stats
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/tags/%s/stats", tag.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var stats map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &stats)

		assert.Equal(t, float64(2), stats["photo_count"])

		libraries := stats["libraries"].([]interface{})
		assert.Equal(t, 2, len(libraries))

		// Verify both libraries are represented
		libraryNames := make(map[string]bool)
		for _, lib := range libraries {
			libMap := lib.(map[string]interface{})
			libraryNames[libMap["library_name"].(string)] = true
			assert.Equal(t, float64(1), libMap["photo_count"])
		}
		assert.True(t, libraryNames[library.Name])
		assert.True(t, libraryNames[otherLibrary.Name])
	})

	t.Run("Get Tag Stats - Not Found", func(t *testing.T) {
		nonExistentID := uuid.New()
		resp := tc.makeRequest("GET", fmt.Sprintf("/api/v1/tags/%s/stats", nonExistentID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})

	t.Run("Tag Photo Integration", func(t *testing.T) {
		// Test the full workflow: create tag, upload photo with tags, verify relationships
		photo := tc.uploadTestPhoto(library.ID, "integration_photo.jpg", nil, "sunset,golden-hour")

		// Verify tags were created during upload
		resp := tc.makeRequest("GET", "/api/v1/tags", nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var tags []TestTag
		json.Unmarshal(resp.Body.Bytes(), &tags)

		// Find the created tags
		var sunsetTag, goldenHourTag *TestTag
		for _, tag := range tags {
			if tag.Name == "sunset" {
				sunsetTag = &tag
			}
			if tag.Name == "golden-hour" {
				goldenHourTag = &tag
			}
		}
		assert.NotNil(t, sunsetTag, "sunset tag should be created")
		assert.NotNil(t, goldenHourTag, "golden-hour tag should be created")

		// Get photo with tags included
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/photos/%s?include_tags=true", photo.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var photoWithTags map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &photoWithTags)

		photoTags := photoWithTags["tags"].([]interface{})
		assert.Equal(t, 2, len(photoTags))

		// Verify tag names
		tagNames := make(map[string]bool)
		for _, tag := range photoTags {
			tagMap := tag.(map[string]interface{})
			tagNames[tagMap["name"].(string)] = true
		}
		assert.True(t, tagNames["sunset"])
		assert.True(t, tagNames["golden-hour"])
	})
}

// TestIntegrationWorkflows tests comprehensive end-to-end workflows
func TestIntegrationWorkflows(t *testing.T) {
	tc := setupTestEnvironment(t)
	defer tc.cleanup()

	t.Run("Complete Photo Management Workflow", func(t *testing.T) {
		// Step 1: Create libraries
		library1 := tc.createTestLibrary("Personal", "Personal photos")
		library2 := tc.createTestLibrary("Work", "Work-related photos")

		// Step 2: Create albums in each library
		album1 := tc.createTestAlbum("Vacation", "Summer vacation photos", library1.ID)
		album2 := tc.createTestAlbum("Projects", "Work project photos", library2.ID)

		// Step 3: Create tags
		tc.createTestTag("outdoor", "#00FF00")
		tc.createTestTag("meeting", "#0000FF")

		// Step 4: Upload photos with different characteristics
		rating4 := 4
		rating5 := 5
		photo1 := tc.uploadTestPhoto(library1.ID, "vacation1.jpg", &rating4, "outdoor,beach")
		photo2 := tc.uploadTestPhoto(library1.ID, "vacation2.jpg", &rating5, "outdoor,sunset")
		photo3 := tc.uploadTestPhoto(library2.ID, "meeting1.jpg", nil, "meeting,indoor")

		// Step 5: Add photos to albums
		addPayload1 := map[string]interface{}{
			"photo_id": photo1.ID,
			"order":    1,
		}
		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", album1.ID), addPayload1)
		assert.Equal(t, http.StatusCreated, resp.Code)

		addPayload2 := map[string]interface{}{
			"photo_id": photo2.ID,
			"order":    2,
		}
		resp = tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", album1.ID), addPayload2)
		assert.Equal(t, http.StatusCreated, resp.Code)

		addPayload3 := map[string]interface{}{
			"photo_id": photo3.ID,
			"order":    1,
		}
		resp = tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", album2.ID), addPayload3)
		assert.Equal(t, http.StatusCreated, resp.Code)

		// Step 6: Copy photo from library1 to library2
		copyPayload := map[string]interface{}{
			"library_id": library2.ID,
		}
		resp = tc.makeRequest("POST", fmt.Sprintf("/api/v1/photos/%s/copy", photo1.ID), copyPayload)
		assert.Equal(t, http.StatusCreated, resp.Code)

		var copyResponse map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &copyResponse)
		copiedPhotoData := copyResponse["copied_photo"].(map[string]interface{})
		copiedPhotoID := copiedPhotoData["id"].(string)

		// Step 7: Verify library statistics
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/libraries/%s/stats", library1.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var stats1 map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &stats1)
		assert.Equal(t, float64(2), stats1["photo_count"])
		assert.Equal(t, float64(1), stats1["album_count"])

		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/libraries/%s/stats", library2.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var stats2 map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &stats2)
		assert.Equal(t, float64(2), stats2["photo_count"]) // original + copied
		assert.Equal(t, float64(1), stats2["album_count"])

		// Step 8: Get photos with various filters
		// Filter by library
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/photos?library_id=%s", library1.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var photoResponse map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &photoResponse)
		photos := photoResponse["photos"].([]interface{})
		assert.Equal(t, 2, len(photos))

		// Filter by rating
		resp = tc.makeRequest("GET", "/api/v1/photos?rating=5", nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		json.Unmarshal(resp.Body.Bytes(), &photoResponse)
		photos = photoResponse["photos"].([]interface{})
		assert.GreaterOrEqual(t, len(photos), 1)

		// Step 9: Update photo rating and verify
		updatePayload := map[string]interface{}{
			"rating": 3,
		}
		resp = tc.makeRequest("PUT", fmt.Sprintf("/api/v1/photos/%s", photo3.ID), updatePayload)
		assert.Equal(t, http.StatusOK, resp.Code)

		// Step 10: Get album with photos and verify ordering
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/albums/%s?include_photos=true", album1.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var albumWithPhotos map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &albumWithPhotos)
		albumPhotos := albumWithPhotos["photos"].([]interface{})
		assert.Equal(t, 2, len(albumPhotos))

		// Step 11: Remove photo from album and verify
		resp = tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/albums/%s/photos/%s", album1.ID, photo2.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/albums/%s?include_photos=true", album1.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		json.Unmarshal(resp.Body.Bytes(), &albumWithPhotos)
		albumPhotos = albumWithPhotos["photos"].([]interface{})
		assert.Equal(t, 1, len(albumPhotos))

		// Step 12: Delete a photo and verify cleanup
		filePath := photo2.FilePath
		resp = tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/photos/%s", photo2.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		// Verify file is removed
		_, err := os.Stat(filePath)
		assert.True(t, os.IsNotExist(err))

		// Step 13: Delete library and verify cascade
		// photo1 is still there at this point
		resp = tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/libraries/%s", library1.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		// Verify library is gone
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/libraries/%s", library1.ID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		// Verify albums in library are gone
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/albums/%s", album1.ID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		// Verify photos in library are gone
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/photos/%s", photo1.ID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		// Verify copied photo in library2 still exists
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/photos/%s", copiedPhotoID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)
	})

	t.Run("Cross-Library Constraint Validation", func(t *testing.T) {
		// Create two separate libraries
		library1 := tc.createTestLibrary("Library A", "First library")
		library2 := tc.createTestLibrary("Library B", "Second library")

		// Create albums in each library
		album1 := tc.createTestAlbum("Album A", "In library A", library1.ID)
		album2 := tc.createTestAlbum("Album B", "In library B", library2.ID)

		// Upload photos to each library
		photo1 := tc.uploadTestPhoto(library1.ID, "photoA.jpg", nil, "")
		photo2 := tc.uploadTestPhoto(library2.ID, "photoB.jpg", nil, "")

		// Test 1: Try to add photo from library1 to album in library2 (should fail)
		payload := map[string]interface{}{
			"photo_id": photo1.ID,
		}
		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", album2.ID), payload)
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Photo and album must be in the same library", response["error"])

		// Test 2: Try to add photo from library2 to album in library1 (should fail)
		payload = map[string]interface{}{
			"photo_id": photo2.ID,
		}
		resp = tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", album1.ID), payload)
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Photo and album must be in the same library", response["error"])

		// Test 3: Verify correct library associations work
		payload = map[string]interface{}{
			"photo_id": photo1.ID,
		}
		resp = tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", album1.ID), payload)
		assert.Equal(t, http.StatusCreated, resp.Code)

		payload = map[string]interface{}{
			"photo_id": photo2.ID,
		}
		resp = tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", album2.ID), payload)
		assert.Equal(t, http.StatusCreated, resp.Code)

		// Test 4: Copy photo and verify it can be added to target library's album
		copyPayload := map[string]interface{}{
			"library_id": library2.ID,
		}
		resp = tc.makeRequest("POST", fmt.Sprintf("/api/v1/photos/%s/copy", photo1.ID), copyPayload)
		assert.Equal(t, http.StatusCreated, resp.Code)

		var copyResponse map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &copyResponse)
		copiedPhotoData := copyResponse["copied_photo"].(map[string]interface{})
		copiedPhotoID := copiedPhotoData["id"].(string)

		// Now the copied photo should be addable to album2
		payload = map[string]interface{}{
			"photo_id": copiedPhotoID,
		}
		resp = tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", album2.ID), payload)
		assert.Equal(t, http.StatusCreated, resp.Code)
	})

	t.Run("Tag Association Across Libraries", func(t *testing.T) {
		// Create libraries and upload photos
		library1 := tc.createTestLibrary("Tag Lib 1", "First tag library")
		library2 := tc.createTestLibrary("Tag Lib 2", "Second tag library")

		photo1 := tc.uploadTestPhoto(library1.ID, "tag_photo1.jpg", nil, "")
		photo2 := tc.uploadTestPhoto(library2.ID, "tag_photo2.jpg", nil, "")

		// Create a tag
		tag := tc.createTestTag("global-tag", "#FF00FF")

		// Add the same tag to photos in different libraries
		addTagPayload1 := map[string]interface{}{
			"photo_id": photo1.ID.String(),
		}
		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/tags/%s/photos", tag.ID), addTagPayload1)
		assert.Equal(t, http.StatusOK, resp.Code)

		addTagPayload2 := map[string]interface{}{
			"photo_id": photo2.ID.String(),
		}
		resp = tc.makeRequest("POST", fmt.Sprintf("/api/v1/tags/%s/photos", tag.ID), addTagPayload2)
		assert.Equal(t, http.StatusOK, resp.Code)

		// Get tag stats and verify multi-library breakdown
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/tags/%s/stats", tag.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var stats map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &stats)

		assert.Equal(t, float64(2), stats["photo_count"])

		libraries := stats["libraries"].([]interface{})
		assert.Equal(t, 2, len(libraries))

		// Verify both libraries are represented with correct counts
		libraryStats := make(map[string]float64)
		for _, lib := range libraries {
			libMap := lib.(map[string]interface{})
			libraryStats[libMap["library_name"].(string)] = libMap["photo_count"].(float64)
		}
		assert.Equal(t, float64(1), libraryStats["Tag Lib 1"])
		assert.Equal(t, float64(1), libraryStats["Tag Lib 2"])

		// Filter photos by tag across all libraries
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/photos?tag=%s", tag.Name), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var photoResponse map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &photoResponse)
		photos := photoResponse["photos"].([]interface{})
		assert.Equal(t, 2, len(photos))

		// Verify photos are from different libraries
		libraryIDs := make(map[string]bool)
		for _, photo := range photos {
			photoMap := photo.(map[string]interface{})
			libraryIDs[photoMap["library_id"].(string)] = true
		}
		assert.True(t, libraryIDs[library1.ID.String()])
		assert.True(t, libraryIDs[library2.ID.String()])
	})

	t.Run("Data Consistency and Cleanup", func(t *testing.T) {
		// Create a complete setup
		library := tc.createTestLibrary("Cleanup Library", "For cleanup testing")
		album := tc.createTestAlbum("Cleanup Album", "For cleanup testing", library.ID)
		tag := tc.createTestTag("cleanup-tag", "#CCCCCC")
		photo := tc.uploadTestPhoto(library.ID, "cleanup_photo.jpg", nil, "cleanup-tag")

		// Add photo to album
		addPayload := map[string]interface{}{
			"photo_id": photo.ID,
		}
		resp := tc.makeRequest("POST", fmt.Sprintf("/api/v1/albums/%s/photos", album.ID), addPayload)
		assert.Equal(t, http.StatusCreated, resp.Code)

		// Verify tag is already associated with photo (from upload)
		tagPayload := map[string]interface{}{
			"photo_id": photo.ID.String(),
		}
		resp = tc.makeRequest("POST", fmt.Sprintf("/api/v1/tags/%s/photos", tag.ID), tagPayload)
		assert.Equal(t, http.StatusConflict, resp.Code) // Tag already exists

		// Verify all relationships exist
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/albums/%s?include_photos=true", album.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var albumData map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &albumData)
		albumPhotos := albumData["photos"].([]interface{})
		assert.Equal(t, 1, len(albumPhotos))

		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/photos/%s?include_tags=true", photo.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var photoData map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &photoData)
		photoTags := photoData["tags"].([]interface{})
		assert.GreaterOrEqual(t, len(photoTags), 1)

		// Delete photo and verify all relationships are cleaned up
		filePath := photo.FilePath
		resp = tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/photos/%s", photo.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		// Verify photo is gone
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/photos/%s", photo.ID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		// Verify file is deleted
		_, err := os.Stat(filePath)
		assert.True(t, os.IsNotExist(err))

		// Verify album no longer has the photo
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/albums/%s?include_photos=true", album.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		// Create a fresh map to avoid data from previous requests
		var freshAlbumData map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &freshAlbumData)

		// When there are no photos, the API omits the "photos" field entirely
		// This is expected behavior (empty slices get omitted in JSON)
		photosField, exists := freshAlbumData["photos"]

		if exists {
			albumPhotos = photosField.([]interface{})
			assert.Equal(t, 0, len(albumPhotos))
		} else {
			// Photos field doesn't exist = no photos, which is correct
			assert.False(t, exists)
		}

		// Verify tag still exists but has no photos
		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/tags/%s/stats", tag.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		var tagStats map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &tagStats)
		assert.Equal(t, float64(0), tagStats["photo_count"])

		// Delete tag and verify cleanup
		resp = tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/tags/%s", tag.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/tags/%s", tag.ID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		// Delete album and verify cleanup
		resp = tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/albums/%s", album.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/albums/%s", album.ID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		// Finally, delete library and verify all cleanup
		libraryPath := library.Images
		resp = tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/libraries/%s", library.ID), nil)
		assert.Equal(t, http.StatusOK, resp.Code)

		resp = tc.makeRequest("GET", fmt.Sprintf("/api/v1/libraries/%s", library.ID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)

		// Verify library directory is removed
		_, err = os.Stat(libraryPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("Error Recovery and Validation", func(t *testing.T) {
		library := tc.createTestLibrary("Error Library", "For error testing")

		// Test malformed requests
		resp := tc.makeRequest("POST", "/api/v1/libraries", map[string]interface{}{
			"invalid": "data",
		})
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		// Test invalid UUIDs
		resp = tc.makeRequest("GET", "/api/v1/photos/not-a-uuid", nil)
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		// Test invalid file uploads
		fields := map[string]string{
			"library_id": library.ID.String(),
		}
		files := map[string][]byte{
			"photo": []byte("not-an-image"),
		}
		resp = tc.makeMultipartRequest("/api/v1/photos/upload", fields, files)
		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var response map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &response)
		assert.Equal(t, "Invalid image file", response["error"])

		// Test operations on non-existent resources
		nonExistentID := uuid.New()

		resp = tc.makeRequest("PUT", fmt.Sprintf("/api/v1/photos/%s", nonExistentID), map[string]interface{}{
			"rating": 5,
		})
		assert.Equal(t, http.StatusNotFound, resp.Code)

		resp = tc.makeRequest("DELETE", fmt.Sprintf("/api/v1/albums/%s", nonExistentID), nil)
		assert.Equal(t, http.StatusNotFound, resp.Code)
	})
}
