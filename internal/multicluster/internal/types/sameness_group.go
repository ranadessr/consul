// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v1alpha1"
)

func RegisterSamenessGroup(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbmulticluster.SamenessGroupType,
		Proto: &pbmulticluster.SamenessGroup{},
		Scope: resource.ScopePartition,
	})
}
