package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

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
