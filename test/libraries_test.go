package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

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
