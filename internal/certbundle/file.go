package certbundle

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

const maxBundleBytes = 32 << 20

// Snapshot captures the bytes and metadata used to approve a plan.
type Snapshot struct {
	Content  []byte
	Mode     fs.FileMode
	UID, GID int
	Token    string
}

// Read inspects a regular, non-symlink bundle without modifying it.
func Read(ctx context.Context, path string) (*Snapshot, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	return read(ctx, root, filepath.Base(path))
}

func read(ctx context.Context, root *os.Root, name string) (*Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// os.Root may resolve in-root symlinks even with O_NOFOLLOW. Inspect the
	// directory entry explicitly and verify that the opened inode still matches.
	entry, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !entry.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular non-symlink file", name)
	}
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || !os.SameFile(entry, info) {
		return nil, fmt.Errorf("%s is not a regular file", name)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("cannot inspect ownership of %s", name)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBundleBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBundleBytes {
		return nil, fmt.Errorf("bundle exceeds %d bytes", maxBundleBytes)
	}
	after, err := file.Stat()
	if err != nil {
		return nil, err
	}
	latest, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !latest.Mode().IsRegular() || !os.SameFile(info, latest) || info.Size() != after.Size() || info.ModTime() != after.ModTime() || info.Mode() != after.Mode() {
		return nil, fmt.Errorf("bundle changed while reading; retry")
	}
	s := &Snapshot{Content: data, Mode: info.Mode(), UID: int(stat.Uid), GID: int(stat.Gid)}
	s.Token = Digest(fmt.Appendf(nil, "%s:%d:%d:%d:%d:%d:%d", Digest(data), s.Mode, s.UID, s.GID, stat.Dev, stat.Ino, info.ModTime().UnixNano()))
	return s, nil
}

// Apply validates the approved snapshot again, backs it up, and atomically
// replaces it with the filtered bundle. It never restores old certificates.
// The returned backup path is empty when no change is required.
func Apply(ctx context.Context, path, expected string, absent []string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("bundle path must be absolute")
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }()
	name := filepath.Base(path)
	// Serialize cooperating writers on the original inode. A waiter must replan
	// after another writer replaces the path; the snapshot token detects that.
	lock, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", err
	}
	defer func() { _ = lock.Close() }()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return "", fmt.Errorf("lock bundle: %w", err)
	}
	current, err := read(ctx, root, name)
	if err != nil {
		return "", err
	}
	if expected == "" || current.Token != expected {
		return "", fmt.Errorf("bundle changed since planning; preview and apply again")
	}
	content, removed, err := Filter(current.Content, absent)
	if err != nil {
		return "", err
	}
	if len(removed) == 0 {
		return "", nil
	}
	if _, err := Parse(content); err != nil {
		return "", fmt.Errorf("validate filtered bundle: %w", err)
	}
	backup := name + ".crucible-backup-" + Digest(current.Content)
	if err := backupSnapshot(ctx, root, backup, current); err != nil {
		return "", fmt.Errorf("back up bundle: %w", err)
	}
	temp := ".crucible-cert-" + rand.Text()
	if err := writeExclusive(root, temp, content, current); err != nil {
		return "", err
	}
	defer func() { _ = root.Remove(temp) }()
	// Do not overwrite an OS update or an external edit made during preparation.
	latest, err := read(ctx, root, name)
	if err != nil {
		return "", err
	}
	if latest.Token != expected {
		return "", fmt.Errorf("bundle changed during apply; preview and apply again")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := root.Rename(temp, name); err != nil {
		return "", fmt.Errorf("replace bundle: %w", err)
	}
	installed, err := read(ctx, root, name)
	if err != nil {
		return "", fmt.Errorf("verify installed bundle (backup %s): %w", backup, err)
	}
	if !bytes.Equal(installed.Content, content) || installed.Mode != current.Mode || installed.UID != current.UID || installed.GID != current.GID {
		return "", fmt.Errorf("installed bundle verification failed; inspect %s and backup %s", path, backup)
	}
	return filepath.Join(filepath.Dir(path), backup), nil
}

func backupSnapshot(ctx context.Context, root *os.Root, name string, s *Snapshot) error {
	err := writeExclusive(root, name, s.Content, s)
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrExist) {
		return err
	}
	existing, err := read(ctx, root, name)
	if err != nil {
		return err
	}
	if !bytes.Equal(existing.Content, s.Content) || existing.Mode != s.Mode || existing.UID != s.UID || existing.GID != s.GID {
		return fmt.Errorf("existing backup does not match current bundle: %s", name)
	}
	return nil
}

func writeExclusive(root *os.Root, name string, content []byte, s *Snapshot) (err error) {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			_ = root.Remove(name)
		}
	}()
	if _, err = file.Write(content); err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot inspect temporary file ownership")
	}
	if int(stat.Uid) != s.UID || int(stat.Gid) != s.GID {
		if err = file.Chown(s.UID, s.GID); err != nil {
			return err
		}
	}
	if err = file.Chmod(s.Mode); err != nil {
		return err
	}
	return file.Sync()
}
