package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

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
