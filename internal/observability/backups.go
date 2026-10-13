package observability

import (
	"os"
	"path/filepath"
	"sort"
)

// Size rotation belongs to lumberjack; deletion stays under the sink lock so
// that pruning cannot race its asynchronous mill or remove symlink backups.
func (s *Sink) limitBackups() error {
	if s.backups == 0 {
		return nil
	}
	dir := filepath.Dir(s.path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, entry := range entries {
		if entry.Name() != "iolinkd.jsonl" && diagnosticFile(entry.Name()) && entry.Type().IsRegular() {
			names = append(names, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for i := s.backups; i < len(names); i++ {
		if err := os.Remove(filepath.Join(dir, names[i])); err != nil {
			return err
		}
	}
	return nil
}
