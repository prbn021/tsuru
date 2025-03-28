// Copyright 2015 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package permission

import (
	permTypes "github.com/tsuru/tsuru/types/permission"
	check "gopkg.in/check.v1"
)

func (s *S) TestRecorderPermissions(c *check.C) {
	r := &registry{}
	r.addWithCtx("app", []permTypes.ContextType{permTypes.CtxApp, permTypes.CtxTeam, permTypes.CtxPool}).
		add("app.update.env.set", "app.update.env.unset", "app.update.restart", "app.deploy").
		addWithCtx("team", []permTypes.ContextType{permTypes.CtxTeam}).
		addWithCtx("team.create", []permTypes.ContextType{}).
		add("team.update")
	expected := permTypes.PermissionSchemeList{
		{},
		{Name: "app", Contexts: []permTypes.ContextType{permTypes.CtxApp, permTypes.CtxTeam, permTypes.CtxPool}},
		{Name: "update"},
		{Name: "env"},
		{Name: "set"},
		{Name: "unset"},
		{Name: "restart"},
		{Name: "deploy"},
		{Name: "team", Contexts: []permTypes.ContextType{permTypes.CtxTeam}},
		{Name: "create", Contexts: []permTypes.ContextType{}},
		{Name: "update"},
	}
	expected[1].Parent = expected[0]
	expected[2].Parent = expected[1]
	expected[3].Parent = expected[2]
	expected[4].Parent = expected[3]
	expected[5].Parent = expected[3]
	expected[6].Parent = expected[2]
	expected[7].Parent = expected[1]
	expected[8].Parent = expected[0]
	expected[9].Parent = expected[8]
	expected[10].Parent = expected[8]
	perms := r.Permissions()
	c.Assert(perms, check.DeepEquals, expected)
	c.Assert(perms[0].FullName(), check.Equals, "")
	c.Assert(perms[1].FullName(), check.Equals, "app")
	c.Assert(perms[2].FullName(), check.Equals, "app.update")
	c.Assert(perms[3].FullName(), check.Equals, "app.update.env")
	c.Assert(perms[4].FullName(), check.Equals, "app.update.env.set")
	c.Assert(perms[5].FullName(), check.Equals, "app.update.env.unset")
	c.Assert(perms[6].FullName(), check.Equals, "app.update.restart")
	c.Assert(perms[7].FullName(), check.Equals, "app.deploy")
	c.Assert(perms[8].FullName(), check.Equals, "team")
	c.Assert(perms[9].FullName(), check.Equals, "team.create")
	c.Assert(perms[10].FullName(), check.Equals, "team.update")
	c.Assert(perms[1].AllowedContexts(), check.DeepEquals, []permTypes.ContextType{permTypes.CtxGlobal, permTypes.CtxApp, permTypes.CtxTeam, permTypes.CtxPool})
	c.Assert(perms[5].AllowedContexts(), check.DeepEquals, []permTypes.ContextType{permTypes.CtxGlobal, permTypes.CtxApp, permTypes.CtxTeam, permTypes.CtxPool})
	c.Assert(perms[9].AllowedContexts(), check.DeepEquals, []permTypes.ContextType{permTypes.CtxGlobal})
	c.Assert(perms[10].AllowedContexts(), check.DeepEquals, []permTypes.ContextType{permTypes.CtxGlobal, permTypes.CtxTeam})
}

func (s *S) TestRecorderGet(c *check.C) {
	r := (&registry{}).add("app.update.env.set")
	perm := r.get("app.update")
	c.Assert(perm, check.NotNil)
	c.Assert(perm.FullName(), check.Equals, "app.update")
	r = (&registry{}).addWithCtx("app", []permTypes.ContextType{permTypes.CtxApp, permTypes.CtxTeam, permTypes.CtxPool}).
		add("app.update.env.set")
	perm = r.get("app.update")
	c.Assert(perm, check.NotNil)
	c.Assert(perm.FullName(), check.Equals, "app.update")
	c.Assert(perm.AllowedContexts(), check.DeepEquals, []permTypes.ContextType{permTypes.CtxGlobal, permTypes.CtxApp, permTypes.CtxTeam, permTypes.CtxPool})
	perm = r.get("app.update.env.set")
	c.Assert(perm, check.NotNil)
	c.Assert(perm.FullName(), check.Equals, "app.update.env.set")
	c.Assert(perm.AllowedContexts(), check.DeepEquals, []permTypes.ContextType{permTypes.CtxGlobal, permTypes.CtxApp, permTypes.CtxTeam, permTypes.CtxPool})
	perm = r.get("")
	c.Assert(perm, check.NotNil)
	c.Assert(perm.FullName(), check.Equals, "")
	c.Assert(func() {
		r.get("app.update.invalid")
	}, check.PanicMatches, `unregistered permission: app\.update\.invalid`)
}
