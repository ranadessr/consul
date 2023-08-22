// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"math"
	"sort"

	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/go-multierror"
)

const (
	WorkloadKind = "Workload"
)

var (
	WorkloadV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         WorkloadKind,
	}

	WorkloadType = WorkloadV1Alpha1Type

	ValidateWorkload = resource.DecodeAndValidate[*pbcatalog.Workload](validateWorkload)
)

type DecodedWorkload = resource.DecodedResource[*pbcatalog.Workload]

func RegisterWorkload(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     WorkloadV1Alpha1Type,
		Proto:    &pbcatalog.Workload{},
		Validate: ValidateWorkload,
	})
}

func validateWorkload(dec *DecodedWorkload) error {
	var err error

	// Validate that the workload has at least one port
	if len(dec.Data.Ports) < 1 {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "ports",
			Wrapped: resource.ErrEmpty,
		})
	}

	var meshPorts []string

	// Validate the Workload Ports
	for portName, port := range dec.Data.Ports {
		if portNameErr := validatePortName(portName); portNameErr != nil {
			err = multierror.Append(err, resource.ErrInvalidMapKey{
				Map:     "ports",
				Key:     portName,
				Wrapped: portNameErr,
			})
		}

		// disallow port 0 for now
		if port.Port < 1 || port.Port > math.MaxUint16 {
			err = multierror.Append(err, resource.ErrInvalidMapValue{
				Map: "ports",
				Key: portName,
				Wrapped: resource.ErrInvalidField{
					Name:    "port",
					Wrapped: errInvalidPhysicalPort,
				},
			})
		}

		// Collect the list of mesh ports
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			meshPorts = append(meshPorts, portName)
		}
	}

	if len(meshPorts) > 1 {
		sort.Strings(meshPorts)
		err = multierror.Append(err, resource.ErrInvalidField{
			Name: "ports",
			Wrapped: errTooMuchMesh{
				Ports: meshPorts,
			},
		})
	}

	// If the workload is mesh enabled then a valid identity must be provided.
	// If not mesh enabled but a non-empty identity is provided then we still
	// validate that its valid.
	if len(meshPorts) > 0 && dec.Data.Identity == "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "identity",
			Wrapped: resource.ErrMissing,
		})
	} else if dec.Data.Identity != "" && !isValidDNSLabel(dec.Data.Identity) {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "identity",
			Wrapped: errNotDNSLabel,
		})
	}

	// Validate workload locality
	if dec.Data.Locality != nil && dec.Data.Locality.Region == "" && dec.Data.Locality.Zone != "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "locality",
			Wrapped: errLocalityZoneNoRegion,
		})
	}

	// Node associations are optional but if present the name should
	// be a valid DNS label.
	if dec.Data.NodeName != "" {
		if !isValidDNSLabel(dec.Data.NodeName) {
			err = multierror.Append(err, resource.ErrInvalidField{
				Name:    "node_name",
				Wrapped: errNotDNSLabel,
			})
		}
	}

	if len(dec.Data.Addresses) < 1 {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "addresses",
			Wrapped: resource.ErrEmpty,
		})
	}

	// Validate Workload Addresses
	for idx, addr := range dec.Data.Addresses {
		if addrErr := validateWorkloadAddress(addr, dec.Data.Ports); addrErr != nil {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:    "addresses",
				Index:   idx,
				Wrapped: addrErr,
			})
		}
	}

	return err
}
