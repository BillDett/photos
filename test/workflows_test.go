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
