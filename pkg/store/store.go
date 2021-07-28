package store

import (
	"bytes"
	"context"
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ibuildthecloud/gitbacked-controller/pkg/git"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/yaml"
)

type ObjectKey struct {
	Kind      string
	Group     string
	Name      string
	Namespace string
}

type Object struct {
	ObjectKey

	Version         string
	ResourceVersion string
	UID             types.UID
	Content         []byte
	Object          *unstructured.Unstructured
	Path            string
}

type Revision struct {
	data     map[ObjectKey]Object
	add      []Object
	deleted  []Object
	modified []Object
}

type Store struct {
	contentLock      sync.RWMutex
	contentBroadcast *sync.Cond

	ctx           context.Context
	url           string
	branch        string
	subDir        string
	repo          *git.Repo
	revisions     []Revision
	currentCommit string
	stopped       bool
}

func New(url, branch, subDir string) (*Store, error) {
	s := &Store{
		url:    url,
		branch: branch,
		subDir: subDir,
		// Add the first two empty revisions to that the revision is always at least 1
		revisions: []Revision{{}, {}},
	}
	s.contentBroadcast = sync.NewCond(&s.contentLock)
	return s, nil
}

func (s *Store) Start(ctx context.Context, interval time.Duration) error {
	repo, err := git.New(ctx, s.url, s.branch)
	if err != nil {
		return err
	}
	s.repo = repo
	s.ctx = ctx

	go s.refresh(interval)

	s.contentLock.Lock()
	defer s.contentLock.Unlock()
	return s.scanAndUpdate()
}

func (s *Store) refresh(interval time.Duration) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(interval):
			if err := s.refreshAndScan(); err != nil {
				logrus.Errorf("failed to update repo: %v", err)
			}
		}
	}
}

func (s *Store) refreshAndScan() error {
	s.contentLock.Lock()
	defer s.contentLock.Unlock()

	commit, err := s.repo.Update(s.ctx)
	if err != nil {
		return err
	}

	if s.currentCommit == commit {
		return nil
	}

	return s.scanAndUpdate()
}

func (s *Store) scanAndUpdate() error {
	commit, files, err := s.scan()
	if err != nil {
		return err
	}

	return s.add(commit, files)
}

func (s *Store) commit(commit string, files map[ObjectKey]Object) {
	defer s.contentBroadcast.Broadcast()

	var (
		rev         = strconv.Itoa(len(s.revisions))
		newRevision = Revision{
			data: map[ObjectKey]Object{},
		}
		currentRev = s.revisions[len(s.revisions)-1]
	)

	for key, obj := range files {
		existingObject, ok := currentRev.data[key]
		if ok {
			if bytes.Equal(existingObject.Content, obj.Content) {
				newRevision.data[key] = existingObject
			} else {
				obj.ResourceVersion = rev
				obj.UID = existingObject.UID
				newRevision.modified = append(newRevision.modified, obj)
				newRevision.data[key] = obj
			}
		} else {
			obj.UID = uuid.NewUUID()
			obj.ResourceVersion = rev
			newRevision.add = append(newRevision.add, obj)
			newRevision.data[key] = obj
		}
	}

	for key, obj := range currentRev.data {
		_, ok := files[key]
		if !ok {
			newRevision.deleted = append(newRevision.deleted, obj)
		}
	}

	// make sure dynamic fields are set
	for _, obj := range newRevision.data {
		obj.Object.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   obj.Group,
			Version: obj.Version,
			Kind:    obj.Kind,
		})
		obj.Object.SetResourceVersion(obj.ResourceVersion)
		obj.Object.SetUID(obj.UID)
	}

	if len(newRevision.add) == 0 &&
		len(newRevision.deleted) == 0 &&
		len(newRevision.modified) == 0 {
		s.currentCommit = commit
		return
	}

	s.revisions = append(s.revisions, newRevision)
	s.currentCommit = commit
	logrus.Infof("Commit: %s", commit)
	for _, obj := range newRevision.add {
		logrus.Infof("-> Added: %s", obj.Path)
	}
	for _, obj := range newRevision.modified {
		logrus.Infof("-> Modified: %s", obj.Path)
	}
	for _, obj := range newRevision.deleted {
		logrus.Infof("-> Deleted: %s", obj.Path)
	}
}

func (s *Store) add(commit string, files []string) error {
	newFiles := map[ObjectKey]Object{}

	for _, file := range files {
		bytes, err := ioutil.ReadFile(file)
		if err != nil {
			logrus.Errorf("Failed to read %s, skipping: %v", file, err)
			continue
		}
		data := map[string]interface{}{}
		if err := yaml.Unmarshal(bytes, &data); err != nil {
			logrus.Errorf("Failed to unmarshal %s, skipping: %v", file, err)
			continue
		}

		unstr := &unstructured.Unstructured{
			Object: data,
		}
		gvk := unstr.GroupVersionKind()

		obj := Object{
			ObjectKey: ObjectKey{
				Kind:      gvk.Kind,
				Group:     gvk.Group,
				Name:      unstr.GetName(),
				Namespace: unstr.GetNamespace(),
			},
			Version: gvk.Version,
			Content: bytes,
			Object:  unstr,
			Path:    file,
		}
		if obj.Kind == "" ||
			obj.Name == "" ||
			obj.Version == "" {
			continue
		}
		newFiles[obj.ObjectKey] = obj
	}

	s.commit(commit, newFiles)
	return nil
}

func (s *Store) scan() (string, []string, error) {
	commit, err := s.repo.Head(s.ctx)
	if err != nil {
		return "", nil, err
	}

	var paths []string
	err = filepath.WalkDir(filepath.Join(s.repo.Dir, s.subDir), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		pathLower := strings.ToLower(path)
		if strings.HasSuffix(pathLower, ".yaml") || strings.HasSuffix(pathLower, ".yml") {
			paths = append(paths, path)
		}
		return nil
	})

	return commit, paths, err
}

func (s *Store) Close() error {
	defer s.contentBroadcast.Broadcast()
	s.contentLock.Lock()
	defer s.contentLock.Unlock()

	s.stopped = true
	if s.repo == nil {
		return nil
	}
	return s.repo.Close()
}
