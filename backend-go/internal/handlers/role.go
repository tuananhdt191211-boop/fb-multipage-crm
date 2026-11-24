package handlers

import (
	"net/http"
	"strconv"

	"fb-multipage-crm/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RoleHandler struct {
	DB *gorm.DB
}

func NewRoleHandler(db *gorm.DB) *RoleHandler {
	return &RoleHandler{DB: db}
}

// GET /api/v1/roles/users
// - Admin: xem toàn bộ danh sách user
// - Manager: xem danh sách user có system_role = 'staff'
func (h *RoleHandler) ListUsers(c *gin.Context) {
	actorID := c.GetInt64("user_id")
	var actor models.User
	if err := h.DB.First(&actor, actorID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "không tìm thấy người dùng"})
		return
	}

	var users []models.User
	switch actor.SystemRole {
	case "admin":
		h.DB.Order("id ASC").Find(&users)
	case "manager":
		h.DB.Where("system_role = ?", "staff").Order("id ASC").Find(&users)
	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "nhân viên không có quyền xem danh sách phân quyền"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}

// PATCH /api/v1/roles/users/:id/role — CHỈ ADMIN mới được phân cấp bậc cho 'manager' và 'staff'
func (h *RoleHandler) AssignRole(c *gin.Context) {
	actorID := c.GetInt64("user_id")
	var actor models.User
	if err := h.DB.First(&actor, actorID).Error; err != nil || actor.SystemRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "chỉ Admin mới có quyền phân cấp bậc cho Quản lý và Nhân viên"})
		return
	}

	targetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || targetID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID người dùng không hợp lệ"})
		return
	}

	var req struct {
		SystemRole string `json:"system_role" binding:"required"` // 'manager' | 'staff'
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dữ liệu role không hợp lệ"})
		return
	}

	// Chỉ cho phép gán cấp bậc 'manager' hoặc 'staff' qua API này (không tự hạ/nâng 'admin' trái phép)
	if req.SystemRole != "manager" && req.SystemRole != "staff" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Admin chỉ có thể phân cấp bậc thành 'manager' hoặc 'staff'"})
		return
	}

	if targetID == actor.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "không thể tự thay đổi role của chính mình"})
		return
	}

	var target models.User
	if err := h.DB.First(&target, targetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "không tìm thấy người dùng mục tiêu"})
		return
	}

	if target.SystemRole == "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "không thể đổi cấp bậc của tài khoản Admin"})
		return
	}

	updates := map[string]any{
		"system_role": req.SystemRole,
	}
	// Nếu chuyển lên manager thì xóa manager_id phụ thuộc cũ
	if req.SystemRole == "manager" {
		updates["manager_id"] = nil
	}

	if err := h.DB.Model(&target).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "không thể cập nhật cấp bậc"})
		return
	}

	h.DB.First(&target, targetID)
	c.JSON(http.StatusOK, gin.H{"message": "cập nhật cấp bậc thành công", "user": target})
}

// PATCH /api/v1/roles/users/:id/permissions
// - Admin: phân quyền cho 'staff' hoặc 'manager'
// - Manager: CHỈ được phân quyền cho tài khoản đang có role 'staff'
func (h *RoleHandler) AssignStaffPermissions(c *gin.Context) {
	actorID := c.GetInt64("user_id")
	var actor models.User
	if err := h.DB.First(&actor, actorID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "không tìm thấy người dùng"})
		return
	}

	if actor.SystemRole != "admin" && actor.SystemRole != "manager" {
		c.JSON(http.StatusForbidden, gin.H{"error": "nhân viên không có quyền phân quyền"})
		return
	}

	targetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || targetID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID người dùng không hợp lệ"})
		return
	}

	var target models.User
	if err := h.DB.First(&target, targetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "không tìm thấy người dùng mục tiêu"})
		return
	}

	// Ràng buộc cốt lõi: Quản lý chỉ được phân quyền cho Nhân viên ('staff')
	if actor.SystemRole == "manager" && target.SystemRole != "staff" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Quản lý chỉ có quyền phân quyền cho tài khoản Nhân viên"})
		return
	}

	var req struct {
		Permissions []string `json:"permissions"`
		IsActive    *bool    `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dữ liệu phân quyền không hợp lệ"})
		return
	}

	updates := map[string]any{
		"permissions": req.Permissions,
	}
	if actor.SystemRole == "manager" {
		updates["manager_id"] = actor.ID
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if err := h.DB.Model(&target).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "không thể lưu phân quyền"})
		return
	}

	h.DB.First(&target, targetID)
	c.JSON(http.StatusOK, gin.H{"message": "cập nhật phân quyền thành công", "user": target})
}
