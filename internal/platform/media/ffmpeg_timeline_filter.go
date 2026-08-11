package media

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
)

func BuildTimelineFilter(request TimelineRenderRequest, subtitlePath string) (string, string, string) {
	parts := make([]string, 0, len(request.Video)+len(request.Audio)+12)
	video := append([]TimelineVideoClip(nil), request.Video...)
	sort.Slice(video, func(i, j int) bool { return video[i].StartMS < video[j].StartMS })
	videoInputs := make([]string, 0, len(video))
	for index, clip := range video {
		duration := clip.EndMS - clip.StartMS
		label := fmt.Sprintf("v%d", index)
		parts = append(parts, fmt.Sprintf("[%d:v]trim=start=%s:duration=%s,setpts=PTS-STARTPTS,scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,fps=%d,format=yuv420p[%s]", index, seconds(clip.SourceIn), seconds(duration), request.Width, request.Height, request.Width, request.Height, request.FrameRate, label))
		videoInputs = append(videoInputs, "["+label+"]")
	}
	parts = append(parts, strings.Join(videoInputs, "")+fmt.Sprintf("concat=n=%d:v=1:a=0[visual]", len(videoInputs)))
	escapedSubtitle := strings.ReplaceAll(filepath.ToSlash(subtitlePath), ":", `\:`)
	escapedSubtitle = strings.ReplaceAll(escapedSubtitle, "'", `\'`)
	parts = append(parts, fmt.Sprintf("[visual]subtitles='%s'[videoout]", escapedSubtitle))

	groups := map[TimelineAudioRole][]string{}
	audioOffset := len(video)
	for index, clip := range request.Audio {
		label := fmt.Sprintf("a%d", index)
		duration := clip.EndMS - clip.StartMS
		filters := []string{fmt.Sprintf("atrim=start=%s:duration=%s", seconds(clip.SourceIn), seconds(duration)), "asetpts=PTS-STARTPTS"}
		if clip.Loop {
			filters = []string{fmt.Sprintf("atrim=start=%s,asetpts=PTS-STARTPTS,aloop=loop=-1:size=2147483647,atrim=duration=%s", seconds(clip.SourceIn), seconds(duration)), "asetpts=PTS-STARTPTS"}
		}
		if math.Abs(clip.GainDB) > 0.001 {
			filters = append(filters, fmt.Sprintf("volume=%.2fdB", clip.GainDB))
		}
		filters = append(filters, fmt.Sprintf("adelay=%d|%d", clip.StartMS, clip.StartMS))
		parts = append(parts, fmt.Sprintf("[%d:a]%s[%s]", audioOffset+index, strings.Join(filters, ","), label))
		groups[clip.Role] = append(groups[clip.Role], "["+label+"]")
	}
	bus := func(role TimelineAudioRole, label string) string {
		inputs := groups[role]
		if len(inputs) == 0 {
			return ""
		}
		parts = append(parts, strings.Join(inputs, "")+fmt.Sprintf("amix=inputs=%d:normalize=0[%s]", len(inputs), label))
		return "[" + label + "]"
	}
	voice, music, sfx := bus(TimelineAudioVoiceover, "voicebus"), bus(TimelineAudioMusic, "musicbus"), bus(TimelineAudioSFX, "sfxbus")
	if voice != "" && music != "" {
		parts = append(parts, music+voice+"sidechaincompress=threshold=0.03:ratio=8:attack=20:release=300[duckedmusic]")
		music = "[duckedmusic]"
	}
	parts = append(parts, fmt.Sprintf("[%d:a]atrim=duration=%s[silence]", len(video)+len(request.Audio), seconds(request.DurationMS)))
	finalInputs := []string{"[silence]"}
	for _, input := range []string{voice, music, sfx} {
		if input != "" {
			finalInputs = append(finalInputs, input)
		}
	}
	parts = append(parts, strings.Join(finalInputs, "")+fmt.Sprintf("amix=inputs=%d:normalize=0,loudnorm=I=-16:TP=-1.5:LRA=11,alimiter=limit=0.95,atrim=duration=%s[audioout]", len(finalInputs), seconds(request.DurationMS)))
	return strings.Join(parts, ";"), "[videoout]", "[audioout]"
}
