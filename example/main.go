package main

import (
	"flag"
	"time"

	v1 "example.com/gitexample/pkg/apis/example.com/v1"
	"example.com/gitexample/pkg/reconciler"
	"github.com/ibuildthecloud/gitbacked-controller"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	url      = flag.String("url", "", "URL to git repo")
	branch   = flag.String("branch", "", "Branch to pull from and push to")
	interval = flag.Duration("interval", 15*time.Second, "how often to poll git")
	subdir   = flag.String("subdir", "", "subdirectory in git to operate on")
)

func main() {
	flag.Parse()
	ctx := ctrl.SetupSignalHandler()

	if *url == "" {
		logrus.Fatal("-url is required")
	}

	git, err := gitbacked.New(ctx, *url, gitbacked.Options{
		Branch:       *branch,
		SubDirectory: *subdir,
		Interval:     *interval,
	})
	if err != nil {
		logrus.Fatal(err)
	}
	defer git.Close()

	//ctrl.SetLogger(zap.New())

	scheme := runtime.NewScheme()
	err = v1.AddToScheme(scheme)
	if err != nil {
		logrus.Fatal(err)
	}

	mgr, err := ctrl.NewManager(&rest.Config{}, ctrl.Options{
		Scheme:         scheme,
		NewClient:      git.NewClient,
		NewCache:       git.NewCache,
		MapperProvider: git.MapperProvider,
	})
	if err != nil {
		logrus.Fatal(err)
	}

	err = ctrl.NewControllerManagedBy(mgr).
		For(&v1.Replicator{}).
		Owns(&v1.Replicated{}).
		Complete(&reconciler.Reconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		})
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Info("Starting")
	if err := mgr.Start(ctx); err != nil {
		logrus.Fatal(err)
	}
}
