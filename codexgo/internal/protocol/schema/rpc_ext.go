// rpc_ext.go holds decode helper methods on the RPC envelope types defined in
// client_types_gen.go. These are NOT regenerated; they extend the generated
// types with protocol-specific behaviour.
package schema

import "encoding/json"

// DecodeParams unmarshals the Params field into v.
func (r RPCRequest) DecodeParams(v any) error {
	if len(r.Params) == 0 {
		return json.Unmarshal([]byte("null"), v)
	}
	return json.Unmarshal(r.Params, v)
}

// DecodeResult unmarshals the Result field into v.
func (r RPCResponse) DecodeResult(v any) error {
	if len(r.Result) == 0 {
		return json.Unmarshal([]byte("null"), v)
	}
	return json.Unmarshal(r.Result, v)
}

// DecodeParams unmarshals the Params field into v.
func (n RPCNotification) DecodeParams(v any) error {
	if len(n.Params) == 0 {
		return json.Unmarshal([]byte("null"), v)
	}
	return json.Unmarshal(n.Params, v)
}
