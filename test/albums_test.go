package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

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
