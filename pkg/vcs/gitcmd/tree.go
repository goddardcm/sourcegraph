package gitcmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/sourcegraph/sourcegraph/pkg/api"
	"github.com/sourcegraph/sourcegraph/pkg/vcs"
	"github.com/sourcegraph/sourcegraph/pkg/vcs/util"
)

// Lstat returns a FileInfo describing the named file at commit. If the file is a symbolic link, the
// returned FileInfo describes the symbolic link.  Lstat makes no attempt to follow the link.
func (r *Repository) Lstat(ctx context.Context, commit api.CommitID, path string) (os.FileInfo, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Git: Lstat")
	span.SetTag("Commit", commit)
	span.SetTag("Path", path)
	defer span.Finish()

	if err := checkSpecArgSafety(string(commit)); err != nil {
		return nil, err
	}

	path = filepath.Clean(util.Rel(path))

	if path == "." {
		// Special case root, which is not returned by `git ls-tree`.
		return &util.FileInfo{Mode_: os.ModeDir}, nil
	}

	fis, err := r.lsTree(ctx, commit, path, false)
	if err != nil {
		return nil, err
	}
	if len(fis) == 0 {
		return nil, &os.PathError{Op: "ls-tree", Path: path, Err: os.ErrNotExist}
	}

	return fis[0], nil
}

// Stat returns a FileInfo describing the named file at commit.
func (r *Repository) Stat(ctx context.Context, commit api.CommitID, path string) (os.FileInfo, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Git: Stat")
	span.SetTag("Commit", commit)
	span.SetTag("Path", path)
	defer span.Finish()

	if err := checkSpecArgSafety(string(commit)); err != nil {
		return nil, err
	}

	path = util.Rel(path)

	fi, err := r.Lstat(ctx, commit, path)
	if err != nil {
		return nil, err
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		// Deref symlink.
		b, err := r.readFileBytes(ctx, commit, path)
		if err != nil {
			return nil, err
		}
		fi2, err := r.Lstat(ctx, commit, string(b))
		if err != nil {
			return nil, err
		}
		fi2.(*util.FileInfo).Name_ = fi.Name()
		return fi2, nil
	}

	return fi, nil
}

// ReadDir reads the contents of the named directory at commit.
func (r *Repository) ReadDir(ctx context.Context, commit api.CommitID, path string, recurse bool) ([]os.FileInfo, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Git: ReadDir")
	span.SetTag("Commit", commit)
	span.SetTag("Path", path)
	span.SetTag("Recurse", recurse)
	defer span.Finish()

	if err := checkSpecArgSafety(string(commit)); err != nil {
		return nil, err
	}

	if path != "" {
		// Trailing slash is necessary to ls-tree under the dir (not just
		// to list the dir's tree entry in its parent dir).
		path = filepath.Clean(util.Rel(path)) + "/"
	}
	return r.lsTree(ctx, commit, path, recurse)
}

// lsTreeRootCache caches the result of running `git ls-tree ...` on a repository's root path
// (because non-root paths are likely to have a lower cache hit rate). It is intended to improve the
// perceived performance of large monorepos, where the tree for a given repo+commit (usually the
// repo's latest commit on default branch) will be requested frequently and would take multiple
// seconds to compute if uncached.
var (
	lsTreeRootCacheMu sync.Mutex
	lsTreeRootCache   = lru.New(5)
)

// lsTree returns ls of tree at path.
func (r *Repository) lsTree(ctx context.Context, commit api.CommitID, path string, recurse bool) ([]os.FileInfo, error) {
	if path != "" || !recurse {
		// Only cache the root recursive ls-tree.
		return r.lsTreeUncached(ctx, commit, path, recurse)
	}

	key := string(r.repoURI) + ":" + string(commit) + ":" + path
	lsTreeRootCacheMu.Lock()
	v, ok := lsTreeRootCache.Get(key)
	lsTreeRootCacheMu.Unlock()
	var entries []os.FileInfo
	if ok {
		// Cache hit.
		entries = v.([]os.FileInfo)
	} else {
		// Cache miss.
		var err error
		start := time.Now()
		entries, err = r.lsTreeUncached(ctx, commit, path, recurse)
		if err != nil {
			return nil, err
		}

		// It's only worthwhile to cache if the operation took a while and returned a lot of
		// data. This is a heuristic.
		if time.Since(start) > 500*time.Millisecond && len(entries) > 5000 {
			lsTreeRootCacheMu.Lock()
			lsTreeRootCache.Add(key, entries)
			lsTreeRootCacheMu.Unlock()
		}
	}
	return entries, nil
}

