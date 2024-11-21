package domain

import (
	"path"
	"strings"
)

var AllowedFileTypes = map[string]bool{
	// Audio files
	"audio/wav":   true,
	"audio/x-wav": true,
	"audio/mpeg":  true,
	"audio/mp3":   true,
	"audio/aiff":  true,
	"audio/mp4":   true,
	"audio/ogg":   true,

	// DAW Project files
	"audio/x-flp":        true, // FL Studio
	"audio/x-logic":      true, // Logic Pro
	"audio/x-ableton":    true, // Ableton
	"audio/x-protools":   true, // Pro Tools
	"audio/x-cubase":     true, // Cubase
	"audio/x-studio-one": true, // Studio One
	"audio/x-reason":     true, // Reason
	"audio/x-reaper":     true, // Reaper
	"audio/x-bitwig":     true, // Bitwig

	// Common project extensions
	"application/x-daw":        true, // Generic DAW files
	"application/x-project":    true, // Generic project files
	"application/octet-stream": true, // Fallback for binary files

	// Archive formats (for project files)
	"application/zip":   true,
	"application/x-rar": true,
	"application/x-7z":  true,
}

// Helper function to determine content type based on extension
func determineContentType(filename string) string {
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
	// DAW Project Files
	case ".flp":
		return "audio/x-flp" // FL Studio
	case ".als", ".alc":
		return "audio/x-ableton" // Ableton
	case ".logic", ".logicx":
		return "audio/x-logic" // Logic Pro
	case ".ptx", ".ptf":
		return "audio/x-protools" // Pro Tools
	case ".cpr":
		return "audio/x-cubase" // Cubase
	case ".rpp":
		return "audio/x-reaper" // Reaper
	case ".reason":
		return "audio/x-reason" // Reason
	case ".song":
		return "audio/x-studio-one" // Studio One
	case ".bwproject":
		return "audio/x-bitwig" // Bitwig

	// Audio Files
	case ".wav":
		return "audio/wav"
	case ".mp3":
		return "audio/mpeg"
	case ".aiff", ".aif":
		return "audio/aiff"
	case ".m4a":
		return "audio/mp4"
	case ".ogg":
		return "audio/ogg"

	// Archives
	case ".zip":
		return "application/zip"
	case ".rar":
		return "application/x-rar"
	case ".7z":
		return "application/x-7z"

	default:
		return "application/octet-stream"
	}
}
