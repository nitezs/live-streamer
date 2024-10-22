package utils

import (
	"live-streamer/constant"
	"path/filepath"
	"slices"
	"strings"
)

func IsSupportedVideo(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return slices.Contains(constant.SupportedStreamingFormats, strings.TrimPrefix(ext, "."))
}
