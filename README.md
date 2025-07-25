# Photo Library Management Server

A comprehensive Go-based server for managing photo libraries with support for albums, tags, ratings, and multiple libraries. Built with a proper database abstraction layer for future scalability.

Full disclaimer: Cursor wrote this code, not me...

## Features

- **Multiple Libraries**: Organize photos into separate libraries with unique names and storage paths
- **Library-Specific Storage**: Each library has its own isolated file storage directory
- **Album Management**: Create albums within libraries to organize photos
- **Photo Upload**: Upload photos with automatic metadata extraction (dimensions, file size, etc.)
- **Photo Copy**: Copy photos within the same library or to different libraries with unique identifiers
- **Tagging System**: Apply textual tags to photos for easy organization and search
- **Rating System**: Rate photos from 0-5 stars
- **RESTful API**: Complete CRUD operations for all entities
- **Database Abstraction**: SQLite by default, easily extensible to PostgreSQL
- **File Management**: Automatic file storage with unique naming to prevent conflicts
- **Statistics**: Get detailed statistics for libraries and tags

## Requirements

- Go 1.21 or higher
- SQLite (automatically handled)

## Installation

1. Clone the repository and navigate to the project directory:
   ```bash
   cd photo-library-server
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Run the server:
   ```bash
   go run main.go
   ```

The server will start on `http://localhost:8080` by default.

## Configuration

The server can be configured using environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `HOST` | `localhost` | Server host |
| `DATABASE_PATH` | `./photo_library.db` | SQLite database file path |
| `MAX_FILE_SIZE` | `52428800` (50MB) | Maximum upload file size in bytes |

Example:
```bash
export PORT=3000
export MAX_FILE_SIZE=104857600  # 100MB
go run main.go
```

## API Documentation

### Base URL
All API endpoints are prefixed with `/api/v1`.

### Libraries

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/libraries` | Create a new library |
| GET | `/libraries` | Get all libraries |
| GET | `/libraries/:id` | Get a specific library |
| PUT | `/libraries/:id` | Update a library |
| DELETE | `/libraries/:id` | Delete a library |
| GET | `/libraries/:id/stats` | Get library statistics |

#### Create Library
```bash
curl -X POST http://localhost:8080/api/v1/libraries \
  -H "Content-Type: application/json" \
  -d '{"name": "My Photos", "description": "Personal photo collection", "images": "./my-photos-storage"}'
```

### Albums

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/albums` | Create a new album |
| GET | `/albums` | Get all albums |
| GET | `/albums/:id` | Get a specific album |
| PUT | `/albums/:id` | Update an album |
| DELETE | `/albums/:id` | Delete an album |
| POST | `/albums/:id/photos` | Add photo to album |
| DELETE | `/albums/:id/photos/:photo_id` | Remove photo from album |
| PUT | `/albums/:id/photos/:photo_id/order` | Update photo order in album |

#### Create Album
```bash
curl -X POST http://localhost:8080/api/v1/albums \
  -H "Content-Type: application/json" \
  -d '{"name": "Vacation 2024", "description": "Summer vacation photos", "library_id": "library-uuid-here"}'
```

### Photos

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/photos/upload` | Upload a new photo |
| GET | `/photos` | Get all photos (with filters) |
| GET | `/photos/:id` | Get a specific photo |
| PUT | `/photos/:id` | Update photo metadata |
| DELETE | `/photos/:id` | Delete a photo |
| GET | `/photos/:id/file` | Serve the actual photo file |
| POST | `/photos/:id/copy` | Copy photo to same or different library |

#### Upload Photo
```bash
curl -X POST http://localhost:8080/api/v1/photos/upload \
  -F "photo=@/path/to/image.jpg" \
  -F "library_id=library-uuid-here" \
  -F "rating=4" \
  -F "tags=vacation,summer,beach"
```

#### Query Photos
```bash
# Get photos from a specific library
curl "http://localhost:8080/api/v1/photos?library_id=library-uuid-here"

# Get photos with specific rating
curl "http://localhost:8080/api/v1/photos?rating=5"

# Get photos with specific tag
curl "http://localhost:8080/api/v1/photos?tag=vacation"

# Pagination and sorting
curl "http://localhost:8080/api/v1/photos?page=2&limit=20&order_by=rating&order_dir=desc"
```

#### Copy Photo
```bash
# Copy photo to the same library
curl -X POST http://localhost:8080/api/v1/photos/photo-uuid-here/copy \
  -H "Content-Type: application/json" \
  -d '{"library_id": "same-library-uuid-here"}'

# Copy photo to a different library
curl -X POST http://localhost:8080/api/v1/photos/photo-uuid-here/copy \
  -H "Content-Type: application/json" \
  -d '{"library_id": "different-library-uuid-here"}'
