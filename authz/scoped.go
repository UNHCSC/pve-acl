package authz

import (
	"fmt"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	"github.com/casbin/govaluate"
)

const ScopedRBACModel = `
[request_definition]
r = sub, dom, act

[policy_definition]
p = sub, dom, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, p.dom) && scopeMatch(p.dom, r.dom) && (p.act == "*" || p.act == r.act)
`

type ScopeMatcher func(policyScope, requestedScope string) bool

func NewScopedEnforcer(scopeMatcher ScopeMatcher) (*casbin.Enforcer, error) {
	if scopeMatcher == nil {
		scopeMatcher = func(policyScope, requestedScope string) bool {
			return policyScope == requestedScope
		}
	}

	model, err := casbinmodel.NewModelFromString(ScopedRBACModel)
	if err != nil {
		return nil, err
	}

	enforcer, err := casbin.NewEnforcer(model)
	if err != nil {
		return nil, err
	}
	enforcer.AddFunction("scopeMatch", scopeMatchFunc(scopeMatcher))
	return enforcer, nil
}

func scopeMatchFunc(scopeMatcher ScopeMatcher) govaluate.ExpressionFunction {
	return func(args ...interface{}) (interface{}, error) {
		if len(args) != 2 {
			return false, fmt.Errorf("scopeMatch expects 2 arguments")
		}
		policyScope, ok := args[0].(string)
		if !ok {
			return false, fmt.Errorf("policy scope must be a string")
		}
		requestedScope, ok := args[1].(string)
		if !ok {
			return false, fmt.Errorf("requested scope must be a string")
		}
		return scopeMatcher(policyScope, requestedScope), nil
	}
}
