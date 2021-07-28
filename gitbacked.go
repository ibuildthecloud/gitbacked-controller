package gitbacked

import (
	"context"
	"time"

	cache3 "github.com/ibuildthecloud/gitbacked-controller/pkg/cache"
	client2 "github.com/ibuildthecloud/gitbacked-controller/pkg/client"
	"github.com/ibuildthecloud/gitbacked-controller/pkg/mapping"
	"github.com/ibuildthecloud/gitbacked-controller/pkg/store"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Options struct {
	Branch       string
	SubDirectory string
	Interval     time.Duration
}

type GitStore struct {
	store *store.Store
}

func (g *GitStore) Close() error {
	if g.store != nil {
		return g.store.Close()
	}
	return nil
}

func (g *GitStore) NewCache(_ *rest.Config, opts cache.Options) (cache.Cache, error) {
	c := client2.NewClient(opts.Scheme, opts.Mapper, g.store)
	return cache3.New(c), nil
}

func (g *GitStore) NewClient(_ cache.Cache, _ *rest.Config, options client.Options, _ ...client.Object) (client.Client, error) {
	return client2.NewClient(options.Scheme, options.Mapper, g.store), nil
}

func (g *GitStore) MapperProvider(c *rest.Config) (meta.RESTMapper, error) {
	return &mapping.Mapper{}, nil
}

func New(ctx context.Context, url string, opts Options) (*GitStore, error) {
	if opts.Interval == 0 {
		opts.Interval = 15 * time.Second
	}

	store, err := store.New(url, opts.Branch, opts.SubDirectory)
	if err != nil {
		return nil, err
	}

	if err := store.Start(ctx, opts.Interval); err != nil {
		return nil, err
	}

	return &GitStore{
		store: store,
	}, nil
}
