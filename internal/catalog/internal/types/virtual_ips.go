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
	VirtualIPsKind = "VirtualIPs"
)

var (
	VirtualIPsV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         VirtualIPsKind,
	}

	VirtualIPsType = VirtualIPsV1Alpha1Type

	ValidateVirtualIPs = resource.DecodeAndValidate[*pbcatalog.VirtualIPs](validateVirtualIPs)
)

type DecodedVirtualIPs = resource.DecodedResource[*pbcatalog.VirtualIPs]

func RegisterVirtualIPs(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     VirtualIPsV1Alpha1Type,
		Proto:    &pbcatalog.VirtualIPs{},
		Validate: ValidateVirtualIPs,
	})
}

func validateVirtualIPs(dec *DecodedVirtualIPs) error {
	var err error
	for idx, ip := range dec.Data.Ips {
		if vipErr := validateIPAddress(ip.Address); vipErr != nil {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:  "ips",
				Index: idx,
				Wrapped: resource.ErrInvalidField{
					Name:    "address",
					Wrapped: vipErr,
				},
			})
		}
	}
	return err
}
