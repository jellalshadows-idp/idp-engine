package floci

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"

	"github.com/jellalshadows-idp/idp-engine/internal/naming"
)

// stringList is an IAM policy element written as one string or a list of them.
type stringList []string

func (l *stringList) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*l = stringList{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return fmt.Errorf("want a string or a list of strings, got %s", b)
	}
	*l = many
	return nil
}

// statement is one IAM policy statement. Decoding refuses any other element
// (NotAction, NotResource, NotPrincipal) and anything after the document, so a
// policy cannot widen itself through an element this comparison does not read.
// A duplicate key is not detected (encoding/json keeps the last one); the
// module's jsonencode never writes one.
type statement struct {
	Sid       string                           `json:"Sid,omitempty"`
	Effect    string                           `json:"Effect"`
	Principal map[string]stringList            `json:"Principal,omitempty"`
	Action    stringList                       `json:"Action"`
	Resource  stringList                       `json:"Resource,omitempty"`
	Condition map[string]map[string]stringList `json:"Condition,omitempty"`
}

// statements is a policy's Statement, written as one statement or a list of them.
type statements []statement

func (s *statements) UnmarshalJSON(b []byte) error {
	if t := bytes.TrimSpace(b); len(t) > 0 && t[0] == '{' {
		var one statement
		if err := strictJSON(t, &one); err != nil {
			return err
		}
		*s = statements{one}
		return nil
	}
	var many []statement
	if err := strictJSON(b, &many); err != nil {
		return err
	}
	*s = many
	return nil
}

// policy is an IAM policy document.
type policy struct {
	Version   string     `json:"Version"`
	Statement statements `json:"Statement"`
}

// strictJSON decodes exactly one JSON value into v, refusing unknown fields and
// anything after the value.
func strictJSON(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	switch err := dec.Decode(new(json.RawMessage)); {
	case errors.Is(err, io.EOF):
		return nil
	case err == nil:
		return errors.New("unexpected data after the document")
	default:
		return fmt.Errorf("unexpected data after the document: %w", err)
	}
}

// decodePolicy reads a policy document as the AWS CLI prints it: an object, or
// the URL-encoded string that the IAM API returns.
func decodePolicy(raw json.RawMessage) (policy, error) {
	doc := []byte(raw)
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		unescaped, err := url.PathUnescape(s)
		if err != nil {
			return policy{}, err
		}
		doc = []byte(unescaped)
	}
	var p policy
	if err := strictJSON(doc, &p); err != nil {
		return policy{}, fmt.Errorf("policy document %s: %w", doc, err)
	}
	return p, nil
}

// trustPolicy is the only trust policy a Component's CI role may have (spec
// §4.4): GitHub Actions tokens of one repository and environment, for STS.
func trustPolicy(providerARN, githubRepository, env string) policy {
	return policy{Version: "2012-10-17", Statement: statements{{
		Effect:    "Allow",
		Principal: map[string]stringList{"Federated": {providerARN}},
		Action:    stringList{"sts:AssumeRoleWithWebIdentity"},
		Condition: map[string]map[string]stringList{"StringEquals": {
			naming.OIDCProviderHost + ":aud": {"sts.amazonaws.com"},
			naming.OIDCProviderHost + ":sub": {"repo:" + githubRepository + ":environment:" + env},
		}},
	}}}
}

// pushPolicy is the only policy a Component's CI role may have (spec §4.4):
// log in to the registry, an account-level action, and push to its repository.
func pushPolicy(repositoryARN string) policy {
	return policy{Version: "2012-10-17", Statement: statements{
		{Sid: "ECRLogin", Effect: "Allow", Action: stringList{"ecr:GetAuthorizationToken"}, Resource: stringList{"*"}},
		{Sid: "ECRPush", Effect: "Allow", Action: stringList{
			"ecr:BatchCheckLayerAvailability",
			"ecr:CompleteLayerUpload",
			"ecr:InitiateLayerUpload",
			"ecr:PutImage",
			"ecr:UploadLayerPart",
		}, Resource: stringList{repositoryARN}},
	}}
}
