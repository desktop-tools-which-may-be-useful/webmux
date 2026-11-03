//go:build unix

package update

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// ChannelFile holds the update channel of `webmux update` on Linux and
// macOS. No package owns it, so installs, upgrades and removal keep it.
const ChannelFile = "/etc/webmux/update-channel"

// ReadChannel returns the channel saved at path. A missing file is the
// stable channel without a warning. An unrecognised value, or a file that is
// not owned by root or is writable by group or others, is also the stable
// channel, with warn describing why: the root update service must not let
// an unprivileged user widen the keys it trusts.
func ReadChannel(path string) (ch Channel, warn error) {
	return readChannel(path, 0)
}

func readChannel(path string, ownerUID uint32) (Channel, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ChannelStable, nil
	}
	if err != nil {
		return ChannelStable, fmt.Errorf("读取更新通道 %s：%w，按 stable 处理", path, err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return ChannelStable, fmt.Errorf("读取更新通道 %s：%w，按 stable 处理", path, err)
	}
	if st, ok := info.Sys().(*syscall.Stat_t); !ok || st.Uid != ownerUID || info.Mode().Perm()&0o022 != 0 {
		return ChannelStable, fmt.Errorf("更新通道文件 %s 不归 root 所有或可被其他用户写入，按 stable 处理", path)
	}
	buf := make([]byte, 64)
	n, _ := f.Read(buf)
	value := strings.TrimSpace(string(buf[:n]))
	ch, ok := ParseChannel(value)
	if !ok {
		return ChannelStable, fmt.Errorf("无法识别的更新通道 %q（%s），按 stable 处理", value, path)
	}
	return ch, nil
}

// WriteChannel saves ch at path, creating its directory with mode 0755
// regardless of the umask, so non-root `webmux update` can read the channel.
// It writes a temporary file and renames it, so readers see the old or the
// new value, never a partial one. The file has mode 0644.
func WriteChannel(path string, ch Channel) error {
	dir := filepath.Dir(path)
	_, statErr := os.Stat(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建 %s：%w", dir, err)
	}
	if errors.Is(statErr, fs.ErrNotExist) {
		if err := os.Chmod(dir, 0o755); err != nil {
			return fmt.Errorf("创建 %s：%w", dir, err)
		}
	}
	tmp, err := os.CreateTemp(dir, ".update-channel-*")
	if err != nil {
		return fmt.Errorf("写入更新通道：%w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	_, werr := tmp.WriteString(string(ch) + "\n")
	cerr := tmp.Chmod(0o644)
	if err := errors.Join(werr, cerr, tmp.Sync(), tmp.Close()); err != nil {
		return fmt.Errorf("写入更新通道：%w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("写入更新通道：%w", err)
	}
	return nil
}
