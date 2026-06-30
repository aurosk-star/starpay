package rbac

import (
	_ "embed"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/util"
)

//go:embed model.conf
var modelText string

func NewEnforcer() (*casbin.Enforcer, error) {
	m, err := model.NewModelFromString(modelText)
	if err != nil {
		return nil, err
	}
	enforcer, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, err
	}
	enforcer.AddFunction("keyMatch2", util.KeyMatch2Func)
	if err := LoadDefaultPolicies(enforcer); err != nil {
		return nil, err
	}
	return enforcer, nil
}
