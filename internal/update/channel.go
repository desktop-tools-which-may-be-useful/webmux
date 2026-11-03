package update

// Channel selects which releases an updater considers.
type Channel string

const (
	// ChannelStable considers tagged releases only. It is the default.
	ChannelStable Channel = "stable"
	// ChannelDev also considers development builds of main and moves on to
	// a stable release once it is higher.
	ChannelDev Channel = "dev"
)

// ParseChannel accepts "stable" or "dev".
func ParseChannel(s string) (Channel, bool) {
	switch Channel(s) {
	case ChannelStable, ChannelDev:
		return Channel(s), true
	}
	return "", false
}

// AheadOfStable explains why a stable-channel updater installs nothing when
// the running development build is above the newest stable release, as it
// is right after switching back from the dev channel. It returns "" in every
// other case.
func AheadOfStable(ch Channel, current, latest string) string {
	if ch == ChannelDev || !IsDevVersion(current) || !Newer(current, latest) {
		return ""
	}
	return "当前开发版 " + current + " 高于最新正式版 " + latest + "，正式版超过它后自动更新"
}
