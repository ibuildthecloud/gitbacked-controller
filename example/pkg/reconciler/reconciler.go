package reconciler

import (
	"context"

	v1 "example.com/gitexample/pkg/apis/example.com/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var replicator v1.Replicator
	if err := r.Get(ctx, req.NamespacedName, &replicator); err != nil {
		return ctrl.Result{}, err
	}

	var replicated v1.ReplicatedList
	if err := r.List(ctx, &replicated, client.MatchingLabelsSelector{
		Selector: labels.SelectorFromSet(replicator.Selector),
	}); err != nil {
		return ctrl.Result{}, err
	}

	for i := 0; i < replicator.Count-len(replicated.Items); i++ {
		newObj := v1.Replicated{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: replicator.Name + "-",
				Labels:       replicator.Selector,
			},
		}
		if err := ctrl.SetControllerReference(&replicator, &newObj, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.Create(ctx, &newObj); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}
