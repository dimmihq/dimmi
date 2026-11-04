/*
|--------------------------------------------------------------------------
| Role Controller
|--------------------------------------------------------------------------
|
| GET /roles lists the built-in roles, then the workspace's custom roles
| by name; GET /roles/{role} shows one custom role. POST /roles creates
| one, PATCH /roles/{role} edits it and DELETE /roles/{role} deletes it.
| GET /permissions lists the catalog a role is shaped from, in the
| reader's language.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/roles"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RolesOutput is every role of the workspace.
*/
type RolesOutput struct {
	Body fhttp.Collection[resources.Role]
}

/*
RoleOutput is one custom role.
*/
type RoleOutput struct {
	Body fhttp.Resource[resources.Role]
}

/*
PermissionsOutput is the permission catalog.
*/
type PermissionsOutput struct {
	Body fhttp.Collection[resources.Permission]
}

/*
RoleController serves the roles of the workspace in play.
*/
type RoleController struct {
	ListRoles       *roles.ListRoles
	ShowRole        *roles.ShowRole
	CreateRole      *roles.CreateRole
	UpdateRole      *roles.UpdateRole
	DeleteRole      *roles.DeleteRole
	ListPermissions *roles.ListPermissions
	IDs             *ids.Registry
}

/*
Index lists the built-in roles and the workspace's custom roles.
*/
func (c *RoleController) Index(ctx context.Context, _ *requests.NoInput) (*RolesOutput, error) {
	list, err := c.ListRoles.Handle(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := []resources.Role{
		resources.NewBuiltInRole(enums.RoleOwner, "Owner", appauth.RolePermissions(enums.RoleOwner)),
		resources.NewBuiltInRole(enums.RoleAdmin, "Admin", appauth.RolePermissions(enums.RoleAdmin)),
		resources.NewBuiltInRole(enums.RoleMember, "Member", appauth.RolePermissions(enums.RoleMember)),
	}

	for _, role := range list.Custom {
		var count *int64
		if list.Counted {
			count = &role.MemberCount
		}

		items = append(items, resources.NewRole(role, count))
	}

	page := pagination.Page[resources.Role]{Items: items, Limit: len(items)}

	return &RolesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Show answers with one custom role.
*/
func (c *RoleController) Show(ctx context.Context, in *requests.ShowRoleRequest) (*RoleOutput, error) {
	id, err := roleID(c.IDs, in.Role)
	if err != nil {
		return nil, err
	}

	m := membership(ctx)

	role, err := c.ShowRole.Handle(ctx, roles.ShowRoleInput{Membership: m, RoleID: id})
	if err != nil {
		return nil, err
	}

	var count *int64
	if m.Can("roles:manage") {
		count = &role.MemberCount
	}

	return &RoleOutput{Body: fhttp.NewResource(resources.NewRole(role, count))}, nil
}

/*
Store creates a custom role and answers with it.
*/
func (c *RoleController) Store(ctx context.Context, in *requests.StoreRoleRequest) (*RoleOutput, error) {
	role, err := c.CreateRole.Handle(ctx, roles.CreateRoleInput{
		Membership: membership(ctx), Name: in.Body.Name, Description: in.Body.Description,
		BaseRole: enums.Role(in.Body.BaseRole), Permissions: in.Body.Permissions,
	})
	if err != nil {
		return nil, err
	}

	return &RoleOutput{Body: fhttp.NewResource(resources.NewRole(role, &role.MemberCount))}, nil
}

/*
Update edits a custom role and answers with it as it now is.
*/
func (c *RoleController) Update(ctx context.Context, in *requests.UpdateRoleRequest) (*RoleOutput, error) {
	id, err := roleID(c.IDs, in.Role)
	if err != nil {
		return nil, err
	}

	role, err := c.UpdateRole.Handle(ctx, roles.UpdateRoleInput{
		Membership: membership(ctx), RoleID: id, Name: in.Body.Name, Description: in.Body.Description,
		Permissions: in.Body.Permissions,
	})
	if err != nil {
		return nil, err
	}

	return &RoleOutput{Body: fhttp.NewResource(resources.NewRole(role, &role.MemberCount))}, nil
}

/*
Destroy deletes a custom role.
*/
func (c *RoleController) Destroy(ctx context.Context, in *requests.ShowRoleRequest) (*EmptyOutput, error) {
	id, err := roleID(c.IDs, in.Role)
	if err != nil {
		return nil, err
	}

	if err := c.DeleteRole.Handle(ctx, roles.DeleteRoleInput{Membership: membership(ctx), RoleID: id}); err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
Permissions lists the permission catalog with what each permission is
called and does.
*/
func (c *RoleController) Permissions(ctx context.Context, _ *requests.NoInput) (*PermissionsOutput, error) {
	list, err := c.ListPermissions.Handle(membership(ctx))
	if err != nil {
		return nil, err
	}

	items := make([]resources.Permission, 0, len(list))
	for _, p := range list {
		bases := make([]string, 0, len(p.BaseRoles))
		for _, role := range p.BaseRoles {
			bases = append(bases, string(role))
		}

		items = append(items, resources.Permission{
			Object: "permission", Key: p.Key, Group: p.Group, Assignable: p.Assignable, BaseRoles: bases,
			Label:       i18n.T(ctx, "permissions."+p.Key+".label", nil),
			Description: i18n.T(ctx, "permissions."+p.Key+".description", nil),
		})
	}

	page := pagination.Page[resources.Permission]{Items: items, Limit: len(items)}

	return &PermissionsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
roleID returns the custom role a path or body names, refusing ids that
name no role as not found.
*/
func roleID(registry *ids.Registry, named string) (ids.UUID, error) {
	id, err := registry.Decode(string(models.PrefixRole), named)
	if err != nil {
		return ids.Nil(), exceptions.ErrNotFound
	}

	return id, nil
}
