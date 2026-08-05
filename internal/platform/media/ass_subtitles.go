package media

import (
	"fmt"
	"strings"
)

func BuildASSSubtitles(captions []TimelineCaption, width, height int) ([]byte, error) {
	if width != 720 || height != 1280 {
		return nil, fmt.Errorf("ASS subtitles require the 720x1280 profile")
	}
	var body strings.Builder
	body.WriteString("[Script Info]\nScriptType: v4.00+\nPlayResX: 720\nPlayResY: 1280\nWrapStyle: 2\nScaledBorderAndShadow: yes\n\n")
	body.WriteString("[V4+ Styles]\nFormat: Name,Fontname,Fontsize,PrimaryColour,SecondaryColour,OutlineColour,BackColour,Bold,Italic,Underline,StrikeOut,ScaleX,ScaleY,Spacing,Angle,BorderStyle,Outline,Shadow,Alignment,MarginL,MarginR,MarginV,Encoding\n")
	body.WriteString("Style: Default,Noto Sans CJK SC,42,&H00FFFFFF,&H000000FF,&H00101010,&H60000000,-1,0,0,0,100,100,0,0,1,3,1,2,70,70,80,1\n\n")
	body.WriteString("[Events]\nFormat: Layer,Start,End,Style,Name,MarginL,MarginR,MarginV,Effect,Text\n")
	for _, caption := range captions {
		if caption.EndMS <= caption.StartMS || strings.TrimSpace(caption.Text) == "" {
			return nil, fmt.Errorf("caption interval and text are required")
		}
		body.WriteString(fmt.Sprintf("Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n", assTime(caption.StartMS), assTime(caption.EndMS), escapeASSText(caption.Text)))
	}
	return []byte(body.String()), nil
}

func assTime(ms int) string {
	hours := ms / 3600000
	minutes := ms / 60000 % 60
	seconds := ms / 1000 % 60
	centiseconds := ms / 10 % 100
	return fmt.Sprintf("%d:%02d:%02d.%02d", hours, minutes, seconds, centiseconds)
}

func escapeASSText(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "{", `\{`)
	value = strings.ReplaceAll(value, "}", `\}`)
	value = strings.ReplaceAll(value, "\r\n", `\N`)
	value = strings.ReplaceAll(value, "\n", `\N`)
	return value
}
