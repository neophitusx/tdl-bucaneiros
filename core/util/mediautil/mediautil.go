package mediautil

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
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
		CodecType         string `json:"codec_type"`
		Width             int    `json:"width"`
		Height            int    `json:"height"`
		SampleAspectRatio string `json:"sample_aspect_ratio"`
		Duration          string `json:"duration"` // seconds as string, may be missing
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

			// Convert anamorphic video dimensions to their display dimensions.
			// For square pixels (SAR 1:1), the stored dimensions are already correct.
			if s.SampleAspectRatio != "" && s.SampleAspectRatio != "1:1" {
				parts := strings.SplitN(s.SampleAspectRatio, ":", 2)
				if len(parts) == 2 {
					num, errNum := strconv.Atoi(parts[0])
					den, errDen := strconv.Atoi(parts[1])
					if errNum == nil && errDen == nil && num > 0 && den > 0 {
						info.Width = int(math.Round(
							float64(s.Width) * float64(num) / float64(den),
						))
					}
				}
			}

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

// GenerateVideoThumbnailFFmpeg extracts a representative frame from a video
// into a temporary JPEG file. The caller is responsible for removing the file.
func GenerateVideoThumbnailFFmpeg(ctx context.Context, filePath string) (string, error) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", fmt.Errorf("ffmpeg not found in PATH (install FFmpeg to generate video thumbnails): %w", err)
	}

	info, err := GetVideoInfoFFmpeg(filePath)
	if err != nil {
		return "", fmt.Errorf("get video metadata: %w", err)
	}

	thumb, err := os.CreateTemp("", "tdl-thumbnail-*.jpg")
	if err != nil {
		return "", fmt.Errorf("create temporary thumbnail: %w", err)
	}
	thumbPath := thumb.Name()
	if err := thumb.Close(); err != nil {
		_ = os.Remove(thumbPath)
		return "", fmt.Errorf("close temporary thumbnail: %w", err)
	}

	cmd := exec.CommandContext(ctx, ffmpeg,
		"-y",
		"-ss", fmt.Sprintf("%.3f", thumbnailTimestamp(info.Duration)),
		"-i", filePath,
		"-vf", "scale=round(iw*sar/2)*2:ih,setsar=1",
		"-frames:v", "1",
		"-q:v", "2",
		thumbPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(thumbPath)
		return "", fmt.Errorf("ffmpeg thumbnail extraction failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	return thumbPath, nil
}

func thumbnailTimestamp(duration int) float64 {
	if duration <= 0 {
		return 0
	}

	timestamp := float64(duration) * 0.1
	if timestamp < 10 {
		timestamp = 10
	}
	if timestamp > 60 {
		timestamp = 60
	}
	if timestamp >= float64(duration) {
		timestamp = float64(duration) / 2
	}

	return timestamp
}