```

### Tags

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/tags` | Create a new tag |
| GET | `/tags` | Get all tags |
| GET | `/tags/:id` | Get a specific tag |
| PUT | `/tags/:id` | Update a tag |
| DELETE | `/tags/:id` | Delete a tag |
| POST | `/tags/:id/photos` | Add tag to photo |
| DELETE | `/tags/:id/photos/:photo_id` | Remove tag from photo |
| GET | `/tags/:id/stats` | Get tag statistics |

#### Create Tag
```bash
curl -X POST http://localhost:8080/api/v1/tags \
  -H "Content-Type: application/json" \
  -d '{"name": "vacation", "color": "#FF6B6B"}'
```

### Health Check
```bash
curl http://localhost:8080/health
```

### API Documentation
```bash
curl http://localhost:8080/api
```

## Supported Image Formats

- JPEG (.jpg, .jpeg)
- PNG (.png)
- GIF (.gif)
- WebP (.webp)
- TIFF (.tiff, .tif)
- BMP (.bmp)

## Library Storage System

Each library has its own isolated storage directory specified by the `images` field:

- **Isolation**: Photos from different libraries are stored in separate directories
- **Unique Paths**: No two libraries can share the same storage path
- **Automatic Cleanup**: When a library is deleted, its entire storage directory is removed
- **Path Validation**: Library paths are validated to prevent security issues

### Storage Structure
```
./library1-photos/     # Library 1 images directory
├── photo1.jpg
├── photo2.png
└── ...

./library2-photos/     # Library 2 images directory  
├── vacation1.jpg
├── vacation2.jpg
└── ...
```

## Database Schema

The server uses the following main entities:

- **Libraries**: Top-level containers with unique names and storage paths
- **Albums**: Collections of photos within a library
- **Photos**: Individual photo files with metadata stored in library-specific directories
- **Tags**: Textual labels that can be applied to photos
- **PhotoTags**: Many-to-many relationship between photos and tags
- **AlbumPhotos**: Many-to-many relationship between albums and photos with ordering

## Development

### Project Structure
```
photo-library-server/
├── main.go                 # Main server file
├── config/                 # Configuration management
├── database/               # Database abstraction layer
├── handlers/               # HTTP request handlers
├── middleware/             # HTTP middleware
├── models/                 # Database models
├── go.mod                  # Go module definition
└── README.md              # This file
```

### Adding PostgreSQL Support

To add PostgreSQL support in the future:

1. Add PostgreSQL driver to `go.mod`:
   ```bash
   go get gorm.io/driver/postgres
   ```

2. Implement `PostgresDB` struct in `database/database.go`:
   ```go
   type PostgresDB struct {
       db *gorm.DB
   }
   
   func NewPostgresDB(dsn string) (*PostgresDB, error) {
       // Implementation here
   }
   ```

3. Update configuration to support PostgreSQL connection strings.

## Testing

The project includes comprehensive unit tests for all models to ensure data integrity and proper functionality.

### Running Tests

To run all tests:
```bash
go test ./...
```

To run tests for a specific package:
```bash
go test ./models
```

To run tests with verbose output:
```bash
go test ./models -v
```

To run tests with coverage:
```bash
go test ./models -cover
```

To run tests with race detection:
```bash
go test ./models -race
```

### Test Coverage

The models package has **100% test coverage**, including:

- **Model Creation**: Testing struct instantiation and field validation
- **Database Hooks**: Validating `BeforeCreate` hooks for UUID generation and timestamp handling
- **Relationships**: Testing foreign key relationships and associations
- **Edge Cases**: Handling nil values, pre-existing data, and constraint validation
- **Integration**: Testing model interactions with the database

### Test Structure

Tests are organized by model with subtests for specific functionality:

```
TestLibrary/
├── Library_struct_creation
├── Library_BeforeCreate_hook
└── Library_with_pre-existing_UUID

TestPhoto/
├── Photo_struct_creation
├── Photo_with_nil_rating
├── Photo_BeforeCreate_hook
└── Photo_with_pre-existing_UploadedAt

... (and more for each model)
```

### Test Database

Tests use an in-memory SQLite database for:
- **Fast execution**: No file I/O overhead
- **Isolation**: Each test runs in a clean environment
- **Portability**: No external database dependencies

### Adding Tests

When adding new models or functionality:

1. Create test functions following the pattern `TestModelName`
2. Use subtests with `t.Run()` for different test cases
3. Ensure proper setup with `setupTestDB(t)`
4. Test both success and edge cases
5. Verify 100% coverage is maintained

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable (maintain 100% coverage)
5. Run the test suite to ensure all tests pass
6. Submit a pull request

## License

This project is open source and available under the MIT License.

## Future Enhancements

- [ ] Image thumbnails generation
- [ ] EXIF data extraction
- [ ] Duplicate photo detection
- [ ] Bulk operations
- [ ] Search functionality
- [ ] User authentication
- [ ] Web UI interface
- [ ] Photo sharing capabilities
- [ ] Backup and sync features 