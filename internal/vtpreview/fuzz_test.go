package vtpreview

import "testing"

func FuzzPreview(f *testing.F) {
	for _, seed := range []string{
		"plain text",
		"a\r\nb\r\nc",
		"\x1b[2J\x1b[Hhome",
		"\x1b[?1049h\x1b[Halternate\x1b[?1049l",
		"\x1b[界界abcd",
		"\x1b]0;title\a\x1b[?2031h",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var preview Preview
		preview.Feed(data)
		preview.FeedThrottled(data)
		preview.Resize(80, 24)
		_ = preview.RecoveryFrame()
		_ = preview.Feed(nil)
	})
}
