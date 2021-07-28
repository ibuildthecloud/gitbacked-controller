package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type watcher struct {
	cancel func()
	c      <-chan watch.Event
}

func (w *watcher) Stop() {
	w.cancel()
}

func (w *watcher) ResultChan() <-chan watch.Event {
	return w.c
}

func (s *Store) getRevision(opts metav1.ListOptions) (int, error) {
	if opts.ResourceVersion == "" {
		return 0, nil
	}
	ret, err := strconv.Atoi(opts.ResourceVersion)
	if err != nil {
		return 0, err
	}
	if ret >= len(s.revisions) {
		return 0, errors.NewBadRequest(fmt.Sprintf("invalid resourceVersion %s", opts.ResourceVersion))
	}
	// start just after requested version
	return ret + 1, nil
}

func (s *Store) Watch(gvk schema.GroupVersionKind, emptyObj client.Object, opts metav1.ListOptions) (watch.Interface, error) {
	rev, err := s.getRevision(opts)
	if err != nil {
		return nil, err
	}

	selector, err := labels.Parse(opts.LabelSelector)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan watch.Event)

	go s.watch(ctx, gvk, emptyObj, c, rev, selector)
	return &watcher{
		cancel: cancel,
		c:      c,
	}, nil
}

func (s *Store) isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
	}

	s.contentLock.RLock()
	stopped := s.stopped
	s.contentLock.RUnlock()
	return stopped
}

func (s *Store) watch(ctx context.Context, gvk schema.GroupVersionKind, emptyObj client.Object, c chan watch.Event, rev int, selector labels.Selector) {
	defer close(c)

	for {
		if s.isDone(ctx) {
			return
		}
		rev = s.readEvents(c, gvk, emptyObj, rev, selector)

		s.contentBroadcast.L.Lock()
		if rev >= len(s.revisions) {
			s.contentBroadcast.Wait()
		}
		s.contentBroadcast.L.Unlock()
	}
}

func (s *Store) readEvents(c chan<- watch.Event, gvk schema.GroupVersionKind, emptyObject client.Object, rev int, selector labels.Selector) int {
	for ; rev < len(s.revisions); rev++ {
		revision := s.revisions[rev]
		sendAll(gvk, watch.Added, c, emptyObject, selector, revision.add)
		sendAll(gvk, watch.Modified, c, emptyObject, selector, revision.modified)
		sendAll(gvk, watch.Deleted, c, emptyObject, selector, revision.deleted)
	}

	return rev
}

func sendAll(gvk schema.GroupVersionKind, event watch.EventType, c chan<- watch.Event, emptyObject client.Object, selector labels.Selector, objs []Object) {
	for _, obj := range objs {
		if gvk.Group != obj.Group || gvk.Kind != obj.Kind {
			continue
		}
		if !selector.Matches(labels.Set(obj.Object.GetLabels())) {
			continue
		}
		ret := emptyObject.DeepCopyObject()
		if err := Convert(ret, obj.Object); err != nil {
			c <- watch.Event{
				Type:   watch.Error,
				Object: &errors.NewBadRequest(err.Error()).ErrStatus,
			}
			continue
		}
		logrus.Infof("WATCH EVENT %s, %v, %s/%s", event, ret.GetObjectKind().GroupVersionKind(),
			ret.(client.Object).GetNamespace(),
			ret.(client.Object).GetName())
		c <- watch.Event{
			Type:   event,
			Object: ret,
		}
	}
}

func Convert(to, from interface{}) error {
	data, err := json.Marshal(from)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, to)
}
