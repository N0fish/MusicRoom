package main

import (
	"crypto/sha1"
	"fmt"
	"strings"
)

func defaultAvatarURL() string {
	return getenv("DEFAULT_AVATAR_URL", "/static/avatars/default.svg")
}

func resolveAvatarForViewer(p UserProfile, viewerIsFriend bool, viewerIsOwner bool) string {
	if p.Visibility == "private" && !viewerIsOwner {
		return defaultAvatarURL()
	}
	if p.Visibility == "friends" && !viewerIsFriend && !viewerIsOwner {
		return defaultAvatarURL()
	}
	if p.HasCustomAvatar && strings.TrimSpace(p.AvatarURL) != "" {
		return p.AvatarURL
	}
	return defaultAvatarURL()
}

func generateIdenticonSVG(seed string) string {
	h := sha1.Sum([]byte(seed))

	const gridSize = 5
	const cellSize = 20
	totalSize := gridSize * cellSize

	bg := "#f0f0f0"
	fg := fmt.Sprintf("#%02x%02x%02x", h[0], h[1], h[2])

	var b strings.Builder
	b.WriteString(fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`,
		totalSize, totalSize, totalSize, totalSize,
	))
	b.WriteString(`<rect width="100%" height="100%" fill="` + bg + `"/>`)

	bitIndex := 0
	for y := 0; y < gridSize; y++ {
		for x := 0; x < (gridSize+1)/2; x++ {
			byteIndex := bitIndex / 8
			bitPos := uint(bitIndex % 8)
			on := (h[4+byteIndex]>>bitPos)&1 == 1
			if on {
				px := x * cellSize
				py := y * cellSize
				b.WriteString(fmt.Sprintf(
					`<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>`,
					px, py, cellSize, cellSize, fg,
				))
				px2 := (gridSize - 1 - x) * cellSize
				b.WriteString(fmt.Sprintf(
					`<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>`,
					px2, py, cellSize, cellSize, fg,
				))
			}
			bitIndex++
		}
	}

	b.WriteString(`</svg>`)
	return b.String()
}
