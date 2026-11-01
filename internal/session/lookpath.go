package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrNoExecutable: a sidecar's or LookPath's executable is not in the
// session's PATH, or not an executable file.
var ErrNoExecutable = errors.New("executable not found")

// LookPath finds name the way a sidecar's argv[0] is found: a name with a
// slash relative to the session's directory, any other in the PATH of the
// session's launch environment (the launch config's PATH, else the
// server's).
func (s *Session) LookPath(name string) (string, error) {
	return lookPath(name, s.info.Cwd, s.childEnv(nil))
}

// Getenv returns variable name of the session's launch environment: the
// launch config's value, else the server's.
func (s *Session) Getenv(name string) string {
	if v, ok := s.options.env[name]; ok {
		return v
	}
	return os.Getenv(name)
}

// lookPath resolves file against PATH in env, relative directories and
// paths against cwd, like exec.Command run in cwd would but with the
// child's PATH rather than the server's. Empty PATH entries are skipped.
func lookPath(file, cwd string, env []string) (string, error) {
	if strings.Contains(file, "/") {
		if !filepath.IsAbs(file) {
			file = filepath.Join(cwd, file)
		}
		if executable(file) {
			return file, nil
		}
		return "", fmt.Errorf("%w: %s", ErrNoExecutable, file)
	}
	for _, dir := range filepath.SplitList(envValue(env, "PATH")) {
		if dir == "" {
			continue
		}
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(cwd, dir)
		}
		if path := filepath.Join(dir, file); executable(path) {
			return path, nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrNoExecutable, file)
}

func executable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

// envValue is the last value of name in env, as the child would see it.
func envValue(env []string, name string) string {
	value := ""
	for _, entry := range env {
		if k, v, ok := strings.Cut(entry, "="); ok && k == name {
			value = v
		}
	}
	return value
}
