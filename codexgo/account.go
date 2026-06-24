package codexgo

import (
	"context"
	"fmt"

	"github.com/zealbase/codex-app-server-go/codexgo/internal/protocol"
)

// PlanType is the ChatGPT subscription plan associated with an account.
type PlanType string

const (
	PlanTypeFree                        PlanType = "free"
	PlanTypeGo                          PlanType = "go"
	PlanTypePlus                        PlanType = "plus"
	PlanTypePro                         PlanType = "pro"
	PlanTypeProLite                     PlanType = "prolite"
	PlanTypeTeam                        PlanType = "team"
	PlanTypeSelfServeBusinessUsageBased PlanType = "self_serve_business_usage_based"
	PlanTypeBusiness                    PlanType = "business"
	PlanTypeEnterpriseCBPUsageBased     PlanType = "enterprise_cbp_usage_based"
	PlanTypeEnterprise                  PlanType = "enterprise"
	PlanTypeEdu                         PlanType = "edu"
	PlanTypeUnknown                     PlanType = "unknown"
)

// AuthMode is the authentication mode for OpenAI-backed providers.
type AuthMode string

const (
	AuthModeAPIKey  AuthMode = "apikey"
	AuthModeChatGPT AuthMode = "chatgpt"
)

// AccountInfo describes the currently authenticated account. The fields present
// depend on Type: "chatgpt" accounts carry Email and PlanType, while "apiKey"
// and "amazonBedrock" accounts only carry Type.
type AccountInfo struct {
	Type     string   `json:"type"`
	Email    string   `json:"email,omitempty"`
	PlanType PlanType `json:"planType,omitempty"`
}

// AccountReadResult is the response to account/read.
type AccountReadResult struct {
	Account            *AccountInfo `json:"account,omitempty"`
	RequiresOpenAIAuth bool         `json:"requiresOpenaiAuth"`
}

// accountReadParams is the optional account/read request payload.
type accountReadParams struct {
	RefreshToken bool `json:"refreshToken,omitempty"`
}

// loginAccountParams is the discriminated-union request for account/login/start.
type loginAccountParams struct {
	Type   string `json:"type"`
	APIKey string `json:"apiKey,omitempty"`
}

// loginAccountResponse is the discriminated-union response for account/login/start.
type loginAccountResponse struct {
	Type            string `json:"type"`
	LoginID         string `json:"loginId,omitempty"`
	AuthURL         string `json:"authUrl,omitempty"`
	VerificationURL string `json:"verificationUrl,omitempty"`
	UserCode        string `json:"userCode,omitempty"`
}

// cancelLoginParams is the account/login/cancel request payload.
type cancelLoginParams struct {
	LoginID string `json:"loginId"`
}

// CancelLoginStatus is the outcome of canceling an interactive login attempt.
type CancelLoginStatus string

const (
	CancelLoginStatusCanceled CancelLoginStatus = "canceled"
	CancelLoginStatusNotFound CancelLoginStatus = "notFound"
)

// cancelLoginResponse is the account/login/cancel response payload.
type cancelLoginResponse struct {
	Status CancelLoginStatus `json:"status"`
}

// LoginCompleted is the payload of the account/login/completed notification.
type LoginCompleted struct {
	Success bool   `json:"success"`
	LoginID string `json:"loginId,omitempty"`
	Error   string `json:"error,omitempty"`
}

// AccountRead returns the currently authenticated account, if any.
func (c *Client) AccountRead(ctx context.Context) (AccountReadResult, error) {
	var result AccountReadResult
	if err := c.transport.Call(ctx, protocol.MethodAccountRead, accountReadParams{}, &result); err != nil {
		return AccountReadResult{}, err
	}
	return result, nil
}

// LoginAPIKey authenticates the account with an OpenAI API key. This is a
// synchronous login: it completes as soon as the RPC returns successfully.
func (c *Client) LoginAPIKey(ctx context.Context, apiKey string) error {
	params := loginAccountParams{Type: "apiKey", APIKey: apiKey}
	var resp loginAccountResponse
	return c.transport.Call(ctx, protocol.MethodAccountLoginStart, params, &resp)
}

