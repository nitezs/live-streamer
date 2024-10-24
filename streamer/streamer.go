package streamer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"live-streamer/config"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type playState struct {
	currentVideoIndex int
	manualControl     bool
	cmd               *exec.Cmd
	ctx               context.Context
	cancel            context.CancelFunc
	waitDone          chan any
}

type Streamer struct {
	playStateMu sync.RWMutex
	playState   playState

	videoMu   sync.RWMutex
	videoList []config.InputItem

	outputMu sync.RWMutex
	output   strings.Builder
}

var GlobalStreamer *Streamer

func NewStreamer(videoList []config.InputItem) *Streamer {
	GlobalStreamer = &Streamer{
		videoList: videoList,
		playState: playState{},
		output:    strings.Builder{},
	}
	return GlobalStreamer
}

func (s *Streamer) start() {
	s.playStateMu.Lock()
	s.playState.ctx, s.playState.cancel = context.WithCancel(context.Background())
	cancel := s.playState.cancel
	currentVideo := s.videoList[s.playState.currentVideoIndex]
	videoPath := currentVideo.Path
	s.playState.cmd = exec.CommandContext(s.playState.ctx, "ffmpeg", s.buildFFmpegArgs(currentVideo)...)
	s.playState.waitDone = make(chan any)
	cmd := s.playState.cmd
	s.playStateMu.Unlock()

	s.writeOutput(fmt.Sprintln("start stream: ", videoPath))

	pipe, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("failed to get pipe: %v", err)
		return
	}

	reader := bufio.NewReader(pipe)

	if err := cmd.Start(); err != nil {
		s.writeOutput(fmt.Sprintf("starting ffmpeg error: %v\n", err))
		return
	}

	go s.log(reader)

	_ = cmd.Wait()
	cancel()

	s.writeOutput(fmt.Sprintf("stop stream: %s\n", videoPath))

	s.playStateMu.Lock()
	if s.playState.manualControl {
		// manualing change video, don't increase currentVideoIndex
		s.playState.manualControl = false
	} else {
		s.playState.currentVideoIndex++
		s.videoMu.RLock()
		if s.playState.currentVideoIndex >= len(s.videoList) {
			s.playState.currentVideoIndex = 0
		}
		s.videoMu.RUnlock()
	}
	close(s.playState.waitDone)
	s.playStateMu.Unlock()
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

func (s *Streamer) Stop() {
	s.playStateMu.Lock()
	cancel := s.playState.cancel
	s.playState.cancel = nil
	cmd := s.playState.cmd
	s.playState.cmd = nil
	done := s.playState.waitDone
	s.playStateMu.Unlock()

	if cancel == nil || cmd == nil {
		return
	}

	cancel()

	if cmd.Process != nil {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
		}
	}
}

func (s *Streamer) Add(videoPath string) {
	s.videoMu.Lock()
	defer s.videoMu.Unlock()
	s.videoList = append(s.videoList, config.InputItem{Path: videoPath})
}

func (s *Streamer) Remove(videoPath string) {
	var needStop bool // removed video is current playing
	var removeIndex int = -1

	s.videoMu.Lock()
	for i, item := range s.videoList {
		if item.Path == videoPath {
			removeIndex = i

			s.playStateMu.RLock()
			needStop = (s.playState.currentVideoIndex == i)
			s.playStateMu.RUnlock()

			break
		}
	}

	if removeIndex >= 0 && removeIndex < len(s.videoList) {
		oldLen := len(s.videoList)
		s.videoList = append(s.videoList[:removeIndex], s.videoList[removeIndex+1:]...)

		s.playStateMu.Lock()
		if s.playState.currentVideoIndex >= oldLen-1 {
			s.playState.currentVideoIndex = 0
		}
		s.playStateMu.Unlock()
	}
	s.videoMu.Unlock()

	if needStop {
		s.Stop()
	}
}

func (s *Streamer) Prev() {
	s.videoMu.RLock()
	videoLen := len(s.videoList)
	if videoLen == 0 {
		return
	}
	s.videoMu.RUnlock()

	s.playStateMu.Lock()
	s.playState.manualControl = true
	s.playState.currentVideoIndex--
	if s.playState.currentVideoIndex < 0 {
		s.playState.currentVideoIndex = videoLen - 1
	}
	s.playStateMu.Unlock()

	s.Stop()
}

func (s *Streamer) Next() {
	s.videoMu.RLock()
	videoLen := len(s.videoList)
	if videoLen == 0 {
		return
	}
	s.videoMu.RUnlock()

	s.playStateMu.Lock()
	s.playState.manualControl = true
	s.playState.currentVideoIndex++
	if s.playState.currentVideoIndex >= videoLen {
		s.playState.currentVideoIndex = 0
	}
	s.playStateMu.Unlock()

	s.Stop()
}

func (s *Streamer) log(reader *bufio.Reader) {
	s.playStateMu.RLock()
	ctx := s.playState.ctx
	s.playStateMu.RUnlock()

	select {
	case <-ctx.Done():
		return
	default:
		if !config.GlobalConfig.Log.PlayState {
			return
		}
		buf := make([]byte, 1024)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				videoPath := s.GetCurrentVideoPath()
				buf = append([]byte(videoPath), buf...)
				s.writeOutput(string(buf[:n+len(videoPath)]))
			}
			if err != nil {
				if err != io.EOF {
					s.writeOutput(fmt.Sprintf("reading ffmpeg output error: %v\n", err))
				}
				break
			}
		}
	}
}

func (s *Streamer) GetCurrentVideoPath() string {
	s.videoMu.RLock()
	defer s.videoMu.RUnlock()
	if len(s.videoList) == 0 {
		return ""
	}
	return s.videoList[s.GetCurrentIndex()].Path
}

func (s *Streamer) GetVideoList() []config.InputItem {
	s.videoMu.RLock()
	defer s.videoMu.RUnlock()
	return s.videoList
}

func (s *Streamer) GetVideoListPath() []string {
	s.videoMu.RLock()
	defer s.videoMu.RUnlock()
	var videoList []string
	for _, item := range s.videoList {
		videoList = append(videoList, item.Path)
	}
	return videoList
}

func (s *Streamer) GetCurrentIndex() int {
	s.playStateMu.RLock()
	defer s.playStateMu.RUnlock()
	return s.playState.currentVideoIndex
}

func (s *Streamer) writeOutput(str string) {
	s.outputMu.Lock()
	defer s.outputMu.Unlock()
	s.output.WriteString(str)
}

func (s *Streamer) GetOutput() string {
	s.outputMu.RLock()
	defer s.outputMu.RUnlock()
	return s.output.String()
}

func (s *Streamer) Close() {
	s.Stop()
	os.Exit(0)
}

func (s *Streamer) buildFFmpegArgs(videoItem config.InputItem) []string {
	videoPath := videoItem.Path

	args := []string{"-re"}
	if videoItem.Start != "" {
		args = append(args, "-ss", videoItem.Start)
	}

	if videoItem.End != "" {
		args = append(args, "-to", videoItem.End)
	}

	args = append(args,
		"-i", videoPath,
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
	)

	if config.GlobalConfig.Play.CustomArgs != "" {
		customArgs := strings.Fields(config.GlobalConfig.Play.CustomArgs)
		args = append(args, customArgs...)
	}

	args = append(args, fmt.Sprintf("%s/%s", config.GlobalConfig.Output.RTMPServer, config.GlobalConfig.Output.StreamKey))

	log.Println("ffmpeg args: ", args)

	return args
}
