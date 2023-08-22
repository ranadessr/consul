package resource

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/protobuf/proto"
)

// DecodedValidationHook is the function signature needed for usage with the DecodeAndValidate function
type DecodedValidationHook[T proto.Message] func(*DecodedResource[T]) error

// DecodeAndValidate will generate a validation hook function that decodes the specified type and
// passes it off to another validation hook. This is mainly a convenience to avoid many other
// validation hooks needing to attempt decoding the data and erroring in a consistent manner.
func DecodeAndValidate[T proto.Message](fn DecodedValidationHook[T]) ValidationHook {
	return func(res *pbresource.Resource) error {
		decoded, err := Decode[T](res)
		if err != nil {
			return err
		}

		return fn(decoded)
	}
}

// DecodedMutationHook is the function signature needed for usage with the DecodeAndMutate function
// The boolean return value indicates whether the Data field within the DecodedResource was modified.
// When true, the DecodeAndMutate hook function will automatically re-encode the Any data and store
// it on the internal Resource's Data field.
type DecodedMutationHook[T proto.Message] func(*DecodedResource[T]) (bool, error)

// DecodeAndMutate will generate a MutationHook that decodes the specified type and passes it
// off to another mutation hook. This is mainly a convenience to avoid other mutation hooks
// needing to decode and potentially reencode the Any data. When the inner mutation hook returns
// no error and that the Data was modified (true for the boolean return value), the generated
// hook will reencode the Any data back into the Resource wrapper
func DecodeAndMutate[T proto.Message](fn DecodedMutationHook[T]) MutationHook {
	return func(res *pbresource.Resource) error {
		decoded, err := Decode[T](res)
		if err != nil {
			return err
		}

		modified, err := fn(decoded)
		if err != nil {
			return err
		}

		if modified {
			return decoded.Resource.Data.MarshalFrom(decoded.Data)
		}
		return nil
	}
}

// DecodeWriteAuthorizationHook is the function signature needed for usage with the DecodeAndAuthorizeWrite
// function.
type DecodedWriteAuthorizationHook[T proto.Message] func(acl.Authorizer, *acl.AuthorizerContext, *DecodedResource[T]) error

// DecodeAndAuthorizeWrite will generate an ACLAuthorizeWriteHook that decodes the specified type and passes
// it off to another authorization hook. This is mainly a convenience to avoid many other write authorization
// hooks needing to attempt decoding the data and erroring in a consistent manner.
func DecodeAndAuthorizeWrite[T proto.Message](fn DecodedWriteAuthorizationHook[T]) ACLAuthorizeWriteHook {
	return func(authz acl.Authorizer, ctx *acl.AuthorizerContext, res *pbresource.Resource) error {
		decoded, err := Decode[T](res)
		if err != nil {
			return err
		}

		return fn(authz, ctx, decoded)
	}
}
