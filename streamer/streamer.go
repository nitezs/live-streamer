package streamer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"live-streamer/config"
	"live-streamer/logger"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Streamer struct {
	videoList         []config.InputItem
	currentVideoIndex int
	cmd               *exec.Cmd
	ctx               context.Context
	cancel            context.CancelFunc
	mu                sync.Mutex
	logger            logger.Logger
}

var GlobalStreamer *Streamer

func NewStreamer(videoList []config.InputItem, logger logger.Logger) *Streamer {
	GlobalStreamer = &Streamer{
		videoList:         videoList,
		currentVideoIndex: 0,
		cmd:               nil,
		ctx:               nil,
		logger:            logger,
	}
	return GlobalStreamer
}

func (s *Streamer) Stream() {
	for {
		if len(s.videoList) == 0 {
			time.Sleep(time.Second)
			continue
		}
		s.start()
	}
}

func (s *Streamer) start() {
	s.Stop()

	s.ctx, s.cancel = context.WithCancel(context.Background())

	currentVideo := s.videoList[s.currentVideoIndex]
	videoPath := currentVideo.Path
	s.logger.Println("start stream: ", videoPath)

	s.mu.Lock()
	s.cmd = exec.CommandContext(s.ctx, "ffmpeg", s.buildFFmpegArgs(currentVideo)...)
	s.mu.Unlock()

	pipe, err := s.cmd.StderrPipe()
	if err != nil {
		s.logger.Printf("failed to get pipe: %v", err)
		return
	}

	reader := bufio.NewReader(pipe)

	if err := s.cmd.Start(); err != nil {
		s.logger.Printf("starting ffmpeg error: %v\n", err)
		return
	}

	go s.log(reader)

	<-s.ctx.Done()
	s.logger.Printf("stop stream: %s", videoPath)

	// stream next video
	s.currentVideoIndex++
	if s.currentVideoIndex >= len(s.videoList) {
		s.currentVideoIndex = 0
	}
}

func (s *Streamer) Stop() {
	if s.cancel != nil {
		stopped := make(chan error)
		go func() {
			stopped <- s.cmd.Wait()
		}()
		s.cancel()
		s.mu.Lock()
		if s.cmd != nil && s.cmd.Process != nil {
			select {
			case <-stopped:
				break
			case <-time.After(3 * time.Second):
				_ = s.cmd.Process.Kill()
				break
			}
			s.cmd = nil
		}
		s.mu.Unlock()
	}
}

func (s *Streamer) Add(videoPath string) {
	s.videoList = append(s.videoList, config.InputItem{Path: videoPath})
}

func (s *Streamer) Remove(videoPath string) {
	for i, item := range s.videoList {
		if item.Path == videoPath {
			s.videoList = append(s.videoList[:i], s.videoList[i+1:]...)
			if s.currentVideoIndex >= len(s.videoList) {
				s.currentVideoIndex = 0
			}
			if s.currentVideoIndex == i {
				s.Stop()
			}
			break
		}
	}
}

func (s *Streamer) Prev() {
	s.currentVideoIndex--
	if s.currentVideoIndex < 0 {
		s.currentVideoIndex = len(s.videoList) - 1
	}
	s.Stop()
}

func (s *Streamer) Next() {
	s.currentVideoIndex++
	if s.currentVideoIndex >= len(s.videoList) {
		s.currentVideoIndex = 0
	}
	s.Stop()
}

func (s *Streamer) log(reader *bufio.Reader) {
	select {
	case <-s.ctx.Done():
		return
	default:
		if !config.GlobalConfig.Log.PlayState {
			return
		}
		buf := make([]byte, 1024)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				videoPath, _ := s.GetCurrentVideoPath()
				buf = append([]byte(videoPath), buf...)
				s.logger.Print(string(buf[:n]))
			}
			if err != nil {
				if err != io.EOF {
					s.logger.Printf("reading ffmpeg error: %v\n", err)
				}
				break
			}
		}
	}
}

func (s *Streamer) buildFFmpegArgs(videoItem config.InputItem) []string {
	videoPath := videoItem.Path

	args := []string{"-re"}
	if videoItem.Start != "" {
		args = append(args, "-ss", videoItem.Start)
	}

	args = append(args, "-i", videoPath)

	if videoItem.End != "" {
		args = append(args, "-to", videoItem.End)
	}

	args = append(args,
		"-c:v", config.GlobalConfig.Play.VideoCodec,
		"-preset", config.GlobalConfig.Play.Preset,
		"-crf", fmt.Sprintf("%d", config.GlobalConfig.Play.CRF),
		"-maxrate", config.GlobalConfig.Play.MaxRate,
		"-bufsize", config.GlobalConfig.Play.BufSize,
		"-vf", fmt.Sprintf("scale=%s", config.GlobalConfig.Play.Scale),
		"-r", fmt.Sprintf("%d", config.GlobalConfig.Play.FrameRate),
		"-c:a", config.GlobalConfig.Play.AudioCodec,
		"-b:a", config.GlobalConfig.Play.AudioBitrate,
		"-ar", fmt.Sprintf("%d", config.GlobalConfig.Play.AudioSampleRate),
		"-f", config.GlobalConfig.Play.OutputFormat,
		"-stats", "-loglevel", "info",
	)

	if config.GlobalConfig.Play.CustomArgs != "" {
		customArgs := strings.Fields(config.GlobalConfig.Play.CustomArgs)
		args = append(args, customArgs...)
	}

	args = append(args, fmt.Sprintf("%s/%s", config.GlobalConfig.Output.RTMPServer, config.GlobalConfig.Output.StreamKey))

	// logger.GlobalLogger.Println("ffmpeg args: ", args)

	return args
}

func (s *Streamer) GetCurrentVideoPath() (string, error) {
	if len(s.videoList) == 0 {
		return "", errors.New("no video streaming")
	}
	return s.videoList[s.currentVideoIndex].Path, nil
}

func (s *Streamer) GetVideoList() []config.InputItem {
	return s.videoList
}

func (s *Streamer) GetVideoListPath() []string {
	var videoList []string
	for _, item := range s.videoList {
		videoList = append(videoList, item.Path)
	}
	return videoList
}

func (s *Streamer) GetCurrentIndex() int {
	return s.currentVideoIndex
}

func (s *Streamer) Close() {
	s.Stop()
}
