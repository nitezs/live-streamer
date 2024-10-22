package utils

import "os/exec"

func HasFFMPEG() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}
