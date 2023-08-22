// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	FailoverPolicyKind = "FailoverPolicy"
)

var (
	FailoverPolicyV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         FailoverPolicyKind,
	}

	FailoverPolicyType = FailoverPolicyV1Alpha1Type

	ValidateFailoverPolicy = resource.DecodeAndValidate[*pbcatalog.FailoverPolicy](validateFailoverPolicy)
	MutateFailoverPolicy   = resource.DecodeAndMutate[*pbcatalog.FailoverPolicy](mutateFailoverPolicy)
)

type DecodedFailoverPolicy = resource.DecodedResource[*pbcatalog.FailoverPolicy]

func RegisterFailoverPolicy(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     FailoverPolicyV1Alpha1Type,
		Proto:    &pbcatalog.FailoverPolicy{},
		Mutate:   MutateFailoverPolicy,
		Validate: ValidateFailoverPolicy,
	})
}

func mutateFailoverPolicy(dec *DecodedFailoverPolicy) (bool, error) {
	changed := false

	// Handle eliding empty configs.
	if dec.Data.Config != nil && dec.Data.Config.IsEmpty() {
		dec.Data.Config = nil
		changed = true
	}
	for port, pc := range dec.Data.PortConfigs {
		if pc.IsEmpty() {
			delete(dec.Data.PortConfigs, port)
			changed = true
		}
	}
	if len(dec.Data.PortConfigs) == 0 {
		dec.Data.PortConfigs = nil
		changed = true
	}

	// TODO(rb): normalize dest ref tenancies

	return changed, nil
}

func validateFailoverPolicy(dec *DecodedFailoverPolicy) error {
	var merr error

	if dec.Data.Config == nil && len(dec.Data.PortConfigs) == 0 {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "config",
			Wrapped: fmt.Errorf("at least one of config or port_configs must be set"),
		})
	}

	if dec.Data.Config != nil {
		for _, err := range validateFailoverConfig(dec.Data.Config, false) {
			merr = multierror.Append(merr, resource.ErrInvalidField{
				Name:    "config",
				Wrapped: err,
			})
		}
	}

	for portName, pc := range dec.Data.PortConfigs {
		if portNameErr := validatePortName(portName); portNameErr != nil {
			merr = multierror.Append(merr, resource.ErrInvalidMapKey{
				Map:     "port_configs",
				Key:     portName,
				Wrapped: portNameErr,
			})
		}

		for _, err := range validateFailoverConfig(pc, true) {
			merr = multierror.Append(merr, resource.ErrInvalidMapValue{
				Map:     "port_configs",
				Key:     portName,
				Wrapped: err,
			})
		}

		// TODO: should sameness group be a ref once that's a resource?
	}

	return merr
}

func validateFailoverConfig(config *pbcatalog.FailoverConfig, ported bool) []error {
	var errs []error

	if (len(config.Destinations) > 0) == (config.SamenessGroup != "") {
		errs = append(errs, resource.ErrInvalidField{
			Name:    "destinations",
			Wrapped: fmt.Errorf("exactly one of destinations or sameness_group should be set"),
		})
	}
	for i, dest := range config.Destinations {
		for _, err := range validateFailoverPolicyDestination(dest, ported) {
			errs = append(errs, resource.ErrInvalidListElement{
				Name:    "destinations",
				Index:   i,
				Wrapped: err,
			})
		}
	}

	// TODO: validate sameness group requirements

	return errs
}

func validateFailoverPolicyDestination(dest *pbcatalog.FailoverDestination, ported bool) []error {
	var errs []error
	if dest.Ref == nil {
		errs = append(errs, resource.ErrInvalidField{
			Name:    "ref",
			Wrapped: resource.ErrMissing,
		})
	} else if !resource.EqualType(dest.Ref.Type, ServiceType) {
		errs = append(errs, resource.ErrInvalidField{
			Name: "ref",
			Wrapped: resource.ErrInvalidReferenceType{
				AllowedType: ServiceType,
			},
		})
	} else if dest.Ref.Section != "" {
		errs = append(errs, resource.ErrInvalidField{
			Name: "ref",
			Wrapped: resource.ErrInvalidField{
				Name:    "section",
				Wrapped: errors.New("section not supported for failover policy dest refs"),
			},
		})
	}

	// NOTE: Destinations here cannot define ports. Port equality is
	// assumed and will be reconciled.
	if dest.Port != "" {
		if ported {
			if portNameErr := validatePortName(dest.Port); portNameErr != nil {
				errs = append(errs, resource.ErrInvalidField{
					Name:    "port",
					Wrapped: portNameErr,
				})
			}
		} else {
			errs = append(errs, resource.ErrInvalidField{
				Name:    "port",
				Wrapped: fmt.Errorf("ports cannot be specified explicitly for the general failover section since it relies upon port alignment"),
			})
		}
	}

	hasPeer := false
	if dest.Ref != nil {
		hasPeer = dest.Ref.Tenancy.PeerName != "local"
	}

	if hasPeer && dest.Datacenter != "" {
		errs = append(errs, resource.ErrInvalidField{
			Name:    "datacenter",
			Wrapped: fmt.Errorf("ref.tenancy.peer_name and datacenter are mutually exclusive fields"),
		})
	}

	return errs
}

// SimplifyFailoverPolicy fully populates the PortConfigs map and clears the
// Configs map using the provided Service.
func SimplifyFailoverPolicy(svc *pbcatalog.Service, failover *pbcatalog.FailoverPolicy) *pbcatalog.FailoverPolicy {
	if failover == nil {
		panic("failover is required")
	}
	if svc == nil {
		panic("service is required")
	}

	// Copy so we can edit it.
	dup := proto.Clone(failover)
	failover = dup.(*pbcatalog.FailoverPolicy)

	if failover.PortConfigs == nil {
		failover.PortConfigs = make(map[string]*pbcatalog.FailoverConfig)
	}

	for _, port := range svc.Ports {
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			continue // skip
		}

		if pc, ok := failover.PortConfigs[port.TargetPort]; ok {
			for i, dest := range pc.Destinations {
				// Assume port alignment.
				if dest.Port == "" {
					dest.Port = port.TargetPort
					pc.Destinations[i] = dest
				}
			}
			continue
		}

		if failover.Config != nil {
			// Duplicate because each port will get this uniquely.
			pc2 := proto.Clone(failover.Config).(*pbcatalog.FailoverConfig)
			for _, dest := range pc2.Destinations {
				dest.Port = port.TargetPort
			}
			failover.PortConfigs[port.TargetPort] = pc2
		}
	}

	if failover.Config != nil {
		failover.Config = nil
	}

	return failover
}
