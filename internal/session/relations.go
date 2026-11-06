package session

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Label limits. Labels are caller-defined metadata for scripts: a few short
// keys per session, kept in memory with the session.
const (
	MaxLabels          = 16
	MaxLabelValueRunes = 256
)

var labelKey = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

// ValidateLabels checks keys, values and the count of labels, so a create
// request can be rejected before anything starts.
func (m *Manager) ValidateLabels(labels map[string]string) error {
	return validateLabels(labels)
}

func validateLabels(labels map[string]string) error {
	if len(labels) > MaxLabels {
		return fmt.Errorf("%w: 每个会话最多 %d 个标签", ErrInvalid, MaxLabels)
	}
	for key, value := range labels {
		if err := validateLabel(key, value); err != nil {
			return err
		}
	}
	return nil
}

func validateLabel(key, value string) error {
	if !labelKey.MatchString(key) {
		return fmt.Errorf("%w: 标签名 %q 须匹配 [a-z0-9][a-z0-9._-]{0,62}", ErrInvalid, key)
	}
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > MaxLabelValueRunes || strings.ContainsFunc(value, unicode.IsControl) {
		return fmt.Errorf("%w: 标签 %s 的值须不超过 %d 个字符且不含控制字符", ErrInvalid, key, MaxLabelValueRunes)
	}
	return nil
}

// UpdateLabels applies set, then remove, to session id's labels as one
// change. An invalid key or value, or a result over MaxLabels, changes
// nothing.
func (m *Manager) UpdateLabels(id string, set map[string]string, remove []string) (Info, error) {
	for key, value := range set {
		if err := validateLabel(key, value); err != nil {
			return Info{}, err
		}
	}
	for _, key := range remove {
		if !labelKey.MatchString(key) {
			return Info{}, fmt.Errorf("%w: 标签名 %q 须匹配 [a-z0-9][a-z0-9._-]{0,62}", ErrInvalid, key)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return Info{}, ErrNotFound
	}
	s.mu.Lock()
	next := maps.Clone(s.info.Labels)
	if next == nil {
		next = make(map[string]string, len(set))
	}
	maps.Copy(next, set)
	for _, key := range remove {
		delete(next, key)
	}
	if len(next) > MaxLabels {
		s.mu.Unlock()
		return Info{}, fmt.Errorf("%w: 每个会话最多 %d 个标签", ErrInvalid, MaxLabels)
	}
	if maps.Equal(next, s.info.Labels) || (len(next) == 0 && len(s.info.Labels) == 0) {
		info := s.infoLocked()
		s.mu.Unlock()
		return info, nil
	}
	if len(next) == 0 {
		next = nil
	}
	s.info.Labels = next
	info := s.infoLocked()
	s.broadcastLocked(s.stateLocked())
	s.mu.Unlock()
	m.publishLocked(Event{Type: "updated", Session: &info})
	return info, nil
}

// SetParent records that session parent's process created session id. It
// is set once, by the controller, from the local socket's peer process; an
// unknown session is ignored.
func (m *Manager) SetParent(id, parent string) {
	if parent == "" || parent == id {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return
	}
	s.mu.Lock()
	if s.info.ParentID != "" {
		s.mu.Unlock()
		return
	}
	s.info.ParentID = parent
	info := s.infoLocked()
	s.broadcastLocked(s.stateLocked())
	s.mu.Unlock()
	m.publishLocked(Event{Type: "updated", Session: &info})
}

// Children lists the sessions whose parent is id, oldest first.
func (m *Manager) Children(id string) []string {
	var children []Info
	for _, info := range m.List() {
		if info.ParentID == id {
			children = append(children, info)
		}
	}
	slices.SortStableFunc(children, func(a, b Info) int { return a.CreatedAt.Compare(b.CreatedAt) })
	ids := make([]string, 0, len(children))
	for _, child := range children {
		ids = append(ids, child.ID)
	}
	return ids
}