func (r *Repository) lsTreeUncached(ctx context.Context, commit api.CommitID, path string, recurse bool) ([]os.FileInfo, error) {
	r.ensureAbsCommit(commit)

	// Don't call filepath.Clean(path) because ReadDir needs to pass
	// path with a trailing slash.

	if err := checkSpecArgSafety(path); err != nil {
		return nil, err
	}

	args := []string{
		"ls-tree",
		"--long", // show size
		"--full-name",
		"-z",
		string(commit),
	}
	if recurse {
		args = append(args, "-r", "-t")
	}
	if path != "" {
		args = append(args, "--", filepath.ToSlash(path))
	}
	cmd := r.command("git", args...)
	out, err := cmd.CombinedOutput(ctx)
	if err != nil {
		if bytes.Contains(out, []byte("exists on disk, but not in")) {
			return nil, &os.PathError{Op: "ls-tree", Path: filepath.ToSlash(path), Err: os.ErrNotExist}
		}
		return nil, fmt.Errorf("exec %v failed: %s. Output was:\n\n%s", cmd.Args, err, out)
	}

	if len(out) == 0 {
		return nil, &os.PathError{Op: "git ls-tree", Path: path, Err: os.ErrNotExist}
	}

	trimPath := strings.TrimPrefix(path, "./")
	prefixLen := strings.LastIndexByte(trimPath, '/') + 1
	lines := strings.Split(string(out), "\x00")
	fis := make([]os.FileInfo, len(lines)-1)
	for i, line := range lines {
		if i == len(lines)-1 {
			// last entry is empty
			continue
		}

		tabPos := strings.IndexByte(line, '\t')
		if tabPos == -1 {
			return nil, fmt.Errorf("invalid `git ls-tree` output: %q", out)
		}
		info := strings.SplitN(line[:tabPos], " ", 4)
		name := line[tabPos+1:]
		if len(name) < len(trimPath) {
			// This is in a submodule; return the original path to avoid a slice out of bounds panic
			// when setting the FileInfo._Name below.
			name = trimPath
		}

		if len(info) != 4 {
			return nil, fmt.Errorf("invalid `git ls-tree` output: %q", out)
		}
		typ := info[1]
		oid := info[2]
		if !vcs.IsAbsoluteRevision(oid) {
			return nil, fmt.Errorf("invalid `git ls-tree` oid output: %q", oid)
		}

		sizeStr := strings.TrimSpace(info[3])
		var size int64
		if sizeStr != "-" {
			// Size of "-" indicates a dir or submodule.
			size, err = strconv.ParseInt(sizeStr, 10, 64)
			if err != nil || size < 0 {
				return nil, fmt.Errorf("invalid `git ls-tree` size output: %q (error: %s)", sizeStr, err)
			}
		}

		var sys interface{}
		mode, err := strconv.ParseInt(info[0], 8, 32)
		if err != nil {
			return nil, err
		}
		switch typ {
		case "blob":
			const gitModeSymlink = 020000
			if mode&gitModeSymlink != 0 {
				mode = int64(os.ModeSymlink)
			} else {
				// Regular file.
				mode = mode | 0644
			}
		case "commit":
			mode = mode | vcs.ModeSubmodule
			cmd := r.command("git", "config", "--get", "submodule."+name+".url")
			url := "" // url is not available if submodules are not initialized
			if out, err := cmd.Output(ctx); err == nil {
				url = string(bytes.TrimSpace(out))
			}
			sys = vcs.SubmoduleInfo{
				URL:      url,
				CommitID: api.CommitID(oid),
			}
		case "tree":
			mode = mode | int64(os.ModeDir)
		}

		fis[i] = &util.FileInfo{
			// This returns the full relative path (e.g. "path/to/file.go") when the path arg is "./"
			// This behavior is necessary to construct the file tree.
			// In all other cases, it returns the basename (e.g. "file.go").
			Name_: name[prefixLen:],
			Mode_: os.FileMode(mode),
			Size_: size,
			Sys_:  sys,
		}
	}
	util.SortFileInfosByName(fis)

	return fis, nil
}