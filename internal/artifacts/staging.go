package artifacts

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// StagingDir names the folder quick starts upload into before their session
// exists. A file stays there until a quick start hands it to its new
// session (Adopt); whatever is left is dropped when the server starts.
// The name is never a session ID, so the sweep leaves it alone.
const StagingDir = "quick-start"

func (s *Service) stagingPath() (string, error) {
	if s == nil {
		return "", fmt.Errorf("%w: 临时文件夹不可用", ErrUnavailable)
	}
	if s.rootErr != nil {
		return "", s.rootErr
	}
	return filepath.Join(s.root, StagingDir), nil
}

// EnsureStaging creates the staging folder if needed and returns its path.
func (s *Service) EnsureStaging() (string, error) {
	dir, err := s.stagingPath()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range []string{s.root, dir} {
		if err := private(p, true); err != nil {
			s.log.Warn("quick start staging folder unavailable", "path", p, "error", err)
			return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
	}
	return dir, nil
}

// StagedPath validates that path names a regular, non-symlink file directly
// inside the staging folder and returns it cleaned.
func (s *Service) StagedPath(path string) (string, error) {
	invalid := fmt.Errorf("%w: 路径不是快速开始的上传文件", ErrInvalid)
	dir, err := s.stagingPath()
	if err != nil || path == "" || !filepath.IsAbs(path) {
		return "", invalid
	}
	clean := filepath.Clean(path)
	if filepath.Dir(clean) != dir {
		return "", invalid
	}
	for _, p := range []string{s.root, dir} {
		if private(p, false) != nil {
			return "", invalid
		}
	}
	info, err := os.Lstat(clean)
	if err != nil || !info.Mode().IsRegular() {
		return "", invalid
	}
	return clean, nil
}

// Adopt moves staged files into session id's uploads folder under their
// names and returns their new paths, which UploadPath accepts. The first
// file that cannot move stops it; files already moved stay with the
// session.
func (s *Service) Adopt(id string, staged []string) ([]string, error) {
	if len(staged) == 0 {
		return nil, nil
	}
	uploads, err := s.EnsureUploads(id)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(staged))
	for _, path := range staged {
		clean, err := s.StagedPath(path)
		if err != nil {
			return out, err
		}
		dest := filepath.Join(uploads, filepath.Base(clean))
		if _, err := os.Lstat(dest); !errors.Is(err, os.ErrNotExist) {
			return out, fmt.Errorf("%w: 会话中已有同名上传文件 %s", ErrInvalid, filepath.Base(clean))
		}
		if err := os.Rename(clean, dest); err != nil {
			return out, fmt.Errorf("%w: 移动上传文件: %v", ErrUnavailable, err)
		}
		out = append(out, dest)
	}
	return out, nil
}

// clearStaging drops files staged before the server started: no quick
// start can send them any more.
func (s *Service) clearStaging() {
	dir, err := s.stagingPath()
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if private(s.root, false) != nil {
		return
	}
	if info, err := os.Lstat(dir); err != nil || !info.IsDir() {
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		s.log.Warn("quick start staging folder removal failed", "path", dir, "error", err)
	}
}
