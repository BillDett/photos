package handlers

import "strings"

// processValidationError extracts field names from gin validation errors
func processValidationError(err error) string {
	errStr := err.Error()

	// Extract field name from gin validation error
	if strings.Contains(errStr, "Error:Field validation for 'Name' failed") {
		if strings.Contains(errStr, "required") {
			return "name is required"
		}
		if strings.Contains(errStr, "max") {
			return "name must be at most 100 characters"
		}
		if strings.Contains(errStr, "min") {
			return "name must be at least 1 character"
		}
		return "name is invalid"
	}
	if strings.Contains(errStr, "Error:Field validation for 'Color' failed") {
		if strings.Contains(errStr, "len") {
			return "color must be exactly 7 characters (e.g., #FF0000)"
		}
		return "color validation failed"
	}
	if strings.Contains(errStr, "Error:Field validation for 'Description' failed") {
		if strings.Contains(errStr, "max") {
			return "description must be at most 500 characters"
		}
		return "description is invalid"
	}
	if strings.Contains(errStr, "Error:Field validation for 'LibraryID' failed") {
		return "library_id is required"
	}
	if strings.Contains(errStr, "Error:Field validation for 'PhotoID' failed") {
		return "photo_id is required"
	}
	if strings.Contains(errStr, "Error:Field validation for 'Order' failed") {
		return "order is required"
	}
	if strings.Contains(errStr, "Error:Field validation for 'Images' failed") {
		if strings.Contains(errStr, "required") {
			return "images is required"
		}
		if strings.Contains(errStr, "max") {
			return "images path must be at most 500 characters"
		}
		if strings.Contains(errStr, "min") {
			return "images path must be at least 1 character"
		}
		return "images path is invalid"
	}
	if strings.Contains(errStr, "Error:Field validation for 'Rating' failed") {
		if strings.Contains(errStr, "min") || strings.Contains(errStr, "max") {
			return "rating must be between 0 and 5"
		}
		return "rating is invalid"
	}

	// Fallback to original error
	return errStr
}
