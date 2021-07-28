package git

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type Repo struct {
	Dir    string
	url    string
	branch string
}

func (r *Repo) Close() error {
	return os.RemoveAll(r.Dir)
}

func (r *Repo) Update(ctx context.Context) (string, error) {
	_, err := git(ctx, r.Dir, "pull", "-r")
	if err != nil {
		return "", err
	}

	return r.Head(ctx)
}

func (r *Repo) Add(ctx context.Context, path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := ioutil.WriteFile(path, data, 0644); err != nil {
		return err
	}
	_, err := git(ctx, r.Dir, "add", path)
	if err != nil {
		os.Remove(path)
		return err
	}

	return r.commitAndPush(ctx)
}

func (r *Repo) Delete(ctx context.Context, path string) error {
	_, err := git(ctx, r.Dir, "rm", "-f", path)
	if err != nil {
		return err
	}

	return r.commitAndPush(ctx)
}

func (r *Repo) commitAndPush(ctx context.Context) error {
	_, err := git(ctx, r.Dir, "commit", "-m", "controller update")
	if err != nil {
		git(ctx, r.Dir, "reset", "--hard", "origin/HEAD")
		return err
	}

	_, err = git(ctx, r.Dir, "push")
	if err != nil {
		git(ctx, r.Dir, "reset", "--hard", "origin/HEAD")
		return err
	}

	return nil
}

func (r *Repo) Head(ctx context.Context) (string, error) {
	buf, err := git(ctx, r.Dir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(buf.String()), nil
}

func New(ctx context.Context, url, branch string) (*Repo, error) {
	d, err := ioutil.TempDir("", "gitbacked-controller-")
	if err != nil {
		return nil, err
	}

	if err := clone(ctx, url, branch, d); err != nil {
		return nil, err
	}

	return &Repo{
		Dir:    d,
		url:    url,
		branch: branch,
	}, nil
}

func clone(ctx context.Context, url, branch, dir string) error {
	args := []string{"clone"}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, url, dir)

	_, err := git(ctx, "", args...)
	return err
}

func git(ctx context.Context, dir string, args ...string) (*bytes.Buffer, error) {
	logrus.Info("git ", strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr

	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	outBuffer := &bytes.Buffer{}
	eg := errgroup.Group{}
	eg.Go(func() error {
		_, err := io.Copy(outBuffer, io.TeeReader(out, os.Stdout))
		return err
	})

	err = cmd.Run()
	if err != nil {
		logrus.Error("git ", strings.Join(args, " "), ":", err)
	}

	_ = eg.Wait()
	return outBuffer, err
}
