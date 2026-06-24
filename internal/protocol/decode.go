package protocol

import (
	"encoding/json"
	"fmt"
)

func DecodeServerRequest(method string, params json.RawMessage) (any, error) {
	switch method {
	case MethodItemCommandExecutionRequestApproval:
		var req CommandExecutionApprovalRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		return req, nil
	case MethodItemFileChangeRequestApproval:
		var req FileChangeApprovalRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		return req, nil
	case MethodItemPermissionsRequestApproval:
		var req PermissionsApprovalRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		return req, nil
	case MethodItemToolRequestUserInput:
		var req UserInputRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		return req, nil
	case MethodItemMCPRequestApproval:
		var req MCPToolCallApprovalRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		return req, nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedServerRequest, method)
	}
}
