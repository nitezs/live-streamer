package server

import (
	"live-streamer/streamer"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetCurrentVideo(c *gin.Context) {
	type response struct {
		Success bool   `json:"success"`
		Data    string `json:"data"`
		Message string `json:"message"`
	}
	videoPath, err := streamer.GlobalStreamer.GetCurrentVideoPath()
	if err != nil {
		c.JSON(http.StatusOK, response{
			Success: false,
			Message: err.Error(),
		})
	}
	c.JSON(http.StatusOK, response{
		Success: true,
		Data:    videoPath,
	})
}

func GetVideoList(c *gin.Context) {
	type response struct {
		Success bool     `json:"success"`
		Data    []string `json:"data"`
	}
	list := streamer.GlobalStreamer.GetVideoListPath()
	c.JSON(http.StatusOK, response{
		Success: true,
		Data:    list,
	})
}
