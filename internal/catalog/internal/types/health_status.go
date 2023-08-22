// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/go-multierror"
)

const (
	HealthStatusKind = "HealthStatus"
)

var (
	HealthStatusV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         HealthStatusKind,
	}

	HealthStatusType = HealthStatusV1Alpha1Type

	ValidateHealthStatus = resource.DecodeAndValidate[*pbcatalog.HealthStatus](validateHealthStatus)
)

type DecodedHealthStatus = resource.DecodedResource[*pbcatalog.HealthStatus]

func RegisterHealthStatus(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     HealthStatusV1Alpha1Type,
		Proto:    &pbcatalog.HealthStatus{},
		Validate: ValidateHealthStatus,
	})
}

func validateHealthStatus(dec *DecodedHealthStatus) error {
	var err error

	// Should we allow empty types? I think for now it will be safest to require
	// the type field is set and we can relax this restriction in the future
	// if we deem it desirable.
	if dec.Data.Type == "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "type",
			Wrapped: resource.ErrMissing,
		})
	}

	switch dec.Data.Status {
	case pbcatalog.Health_HEALTH_PASSING,
		pbcatalog.Health_HEALTH_WARNING,
		pbcatalog.Health_HEALTH_CRITICAL,
		pbcatalog.Health_HEALTH_MAINTENANCE:
	default:
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "status",
			Wrapped: errInvalidHealth,
		})
	}

	// Ensure that the HealthStatus' owner is a type that we want to allow. The
	// owner is currently the resource that this HealthStatus applies to. If we
	// change this to be a parent reference within the HealthStatus.Data then
	// we could allow for other owners.
	if dec.Resource.Owner == nil {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "owner",
			Wrapped: resource.ErrMissing,
		})
	} else if !resource.EqualType(dec.Resource.Owner.Type, WorkloadType) && !resource.EqualType(dec.Resource.Owner.Type, NodeType) {
		err = multierror.Append(err, resource.ErrOwnerTypeInvalid{ResourceType: dec.Resource.Id.Type, OwnerType: dec.Resource.Owner.Type})
	}

	return err
}
