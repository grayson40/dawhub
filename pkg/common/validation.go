package common

import "regexp"

var (
	nameRegex     = regexp.MustCompile(`^[a-zA-Z0-9\s\-_\.]{3,50}$`)
	versionRegex  = regexp.MustCompile(`^[0-9]+\.[0-9]+(\.[0-9]+)?$`)
	filenameRegex = regexp.MustCompile(`^[a-zA-Z0-9\s\-_\.]+\.[a-zA-Z0-9]+$`)
)

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

// Error implements the error interface for ValidationErrors
func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return "no validation errors"
	}
	return ve[0].Message
}

// ValidateProject validates a project's fields
func ValidateProject(name, version string) error {
	var errors ValidationErrors

	if !nameRegex.MatchString(name) {
		errors = append(errors, ValidationError{
			Field:   "name",
			Message: "Name must be between 3 and 50 characters and contain only letters, numbers, spaces, hyphens, underscores, and dots",
		})
	}

	if version != "" && !versionRegex.MatchString(version) {
		errors = append(errors, ValidationError{
			Field:   "version",
			Message: "Version must be in format X.Y or X.Y.Z where X, Y, and Z are numbers",
		})
	}

	if len(errors) > 0 {
		return NewError(ErrValidation,
			WithMessage("Validation failed"),
			WithStatusCode(422),
			WithDetails(map[string]interface{}{
				"errors": errors,
			}),
		)
	}

	return nil
}

// ValidateFilename validates a file name
func ValidateFilename(filename string) error {
	if !filenameRegex.MatchString(filename) {
		return NewValidationError("filename", "Invalid filename format")
	}
	return nil
}
