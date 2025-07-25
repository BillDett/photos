package main

import (
	"fmt"
	"log"
	"photo-library-server/config"
	"photo-library-server/database"
	"photo-library-server/handlers"
	"photo-library-server/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Initialize database
	sqliteDB, err := database.NewSQLiteDB(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer sqliteDB.Close()

	// Run migrations
	if err := sqliteDB.Migrate(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Create indexes for better performance
	if err := sqliteDB.CreateIndexes(); err != nil {
		log.Printf("Warning: Failed to create indexes: %v", err)
	}

	// Initialize Gin router
	if gin.Mode() == gin.DebugMode {
		gin.SetMode(gin.ReleaseMode) // Use release mode for better performance
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.CORSMiddleware())

	// Initialize handlers
	libraryHandler := handlers.NewLibraryHandler(sqliteDB.GetDB())
	albumHandler := handlers.NewAlbumHandler(sqliteDB.GetDB())
	photoHandler := handlers.NewPhotoHandler(sqliteDB.GetDB(), cfg)
	tagHandler := handlers.NewTagHandler(sqliteDB.GetDB())

	// API routes
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
			photos.GET("/:id/file", photoHandler.ServePhoto) // Serve actual photo file
			photos.POST("/:id/copy", photoHandler.CopyPhoto) // Copy photo to same or different library
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

	// API documentation endpoint
	router.GET("/api", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"service": "Photo Library Management Server",
			"version": "1.0.0",
			"endpoints": gin.H{
				"libraries": gin.H{
					"POST   /api/v1/libraries":           "Create a new library",
					"GET    /api/v1/libraries":           "Get all libraries",
					"GET    /api/v1/libraries/:id":       "Get a specific library",
					"PUT    /api/v1/libraries/:id":       "Update a library",
					"DELETE /api/v1/libraries/:id":       "Delete a library",
					"GET    /api/v1/libraries/:id/stats": "Get library statistics",
				},
				"albums": gin.H{
					"POST   /api/v1/albums":                            "Create a new album",
					"GET    /api/v1/albums":                            "Get all albums",
					"GET    /api/v1/albums/:id":                        "Get a specific album",
					"PUT    /api/v1/albums/:id":                        "Update an album",
					"DELETE /api/v1/albums/:id":                        "Delete an album",
					"POST   /api/v1/albums/:id/photos":                 "Add photo to album",
					"DELETE /api/v1/albums/:id/photos/:photo_id":       "Remove photo from album",
					"PUT    /api/v1/albums/:id/photos/:photo_id/order": "Update photo order in album",
				},
				"photos": gin.H{
					"POST   /api/v1/photos/upload":   "Upload a new photo",
					"GET    /api/v1/photos":          "Get all photos with filters",
					"GET    /api/v1/photos/:id":      "Get a specific photo",
					"PUT    /api/v1/photos/:id":      "Update photo metadata",
					"DELETE /api/v1/photos/:id":      "Delete a photo",
					"GET    /api/v1/photos/:id/file": "Serve the actual photo file",
					"POST   /api/v1/photos/:id/copy": "Copy photo to same or different library",
				},
				"tags": gin.H{
					"POST   /api/v1/tags":                      "Create a new tag",
					"GET    /api/v1/tags":                      "Get all tags",
					"GET    /api/v1/tags/:id":                  "Get a specific tag",
					"PUT    /api/v1/tags/:id":                  "Update a tag",
					"DELETE /api/v1/tags/:id":                  "Delete a tag",
					"POST   /api/v1/tags/:id/photos":           "Add tag to photo",
					"DELETE /api/v1/tags/:id/photos/:photo_id": "Remove tag from photo",
					"GET    /api/v1/tags/:id/stats":            "Get tag statistics",
				},
				"health": gin.H{
					"GET /health": "Health check endpoint",
				},
			},
		})
	})

	// Start server
	address := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	log.Printf("Starting Photo Library Server on %s", address)
	log.Printf("Database: %s", cfg.DatabasePath)
	log.Printf("Max file size: %d bytes (%.1f MB)", cfg.MaxFileSize, float64(cfg.MaxFileSize)/(1024*1024))
	log.Printf("Images stored in library-specific directories")
	log.Printf("API documentation available at: http://%s/api", address)

	if err := router.Run(address); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
