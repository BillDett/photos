package handlers

import (
	"net/http"
	"photo-library-server/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TagHandler handles tag-related HTTP requests
type TagHandler struct {
	db *gorm.DB
}

// NewTagHandler creates a new tag handler
func NewTagHandler(db *gorm.DB) *TagHandler {
	return &TagHandler{db: db}
}

// CreateTag creates a new tag
func (h *TagHandler) CreateTag(c *gin.Context) {
	var req struct {
		Name  string `json:"name" binding:"required,min=1,max=50"`
		Color string `json:"color" binding:"omitempty,len=7"` // hex color like #FF0000
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if tag with same name already exists
	var existingTag models.Tag
	if err := h.db.Where("name = ?", req.Name).First(&existingTag).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Tag with this name already exists"})
		return
	}

	tag := models.Tag{
		Name:  req.Name,
		Color: req.Color,
	}

	if err := h.db.Create(&tag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create tag"})
		return
	}

	c.JSON(http.StatusCreated, tag)
}

// GetTags returns all tags
func (h *TagHandler) GetTags(c *gin.Context) {
	var tags []models.Tag

	query := h.db.Model(&models.Tag{})

	// Optional: include photo count
	if c.Query("include_count") == "true" {
		// Use a subquery to count photos for each tag
		query = query.Select("tags.*, (SELECT COUNT(*) FROM photo_tags WHERE photo_tags.tag_id = tags.id) as photo_count")
	}

	// Optional: include photos
	if c.Query("include_photos") == "true" {
		query = query.Preload("Photos")
	}

	if err := query.Find(&tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tags"})
		return
	}

	c.JSON(http.StatusOK, tags)
}

// GetTag returns a specific tag by ID
func (h *TagHandler) GetTag(c *gin.Context) {
	tagID := c.Param("id")

	id, err := uuid.Parse(tagID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}

	var tag models.Tag
	query := h.db.Model(&models.Tag{})

	// Optional: include photos
	if c.Query("include_photos") == "true" {
		query = query.Preload("Photos")
	}

	if err := query.First(&tag, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tag"})
		return
	}

	c.JSON(http.StatusOK, tag)
}

// UpdateTag updates a tag
func (h *TagHandler) UpdateTag(c *gin.Context) {
	tagID := c.Param("id")

	id, err := uuid.Parse(tagID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}

	var req struct {
		Name  string `json:"name" binding:"required,min=1,max=50"`
		Color string `json:"color" binding:"omitempty,len=7"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var tag models.Tag
	if err := h.db.First(&tag, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tag"})
		return
	}

	// Check if another tag with same name exists
	var existingTag models.Tag
	if err := h.db.Where("name = ? AND id != ?", req.Name, id).First(&existingTag).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Tag with this name already exists"})
		return
	}

	// Update fields
	tag.Name = req.Name
	tag.Color = req.Color

	if err := h.db.Save(&tag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update tag"})
		return
	}

	c.JSON(http.StatusOK, tag)
}

// DeleteTag deletes a tag and all its relationships
func (h *TagHandler) DeleteTag(c *gin.Context) {
	tagID := c.Param("id")

	id, err := uuid.Parse(tagID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}

	var tag models.Tag
	if err := h.db.First(&tag, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tag"})
		return
	}

	// Use transaction to clean up relationships
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete photo_tags relationships
	if err := tx.Where("tag_id = ?", id).Delete(&models.PhotoTag{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove tag from photos"})
		return
	}

	// Delete the tag itself
	if err := tx.Delete(&tag).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete tag"})
		return
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"message": "Tag deleted successfully"})
}

// AddTagToPhoto adds a tag to a photo
func (h *TagHandler) AddTagToPhoto(c *gin.Context) {
	tagID := c.Param("id")

	id, err := uuid.Parse(tagID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}

	var req struct {
		PhotoID uuid.UUID `json:"photo_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify tag exists
	var tag models.Tag
	if err := h.db.First(&tag, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify tag"})
		return
	}

	// Verify photo exists
	var photo models.Photo
	if err := h.db.First(&photo, req.PhotoID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Photo not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify photo"})
		return
	}

	// Check if relationship already exists
	var existingRelation models.PhotoTag
	if err := h.db.Where("tag_id = ? AND photo_id = ?", id, req.PhotoID).First(&existingRelation).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Tag is already applied to this photo"})
		return
	}

	photoTag := models.PhotoTag{
		TagID:   id,
		PhotoID: req.PhotoID,
	}

	if err := h.db.Create(&photoTag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add tag to photo"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Tag added to photo successfully"})
}

// RemoveTagFromPhoto removes a tag from a photo
func (h *TagHandler) RemoveTagFromPhoto(c *gin.Context) {
	tagID := c.Param("id")
	photoID := c.Param("photo_id")

	tagUUID, err := uuid.Parse(tagID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}

	photoUUID, err := uuid.Parse(photoID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid photo ID"})
		return
	}

	result := h.db.Where("tag_id = ? AND photo_id = ?", tagUUID, photoUUID).Delete(&models.PhotoTag{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove tag from photo"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found on photo"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tag removed from photo successfully"})
}

// GetTagStats returns statistics for a tag
func (h *TagHandler) GetTagStats(c *gin.Context) {
	tagID := c.Param("id")

	id, err := uuid.Parse(tagID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}

	// Check if tag exists
	var tag models.Tag
	if err := h.db.First(&tag, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tag"})
		return
	}

	// Get library breakdown
	type LibraryStats struct {
		LibraryID   uuid.UUID `json:"library_id"`
		LibraryName string    `json:"library_name"`
		PhotoCount  int64     `json:"photo_count"`
	}

	stats := struct {
		TagID      uuid.UUID      `json:"tag_id"`
		TagName    string         `json:"tag_name"`
		PhotoCount int64          `json:"photo_count"`
		Libraries  []LibraryStats `json:"libraries"`
	}{
		TagID:   tag.ID,
		TagName: tag.Name,
	}

	// Count total photos with this tag
	h.db.Model(&models.PhotoTag{}).Where("tag_id = ?", id).Count(&stats.PhotoCount)

	var libraryStats []LibraryStats
	h.db.Table("libraries").
		Select("libraries.id as library_id, libraries.name as library_name, COUNT(photo_tags.photo_id) as photo_count").
		Joins("JOIN photos ON libraries.id = photos.library_id").
		Joins("JOIN photo_tags ON photos.id = photo_tags.photo_id").
		Where("photo_tags.tag_id = ?", id).
		Group("libraries.id, libraries.name").
		Find(&libraryStats)

	stats.Libraries = libraryStats

	c.JSON(http.StatusOK, stats)
}
