module example.com/gitexample

go 1.16

require (
	github.com/ibuildthecloud/gitbacked-controller v0.0.0
	github.com/sirupsen/logrus v1.8.1
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	sigs.k8s.io/controller-runtime v0.9.3
)

replace github.com/ibuildthecloud/gitbacked-controller => ../
