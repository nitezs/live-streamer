package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"live-streamer/utils"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

type OutputConfig struct {
	RTMPServer string `json:"rtmp_server"`
	StreamKey  string `json:"stream_key"`
}

type InputItem struct {
	Path     string `json:"path"`
	Start    string `json:"start"`
	End      string `json:"end"`
	ItemType string `json:"-"`
}

type PlayConfig struct {
	VideoCodec      string `json:"video_codec"`
	Preset          string `json:"preset"`
	CRF             int    `json:"crf"`
	MaxRate         string `json:"max_rate"`
	BufSize         string `json:"buf_size"`
	Scale           string `json:"scale"`
	FrameRate       int    `json:"frame_rate"`
	AudioCodec      string `json:"audio_codec"`
	AudioBitrate    string `json:"audio_bitrate"`
	AudioSampleRate int    `json:"audio_sample_rate"`
	OutputFormat    string `json:"output_format"`
	CustomArgs      string `json:"custom_args"`
}

type Config struct {
	Input      []any        `json:"input"`
	inputItems []InputItem  `json:"-"` // contains video file or dir
	PlayList   []InputItem  `json:"-"` // only contains video file
	Play       PlayConfig   `json:"play"`
	Output     OutputConfig `json:"output"`
}

var GlobalConfig Config

func init() {
	GlobalConfig = Config{}
	err := readConfig("config.json")
	for i, item := range GlobalConfig.inputItems {
		if item.ItemType == "file" {
			GlobalConfig.PlayList = append(GlobalConfig.PlayList, item)
		} else if item.ItemType == "dir" {
			videos, err := getAllVideos(item.Path)
			if err != nil {
				log.Fatalf("input[%v] walk error: %v", i, err)
			}
			GlobalConfig.PlayList = append(GlobalConfig.PlayList, videos...)
		}
	}
	if len(GlobalConfig.PlayList) == 0 {
		log.Fatal("No input video found")
	}
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatal("Config not exists")
		} else {
			log.Fatal(err)
		}
	}
}

func readConfig(configPath string) error {
	stat, err := os.Stat(configPath)
	if err != nil {
		return fmt.Errorf("config read failed: %v", err)
	}
	if stat.IsDir() {
		return os.ErrNotExist
	}
	databytes, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("Config read failed: %v", err)
	}
	if err = json.Unmarshal(databytes, &GlobalConfig); err != nil {
		return fmt.Errorf("config unmarshal failed: %v", err)
	}
	err = validateConfig()
	if err != nil {
		return fmt.Errorf("config validate failed: %v", err)
	}
	return nil
}

func validateInputConfig() error {
	if GlobalConfig.Input == nil {
		return errors.New("video_path is nil")
	} else {
		for i, item := range GlobalConfig.Input {
			typeOf := reflect.TypeOf(item)
			var inputItem InputItem
			if typeOf.Kind() == reflect.String {
				inputItem = InputItem{Path: item.(string)}
			}
			if inputItem.Path == "" {
				return fmt.Errorf("video_path[%v] is empty", i)
			}
			stat, err := os.Stat(inputItem.Path)
			if err != nil {
				return fmt.Errorf("video_path[%v] stat failed: %v", i, err)
			}
			if stat.IsDir() {
				inputItem.ItemType = "dir"
			} else {
				inputItem.ItemType = "file"
				if !utils.IsSupportedVideo(inputItem.Path) {
					return fmt.Errorf("video_path[%v] is not supported", i)
				}
			}
			GlobalConfig.inputItems = append(GlobalConfig.inputItems, inputItem)
		}
	}
	return nil
}

func validateOutputConfig() error {
	if GlobalConfig.Output.RTMPServer == "" {
		return errors.New("rtmp_server is empty")
	} else if !strings.HasPrefix(GlobalConfig.Output.RTMPServer, "rtmp://") &&
		!strings.HasPrefix(GlobalConfig.Output.RTMPServer, "rtmps://") {
		return errors.New("rtmp_server is not a valid rtmp server")
	} else {
		GlobalConfig.Output.RTMPServer = strings.TrimSuffix(GlobalConfig.Output.RTMPServer, "/")
	}
	if GlobalConfig.Output.StreamKey == "" {
		return errors.New("stream_key is empty")
	} else {
		GlobalConfig.Output.StreamKey = strings.TrimPrefix(GlobalConfig.Output.StreamKey, "/")
	}
	return nil
}

func validatePlayConfig() error {
	if GlobalConfig.Play.VideoCodec == "" {
		GlobalConfig.Play.VideoCodec = "libx264"
	}
	if GlobalConfig.Play.Preset == "" {
		GlobalConfig.Play.Preset = "fast"
	}
	if GlobalConfig.Play.CRF == 0 {
		GlobalConfig.Play.CRF = 23
	}
	if GlobalConfig.Play.MaxRate == "" {
		GlobalConfig.Play.MaxRate = "8000k"
	}
	if GlobalConfig.Play.BufSize == "" {
		GlobalConfig.Play.BufSize = "12000k"
	}
	if GlobalConfig.Play.Scale == "" {
		GlobalConfig.Play.Scale = "1920:1080"
	}
	if GlobalConfig.Play.FrameRate == 0 {
		GlobalConfig.Play.FrameRate = 30
	}
	if GlobalConfig.Play.AudioCodec == "" {
		GlobalConfig.Play.AudioCodec = "aac"
	}
	if GlobalConfig.Play.AudioBitrate == "" {
		GlobalConfig.Play.AudioBitrate = "192k"
	}
	if GlobalConfig.Play.AudioSampleRate == 0 {
		GlobalConfig.Play.AudioSampleRate = 48000
	}
	if GlobalConfig.Play.OutputFormat == "" {
		GlobalConfig.Play.OutputFormat = "flv"
	}
	return nil
}

func validateConfig() error {
	if err := validateInputConfig(); err != nil {
		return err
	}
	if err := validateOutputConfig(); err != nil {
		return err
	}
	if err := validatePlayConfig(); err != nil {
		return err
	}
	return nil
}

func getAllVideos(dirPath string) ([]InputItem, error) {
	res := []InputItem{}
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && utils.IsSupportedVideo(path) {
			res = append(res, InputItem{Path: path})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}
