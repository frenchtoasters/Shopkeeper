package scope

import (
	"context"

	interactionsv1alpha1 "frenchtoasters.io/shopkeeper/api/v1alpha1"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ShopkeeperScopeParams struct {
	Client client.Client
	Task   *interactionsv1alpha1.Task
}

// ShopkeeperScope defines the basic context for an actuator to operate upon.
type ShopkeeperScope struct {
	client      client.Client
	patchHelper *patch.Helper

	Task *interactionsv1alpha1.Task
}

// NewShopkeeperScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewShopkeeperScope(ctx context.Context, params ShopkeeperScopeParams) (*ShopkeeperScope, error) {
	if params.Task == nil {
		return nil, errors.New("failed to generate new scope from nil Shopkeeper")
	}

	helper, err := patch.NewHelper(params.Task, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ShopkeeperScope{
		client:      params.Client,
		patchHelper: helper,
		Task:        params.Task,
	}, nil
}

// PatchObject persists the keycloak configuration and status.
func (s *ShopkeeperScope) PatchObject() error {
	return s.patchHelper.Patch(context.TODO(), s.Task)
}

// Close closes the current scope persisting the task configuration and status.
func (s *ShopkeeperScope) Close() error {
	return s.PatchObject()
}
