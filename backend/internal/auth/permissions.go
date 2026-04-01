package auth

// Role constants used across backend and middleware.
const (
	RoleAdministrator         = "administrator"
	RoleOperationsManager     = "operations_manager"
	RoleProcurementSpecialist = "procurement_specialist"
	RoleCoach                 = "coach"
	RoleMember                = "member"
)

// Permission is a typed capability string.
type Permission string

const (
	// System
	PermSystemConfig Permission = "system:config"
	PermUserManage   Permission = "user:manage"

	// Catalog
	PermCatalogRead  Permission = "catalog:read"
	PermCatalogWrite Permission = "catalog:write"

	// Inventory
	PermInventoryRead   Permission = "inventory:read"
	PermInventoryAdjust Permission = "inventory:adjust"

	// Suppliers & purchase orders
	PermSupplierRead  Permission = "supplier:read"
	PermSupplierWrite Permission = "supplier:write"
	PermPORead        Permission = "po:read"
	PermPOWrite       Permission = "po:write"
	PermPOReceive     Permission = "po:receive"

	// Reporting & exports
	PermReportDashboard Permission = "report:dashboard"
	PermReportFull      Permission = "report:full"
	PermReportCoach     Permission = "report:coach"
	PermExportGenerate  Permission = "export:generate"

	// Classes
	PermClassRead      Permission = "class:read"
	PermClassWrite     Permission = "class:write"
	PermClassReadiness Permission = "class:readiness"

	// Group-buys
	PermGroupBuyRead   Permission = "groupbuy:read"
	PermGroupBuyCreate Permission = "groupbuy:create"
	PermGroupBuyJoin   Permission = "groupbuy:join"
	PermGroupBuyManage Permission = "groupbuy:manage"

	// Orders
	PermOrderRead     Permission = "order:read"
	PermOrderOwnRead  Permission = "order:own_read"
	PermOrderAdjust   Permission = "order:adjust"
	PermOrderNoteAdd  Permission = "order:note_add"
	PermOrderTimeline Permission = "order:timeline"

	// Members
	PermMemberRead   Permission = "member:read"
	PermMemberBrowse Permission = "member:browse"

	// Audit
	PermAuditRead Permission = "audit:read"
)

// rolePermissions is the authoritative permission matrix.
// Coach and Member never receive supplier, PO, or system config permissions.
var rolePermissions = map[string]map[Permission]bool{
	RoleAdministrator: buildSet(
		PermSystemConfig, PermUserManage,
		PermCatalogRead, PermCatalogWrite,
		PermInventoryRead, PermInventoryAdjust,
		PermSupplierRead, PermSupplierWrite,
		PermPORead, PermPOWrite, PermPOReceive,
		PermReportDashboard, PermReportFull,
		PermExportGenerate,
		PermClassRead, PermClassWrite, PermClassReadiness,
		PermGroupBuyRead, PermGroupBuyCreate, PermGroupBuyManage,
		PermOrderRead, PermOrderAdjust, PermOrderNoteAdd, PermOrderTimeline,
		PermMemberRead,
		PermAuditRead,
	),
	RoleOperationsManager: buildSet(
		PermCatalogRead, PermCatalogWrite,
		PermInventoryRead, PermInventoryAdjust,
		PermSupplierRead,
		PermPORead,
		PermReportDashboard, PermReportFull,
		PermExportGenerate,
		PermClassRead,
		PermGroupBuyRead, PermGroupBuyManage,
		PermOrderRead, PermOrderAdjust, PermOrderNoteAdd, PermOrderTimeline,
		PermMemberRead,
	),
	RoleProcurementSpecialist: buildSet(
		PermCatalogRead,
		PermInventoryRead, PermInventoryAdjust,
		PermSupplierRead, PermSupplierWrite,
		PermPORead, PermPOWrite, PermPOReceive,
		PermReportDashboard,
		PermExportGenerate,
	),
	RoleCoach: buildSet(
		PermCatalogRead,
		PermClassRead, PermClassWrite, PermClassReadiness,
		PermReportDashboard, PermReportCoach,
	),
	RoleMember: buildSet(
		PermCatalogRead,
		PermGroupBuyRead, PermGroupBuyCreate, PermGroupBuyJoin,
		PermOrderOwnRead,
		PermMemberBrowse,
	),
}

// HasPermission reports whether role holds permission.
func HasPermission(role string, perm Permission) bool {
	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}
	return perms[perm]
}

// GetPermissions returns all permissions for a role.
func GetPermissions(role string) []Permission {
	perms := rolePermissions[role]
	result := make([]Permission, 0, len(perms))
	for p := range perms {
		result = append(result, p)
	}
	return result
}

func buildSet(perms ...Permission) map[Permission]bool {
	m := make(map[Permission]bool, len(perms))
	for _, p := range perms {
		m[p] = true
	}
	return m
}