// LoginChatGPT begins a browser-based ChatGPT OAuth login. It returns a
// LoginHandle whose AuthURL the caller should open in a browser; call
// handle.Wait to block until the login completes.
func (c *Client) LoginChatGPT(ctx context.Context) (*LoginHandle, error) {
	return c.startInteractiveLogin(ctx, loginAccountParams{Type: "chatgpt"})
}

// LoginDeviceCode begins a device-code ChatGPT login. It returns a LoginHandle
// whose VerificationURL and UserCode the caller should present to the user;
// call handle.Wait to block until the login completes.
func (c *Client) LoginDeviceCode(ctx context.Context) (*LoginHandle, error) {
	return c.startInteractiveLogin(ctx, loginAccountParams{Type: "chatgptDeviceCode"})
}

func (c *Client) startInteractiveLogin(ctx context.Context, params loginAccountParams) (*LoginHandle, error) {
	var resp loginAccountResponse
	if err := c.transport.Call(ctx, protocol.MethodAccountLoginStart, params, &resp); err != nil {
		return nil, err
	}
	if resp.LoginID == "" {
		return nil, fmt.Errorf("account/login/start returned no loginId for type %q", resp.Type)
	}
	return &LoginHandle{
		client:          c,
		Type:            resp.Type,
		LoginID:         resp.LoginID,
		AuthURL:         resp.AuthURL,
		VerificationURL: resp.VerificationURL,
		UserCode:        resp.UserCode,
	}, nil
}

// Logout clears the stored credentials for the current account.
func (c *Client) Logout(ctx context.Context) error {
	return c.transport.Call(ctx, protocol.MethodAccountLogout, nil, nil)
}

// LoginHandle represents an in-flight interactive login attempt (ChatGPT OAuth
// or device code). The caller surfaces the URL/code fields to the user, then
// calls Wait to block until the matching account/login/completed notification
// arrives, or Cancel to abandon the attempt.
type LoginHandle struct {
	client *Client

	// Type is the login flavor: "chatgpt" or "chatgptDeviceCode".
	Type string
	// LoginID identifies this attempt; it matches the loginId in the
	// account/login/completed notification.
	LoginID string
	// AuthURL is the browser OAuth URL (chatgpt logins only).
	AuthURL string
	// VerificationURL is the device-code verification URL (device-code logins only).
	VerificationURL string
	// UserCode is the one-time code the user enters (device-code logins only).
	UserCode string
}

// Wait blocks until this login attempt completes (or ctx is done). It returns
// the completion payload; a failed login has Success=false and a non-empty Error.
func (h *LoginHandle) Wait(ctx context.Context) (LoginCompleted, error) {
	if h.client == nil || h.client.events == nil {
		return LoginCompleted{}, fmt.Errorf("login wait requires a notification-capable transport")
	}
	sub := h.client.events.Subscribe()
	defer sub.Close()

	for {
		select {
		case <-ctx.Done():
			return LoginCompleted{}, ctx.Err()
		case event, ok := <-sub.C():
			if !ok {
				return LoginCompleted{}, fmt.Errorf("event stream closed before login %q completed", h.LoginID)
			}
			if event.Method != protocol.MethodAccountLoginCompleted {
				continue
			}
			var payload LoginCompleted
			if err := event.Decode(&payload); err != nil {
				continue
			}
			// A null loginId on the notification matches any pending attempt.
			if payload.LoginID != "" && payload.LoginID != h.LoginID {
				continue
			}
			return payload, nil
		}
	}
}

// Cancel abandons this interactive login attempt.
func (h *LoginHandle) Cancel(ctx context.Context) (CancelLoginStatus, error) {
	var resp cancelLoginResponse
	err := h.client.transport.Call(ctx, protocol.MethodAccountLoginCancel, cancelLoginParams{LoginID: h.LoginID}, &resp)
	if err != nil {
		return "", err
	}
	return resp.Status, nil
}
