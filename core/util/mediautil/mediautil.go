package mediautil

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/yapingcat/gomedia/go-mp4"
)

func split(mime string) (primary string, sub string, ok bool) {
	types := strings.Split(mime, "/")

	if len(types) != 2 {
		return "", "", false
	}

	return types[0], types[1], true
}

func IsVideo(mime string) bool {
	primary, _, ok := split(mime)

	return primary == "video" && ok
}

func IsAudio(mime string) bool {
	primary, _, ok := split(mime)

	return primary == "audio" && ok
}

func IsImage(mime string) bool {
	primary, _, ok := split(mime)

	return primary == "image" && ok
}

// GetMP4Info returns duration, width, height, error
func GetMP4Info(r io.ReadSeeker) (int, int, int, error) {
	d := mp4.CreateMp4Demuxer(r)

	tracks, err := d.ReadHead()
	if err != nil {
		return 0, 0, 0, err
	}

	for _, track := range tracks {
		if track.Cid == mp4.MP4_CODEC_H264 {
			info := d.GetMp4Info()
			return int(info.Duration / info.Timescale), int(track.Width), int(track.Height), nil
		}
	}

	return 0, 0, 0, fmt.Errorf("no h264 track found")
}

// VideoInfo holds basic metadata about a video file.
type VideoInfo struct {
	Duration int // seconds
	Width    int
	Height   int
}

// ffprobeOutput is a minimal subset of ffprobe's JSON output.
type ffprobeOutput struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		Duration  string `json:"duration"` // seconds as string, may be missing
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"` // seconds as string
	} `json:"format"`
}

// GetVideoInfoFFmpeg uses ffprobe (bundled with FFmpeg) to retrieve the
// duration, width and height of any video file. It returns an error that
// advises the user to install FFmpeg when the binary is not found in PATH.
func GetVideoInfoFFmpeg(filePath string) (VideoInfo, error) {
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		return VideoInfo{}, fmt.Errorf(
			"ffprobe not found in PATH (install FFmpeg to enable video metadata extraction): %w", err)
	}

	cmd := exec.Command(ffprobe,
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		filePath,
	)

	out, err := cmd.Output()
	if err != nil {
		return VideoInfo{}, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probe ffprobeOutput
	if err = json.Unmarshal(out, &probe); err != nil {
		return VideoInfo{}, fmt.Errorf("parse ffprobe output: %w", err)
	}

	var info VideoInfo

	// Duration: prefer format-level, fall back to first video stream
	if probe.Format.Duration != "" {
		if d, err := strconv.ParseFloat(probe.Format.Duration, 64); err == nil {
			info.Duration = int(d)
		}
	}

	for _, s := range probe.Streams {
		if s.CodecType == "video" {
			info.Width = s.Width
			info.Height = s.Height
			if info.Duration == 0 && s.Duration != "" {
				if d, err := strconv.ParseFloat(s.Duration, 64); err == nil {
					info.Duration = int(d)
				}
			}
			break
		}
	}

	if info.Width == 0 && info.Height == 0 {
		return info, fmt.Errorf("no video stream found in file")
	}

	return info, nil
}
