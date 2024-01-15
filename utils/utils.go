package utils

import "unicode"

func TruncateString(str string, maxLen int, suffix string) string {
	currentIndex := 0
	spaceIndex := maxLen

	for i, r := range str {
		if unicode.IsSpace(r) {
			spaceIndex = i
		}

		currentIndex++
		if currentIndex > maxLen {
			return str[:spaceIndex] + suffix
		}
	}

	return str
}
