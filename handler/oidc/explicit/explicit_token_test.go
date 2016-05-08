package explicit

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/go-errors/errors"
	"github.com/golang/mock/gomock"
	"github.com/ory-am/fosite"
	"github.com/ory-am/fosite/handler/oidc"
	"github.com/ory-am/fosite/handler/oidc/strategy"
	"github.com/ory-am/fosite/internal"
	"github.com/ory-am/fosite/token/jwt"
	"github.com/stretchr/testify/assert"
)

func TestHandleTokenEndpointRequest(t *testing.T) {
	h := &OpenIDConnectExplicitHandler{}
	areq := fosite.NewAccessRequest(nil)
	areq.Client = &fosite.DefaultClient{
		ResponseTypes: fosite.Arguments{"id_token"},
	}
	assert.True(t, errors.Is(h.HandleTokenEndpointRequest(nil, nil, areq), fosite.ErrUnknownRequest))
}

func TestPopulateTokenEndpointResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := internal.NewMockOpenIDConnectRequestStorage(ctrl)
	defer ctrl.Finish()

	session := &strategy.DefaultSession{
		Claims: &jwt.IDTokenClaims{
			Subject: "peter",
		},
		Headers: &jwt.Headers{},
	}
	aresp := fosite.NewAccessResponse()
	areq := fosite.NewAccessRequest(session)
	httpreq := &http.Request{PostForm: url.Values{}}

	h := &OpenIDConnectExplicitHandler{
		OpenIDConnectRequestStorage: store,
		IDTokenHandleHelper: &oidc.IDTokenHandleHelper{
			IDTokenStrategy: j,
		},
	}
	for k, c := range []struct {
		description string
		setup       func()
		expectErr   error
	}{
		{
			description: "should fail because invalid response type",
			setup:       func() {},
			expectErr:   fosite.ErrUnknownRequest,
		},
		{
			description: "should fail because lookup returns not found",
			setup: func() {
				areq.GrantTypes = fosite.Arguments{"authorization_code"}
				areq.Client = &fosite.DefaultClient{
					GrantTypes:    fosite.Arguments{"code"},
					ResponseTypes: fosite.Arguments{"id_token"},
				}
				areq.Form.Set("code", "foobar")
				store.EXPECT().GetOpenIDConnectSession(nil, "foobar", areq).Return(nil, oidc.ErrNoSessionFound)
			},
			expectErr: fosite.ErrUnknownRequest,
		},
		{
			description: "should fail because lookup fails",
			setup: func() {
				areq.GrantTypes = fosite.Arguments{"authorization_code"}
				store.EXPECT().GetOpenIDConnectSession(nil, "foobar", areq).Return(nil, errors.New(""))
			},
			expectErr: fosite.ErrServerError,
		},
		{
			description: "should fail because missing scope in original request",
			setup: func() {
				areq.GrantTypes = fosite.Arguments{"authorization_code"}
				store.EXPECT().GetOpenIDConnectSession(nil, "foobar", areq).Return(fosite.NewAuthorizeRequest(), nil)
			},
			expectErr: fosite.ErrUnknownRequest,
		},
		{
			description: "should pass",
			setup: func() {
				r := fosite.NewAuthorizeRequest()
				r.Session = areq.Session
				r.Scopes = fosite.Arguments{"openid"}
				r.Form.Set("nonce", "1111111111111111")
				store.EXPECT().GetOpenIDConnectSession(nil, gomock.Any(), areq).AnyTimes().Return(r, nil)
			},
		},
	} {
		c.setup()
		err := h.PopulateTokenEndpointResponse(nil, httpreq, areq, aresp)
		assert.True(t, errors.Is(c.expectErr, err), "(%d) %s\n%s\n%s", k, c.description, err, c.expectErr)
		t.Logf("Passed test case %d", k)
	}
}
