package sqldb

import (
	"fmt"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
)

var (
	ExplosiveOffloadNodeStatusRepo OffloadNodeStatusRepo = &explosiveOffloadNodeStatusRepo{}
	ErrOffloadNotSupported                               = fmt.Errorf("offload node status is not supported")
)

type explosiveOffloadNodeStatusRepo struct{}

func (n *explosiveOffloadNodeStatusRepo) IsEnabled() bool {
	return false
}

func (n *explosiveOffloadNodeStatusRepo) Save(string, string, wfv1.Nodes) (string, error) {
	return "", ErrOffloadNotSupported
}

func (n *explosiveOffloadNodeStatusRepo) Get(string, string) (wfv1.Nodes, error) {
	return nil, ErrOffloadNotSupported
}

func (n *explosiveOffloadNodeStatusRepo) List(string) (map[UUIDVersion]wfv1.Nodes, error) {
	return nil, ErrOffloadNotSupported
}

func (n *explosiveOffloadNodeStatusRepo) Delete(string, string) error {
	return ErrOffloadNotSupported
}

func (n *explosiveOffloadNodeStatusRepo) ListOldOffloads(string) (map[string][]string, error) {
	return nil, ErrOffloadNotSupported
}
