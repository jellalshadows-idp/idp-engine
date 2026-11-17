package floci

import (
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/jellalshadows-idp/idp-engine/internal/naming"
)

// The documents that modules/aws/component plans for api in dev, account
// 000000000001, as OpenTofu's jsonencode writes them (keys sorted). Its tftest
// asserts the same documents. Nothing here reads the module: these constants
// pin the Go builders offline, and TestFloci ties the builders to the module by
// comparing them with the policies floci stores.
const (
	moduleTrustPolicy = `{"Statement":[{"Action":"sts:AssumeRoleWithWebIdentity","Condition":{"StringEquals":{"token.actions.githubusercontent.com:aud":"sts.amazonaws.com","token.actions.githubusercontent.com:sub":"repo:acme/api:environment:dev"}},"Effect":"Allow","Principal":{"Federated":"arn:aws:iam::000000000001:oidc-provider/token.actions.githubusercontent.com"}}],"Version":"2012-10-17"}`
	modulePushPolicy  = `{"Statement":[{"Action":["ecr:GetAuthorizationToken"],"Effect":"Allow","Resource":"*","Sid":"ECRLogin"},{"Action":["ecr:BatchCheckLayerAvailability","ecr:CompleteLayerUpload","ecr:InitiateLayerUpload","ecr:PutImage","ecr:UploadLayerPart"],"Effect":"Allow","Resource":"arn:aws:ecr:eu-west-1:000000000001:repository/api","Sid":"ECRPush"}],"Version":"2012-10-17"}`
)

func TestExpectedPoliciesAreTheModulesOwn(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want policy
	}{
		{"trust policy", moduleTrustPolicy, trustPolicy(naming.OIDCProviderARN("000000000001"), "acme/api", "dev")},
		{"push policy", modulePushPolicy, pushPolicy("arn:aws:ecr:eu-west-1:000000000001:repository/api")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodePolicy(json.RawMessage(tt.doc))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decoded %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestDecodePolicy(t *testing.T) {
	quoted, err := json.Marshal(url.PathEscape(moduleTrustPolicy))
	if err != nil {
		t.Fatal(err)
	}
	trust := trustPolicy(naming.OIDCProviderARN("000000000001"), "acme/api", "dev")
	tests := []struct {
		name string
		raw  string
		want policy
		bad  string // a substring of the error, when decoding must fail
	}{
		{name: "an object, as the AWS CLI prints it", raw: moduleTrustPolicy, want: trust},
		{name: "a URL-encoded string, as the IAM API returns it", raw: string(quoted), want: trust},
		{name: "a single statement and single values", raw: `{"Version":"2012-10-17","Statement":{"Effect":"Allow","Action":"ecr:PutImage","Resource":"*"}}`,
			want: policy{Version: "2012-10-17", Statement: statements{{Effect: "Allow", Action: stringList{"ecr:PutImage"}, Resource: stringList{"*"}}}}},
		{name: "NotResource, which widens a policy", raw: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"ecr:PutImage","NotResource":"x"}]}`, bad: "NotResource"},
		{name: "an unknown top-level element", raw: `{"Version":"2012-10-17","Id":"x","Statement":[]}`, bad: "Id"},
		{name: "a number for an action", raw: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":7}]}`, bad: "want a string or a list of strings"},
		{name: "a second document after the policy", raw: moduleTrustPolicy + `{"Statement":[{"Effect":"Allow","Action":"*","Resource":"*"}]}`, bad: "after the document"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodePolicy(json.RawMessage(tt.raw))
			if tt.bad != "" {
				if err == nil || !strings.Contains(err.Error(), tt.bad) {
					t.Fatalf("decodePolicy = %+v, %v; want an error about %s", got, err, tt.bad)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decoded %+v, want %+v", got, tt.want)
			}
		})
	}
}
