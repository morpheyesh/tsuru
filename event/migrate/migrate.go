// Copyright 2016 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package migrate

import (
	"fmt"
	"strings"

	"github.com/tsuru/tsuru/app"
	"github.com/tsuru/tsuru/event"
	"github.com/tsuru/tsuru/permission"
	"github.com/tsuru/tsuru/provision"
	"github.com/tsuru/tsuru/service"
	"gopkg.in/mgo.v2/bson"
)

func MigrateRCEvents() error {
	return event.Migrate(bson.M{"allowed.scheme": bson.M{"$exists": false}}, setAllowed)
}

func setAllowed(evt *event.Event) (err error) {
	defer func() {
		if err != nil {
			fmt.Printf("setting global context to evt %q: %s\n", evt.String(), err)
			err = nil
		}
	}()
	switch evt.Target.Type {
	case event.TargetTypeApp:
		var a *app.App
		a, err = app.GetByName(evt.Target.Value)
		if err != nil {
			evt.Allowed = event.Allowed(permission.PermAppReadEvents)
			if evt.Cancelable {
				evt.Allowed = event.Allowed(permission.PermAppUpdateEvents)
			}
			return err
		}
		ctxs := append(permission.Contexts(permission.CtxTeam, a.Teams),
			permission.Context(permission.CtxApp, a.Name),
			permission.Context(permission.CtxPool, a.Pool),
		)
		evt.Allowed = event.Allowed(permission.PermAppReadEvents, ctxs...)
		if evt.Cancelable {
			evt.Allowed = event.Allowed(permission.PermAppUpdateEvents, ctxs...)
		}
	case event.TargetTypeTeam:
		evt.Allowed = event.Allowed(permission.PermTeamReadEvents, permission.Context(permission.CtxTeam, evt.Target.Value))
	case event.TargetTypeService:
		s := service.Service{Name: evt.Target.Value}
		err = s.Get()
		if err != nil {
			evt.Allowed = event.Allowed(permission.PermServiceReadEvents)
			return err
		}
		evt.Allowed = event.Allowed(permission.PermServiceReadEvents,
			append(permission.Contexts(permission.CtxTeam, s.OwnerTeams),
				permission.Context(permission.CtxService, s.Name),
			)...,
		)
	case event.TargetTypeServiceInstance:
		v := strings.SplitN(evt.Target.Value, "/", 2)
		if len(v) != 2 {
			evt.Allowed = event.Allowed(permission.PermServiceInstanceReadEvents)
			return nil
		}
		var si *service.ServiceInstance
		si, err = service.GetServiceInstance(v[0], v[1])
		if err != nil {
			evt.Allowed = event.Allowed(permission.PermServiceInstanceReadEvents)
			return err
		}
		evt.Allowed = event.Allowed(permission.PermServiceReadEvents,
			append(permission.Contexts(permission.CtxTeam, si.Teams),
				permission.Context(permission.CtxServiceInstance, evt.Target.Value),
			)...,
		)
	case event.TargetTypePool:
		evt.Allowed = event.Allowed(permission.PermPoolReadEvents, permission.Context(permission.CtxPool, evt.Target.Value))
	case event.TargetTypeUser:
		evt.Allowed = event.Allowed(permission.PermUserReadEvents, permission.Context(permission.CtxUser, evt.Target.Value))
	case event.TargetTypeIaas:
		evt.Allowed = event.Allowed(permission.PermMachineReadEvents, permission.Context(permission.CtxIaaS, evt.Target.Value))
	case event.TargetTypeContainer:
		var a provision.App
		a, err = app.Provisioner.GetAppFromUnitID(evt.Target.Value)
		if err != nil {
			evt.Allowed = event.Allowed(permission.PermAppReadEvents)
			return err
		}
		evt.Allowed = event.Allowed(permission.PermAppReadEvents,
			append(permission.Contexts(permission.CtxTeam, a.GetTeamsName()),
				permission.Context(permission.CtxApp, a.GetName()),
				permission.Context(permission.CtxPool, a.GetPool()),
			)...,
		)
	case event.TargetTypeNode:
		nodeProvisioner, ok := app.Provisioner.(provision.NodeProvisioner)
		if !ok {
			evt.Allowed = event.Allowed(permission.PermPoolReadEvents)
			return nil
		}
		var nodes []provision.Node
		nodes, err = nodeProvisioner.ListNodes([]string{evt.Target.Value})
		if err != nil {
			evt.Allowed = event.Allowed(permission.PermPoolReadEvents)
			return err
		}
		var ctxs []permission.PermissionContext
		if len(nodes) > 0 {
			ctxs = append(ctxs, permission.Context(permission.CtxPool, nodes[0].Pool()))
		}
		evt.Allowed = event.Allowed(permission.PermPoolReadEvents, ctxs...)
	case event.TargetTypeRole:
		evt.Allowed = event.Allowed(permission.PermRoleReadEvents)
	default:
		evt.Allowed = event.Allowed(permission.PermDebug)
	}
	return nil
}
