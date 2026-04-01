package exports

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"fitcommerce/backend/internal/auth"
	"fitcommerce/backend/internal/http/response"
	"fitcommerce/backend/internal/middleware"
)

type Handler struct {
	db        *pgxpool.Pool
	rdb       *redis.Client
	exportDir string
}

func NewHandler(db *pgxpool.Pool, rdb *redis.Client, exportDir string) *Handler {
	return &Handler{db: db, rdb: rdb, exportDir: exportDir}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/exports")
	g.Use(middleware.RequireRoles(
		auth.RoleAdministrator, auth.RoleOperationsManager, auth.RoleProcurementSpecialist,
	))
	g.POST("", h.create)
	g.GET("", h.list)
	g.GET("/:id", h.get)
	g.GET("/:id/download", h.download)
}

// ── Types ─────────────────────────────────────────────────────────────────────

type createExportRequest struct {
	ReportType string            `json:"report_type" binding:"required"`
	Format     string            `json:"format" binding:"required"`
	Filters    map[string]string `json:"filters"`
}

type exportResponse struct {
	ID         string  `json:"id"`
	ReportType string  `json:"report_type"`
	Format     string  `json:"format"`
	Status     string  `json:"status"`
	FilePath   *string `json:"file_path"`
	ErrorMsg   *string `json:"error_msg"`
	CreatedBy  *string `json:"created_by"`
	CreatedAt  string  `json:"created_at"`
}

var validReportTypes = map[string]bool{
	"dashboard": true, "member-growth": true, "churn": true,
	"inventory": true, "group-buys": true, "orders": true,
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *Handler) create(c *gin.Context) {
	var req createExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.Format != "csv" && req.Format != "pdf" {
		response.BadRequest(c, "VALIDATION_ERROR", "format must be csv or pdf")
		return
	}
	if !validReportTypes[req.ReportType] {
		response.BadRequest(c, "VALIDATION_ERROR", "invalid report_type")
		return
	}

	ctx := c.Request.Context()
	userID := c.GetString(middleware.KeyUserID)

	var id string
	var createdAt time.Time
	err := h.db.QueryRow(ctx, `
		INSERT INTO export_jobs (report_type, format, filters, created_by)
		VALUES ($1, $2, $3, $4) RETURNING id, created_at
	`, req.ReportType, req.Format, req.Filters, userID).Scan(&id, &createdAt)
	if err != nil {
		response.InternalError(c)
		return
	}

	// Opportunistically process from API path; worker may also compete to claim.
	go h.processQueuedExport(id)

	response.Created(c, exportResponse{
		ID: id, ReportType: req.ReportType, Format: req.Format,
		Status: "queued", CreatedBy: &userID,
		CreatedAt: createdAt.UTC().Format(time.RFC3339),
	})
}

func (h *Handler) list(c *gin.Context) {
	ctx := c.Request.Context()
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "25"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}

	var total int
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM export_jobs`).Scan(&total)

	rows, err := h.db.Query(ctx, `
		SELECT id, report_type, format, status, file_path, error_msg, created_by, created_at
		FROM export_jobs ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`, perPage, (page-1)*perPage)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer rows.Close()

	exports := []exportResponse{}
	for rows.Next() {
		var e exportResponse
		var createdAt time.Time
		if err := rows.Scan(&e.ID, &e.ReportType, &e.Format, &e.Status, &e.FilePath, &e.ErrorMsg,
			&e.CreatedBy, &createdAt); err != nil {
			response.InternalError(c)
			return
		}
		e.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		exports = append(exports, e)
	}

	response.OKPaginated(c, exports, response.Meta{Page: page, PerPage: perPage, Total: total})
}

func (h *Handler) get(c *gin.Context) {
	id := c.Param("id")
	var e exportResponse
	var createdAt time.Time
	err := h.db.QueryRow(c.Request.Context(), `
		SELECT id, report_type, format, status, file_path, error_msg, created_by, created_at
		FROM export_jobs WHERE id = $1
	`, id).Scan(&e.ID, &e.ReportType, &e.Format, &e.Status, &e.FilePath, &e.ErrorMsg,
		&e.CreatedBy, &createdAt)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "export job")
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}
	e.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	response.OK(c, e)
}

func (h *Handler) download(c *gin.Context) {
	id := c.Param("id")
	var filePath *string
	var status, format string
	err := h.db.QueryRow(c.Request.Context(), `SELECT status, file_path, format FROM export_jobs WHERE id = $1`, id).
		Scan(&status, &filePath, &format)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "export job")
		return
	}
	if status != "completed" || filePath == nil {
		response.BadRequest(c, "NOT_READY", "export is not completed yet")
		return
	}

	contentType := "text/csv"
	if format == "pdf" {
		contentType = "application/pdf"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(*filePath)))
	c.File(*filePath)
}

// ── Export processing ─────────────────────────────────────────────────────────

func (h *Handler) processQueuedExport(jobID string) {
	ctx := context.Background()
	var claimedID string
	err := h.db.QueryRow(ctx, `
		UPDATE export_jobs
		SET status = 'processing', updated_at = NOW()
		WHERE id = $1 AND status = 'queued'
		RETURNING id
	`, jobID).Scan(&claimedID)
	if err != nil {
		return
	}
	h.processClaimedExport(ctx, claimedID)
}

func (h *Handler) processClaimedExport(ctx context.Context, jobID string) {
	var reportType, format string
	err := h.db.QueryRow(ctx, `SELECT report_type, format FROM export_jobs WHERE id = $1`, jobID).
		Scan(&reportType, &format)
	if err != nil {
		h.failExport(ctx, jobID, "failed to read job")
		return
	}

	// Generate filename
	ts := time.Now().UTC().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s.%s", reportType, ts, format)
	filePath := filepath.Join(h.exportDir, filename)

	// Ensure directory exists
	_ = os.MkdirAll(h.exportDir, 0755)

	if format == "csv" {
		err = h.generateCSV(ctx, reportType, filePath)
	} else {
		err = h.generatePDF(ctx, reportType, filePath)
	}

	if err != nil {
		h.failExport(ctx, jobID, err.Error())
		return
	}

	_, _ = h.db.Exec(ctx, `UPDATE export_jobs SET status = 'completed', file_path = $1, updated_at = NOW() WHERE id = $2`,
		filePath, jobID)
}

func (h *Handler) failExport(ctx context.Context, jobID, msg string) {
	_, _ = h.db.Exec(ctx, `UPDATE export_jobs SET status = 'failed', error_msg = $1, updated_at = NOW() WHERE id = $2`,
		msg, jobID)
}

func (h *Handler) generateCSV(ctx context.Context, reportType, filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	switch reportType {
	case "inventory":
		w.Write([]string{"Item ID", "Item Name", "On Hand", "Reserved", "Available"})
		rows, err := h.db.Query(ctx, `
			SELECT s.item_id, i.name, s.on_hand, s.reserved, (s.on_hand - s.reserved - s.allocated)
			FROM inventory_stock s JOIN items i ON i.id = s.item_id ORDER BY i.name
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var itemID, name string
			var onHand, reserved, available int
			rows.Scan(&itemID, &name, &onHand, &reserved, &available)
			w.Write([]string{itemID, name, itoa(onHand), itoa(reserved), itoa(available)})
		}

	case "orders":
		w.Write([]string{"Order ID", "Member ID", "Status", "Total", "Created At"})
		rows, err := h.db.Query(ctx, `SELECT id, member_id, status, total_amount, created_at FROM orders ORDER BY created_at DESC`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id, memberID, status string
			var total float64
			var createdAt time.Time
			rows.Scan(&id, &memberID, &status, &total, &createdAt)
			w.Write([]string{id, memberID, status, fmt.Sprintf("%.2f", total), createdAt.UTC().Format(time.RFC3339)})
		}

	case "member-growth":
		w.Write([]string{"Member ID", "User ID", "Status", "Membership Start", "Created At"})
		rows, err := h.db.Query(ctx, `SELECT id, user_id, status, COALESCE(membership_start::text, ''), created_at FROM members ORDER BY created_at`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id, uid, status, start string
			var createdAt time.Time
			rows.Scan(&id, &uid, &status, &start, &createdAt)
			w.Write([]string{id, uid, status, start, createdAt.UTC().Format(time.RFC3339)})
		}

	case "group-buys":
		w.Write([]string{"ID", "Title", "Status", "Min Qty", "Current Qty", "Cutoff"})
		rows, err := h.db.Query(ctx, `SELECT id, title, status, min_quantity, current_quantity, cutoff_at FROM group_buys ORDER BY cutoff_at DESC`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id, title, status string
			var minQ, curQ int
			var cutoff time.Time
			rows.Scan(&id, &title, &status, &minQ, &curQ, &cutoff)
			w.Write([]string{id, title, status, itoa(minQ), itoa(curQ), cutoff.UTC().Format(time.RFC3339)})
		}

	default:
		w.Write([]string{"Report", "Generated At"})
		w.Write([]string{reportType, time.Now().UTC().Format(time.RFC3339)})
	}

	return nil
}

func (h *Handler) generatePDF(ctx context.Context, reportType, filePath string) error {
	// Minimal spec-compliant PDF with correct xref byte offsets.
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	ts := time.Now().UTC().Format("2006-01-02 15:04:05 UTC")
	title := fmt.Sprintf("FitCommerce Report: %s", reportType)
	content := fmt.Sprintf("Generated at: %s\n\nReport type: %s\n\n", ts, reportType)

	switch reportType {
	case "inventory":
		var totalItems, totalOnHand int
		h.db.QueryRow(ctx, `SELECT COUNT(*) FROM items`).Scan(&totalItems)
		h.db.QueryRow(ctx, `SELECT COALESCE(SUM(on_hand), 0) FROM inventory_stock`).Scan(&totalOnHand)
		content += fmt.Sprintf("Total Items: %d\nTotal On Hand: %d\n", totalItems, totalOnHand)
	case "member-growth":
		var total int
		h.db.QueryRow(ctx, `SELECT COUNT(*) FROM members`).Scan(&total)
		content += fmt.Sprintf("Total Members: %d\n", total)
	case "group-buys":
		var total, active, succeeded int
		h.db.QueryRow(ctx, `SELECT COUNT(*) FROM group_buys`).Scan(&total)
		h.db.QueryRow(ctx, `SELECT COUNT(*) FROM group_buys WHERE status = 'active'`).Scan(&active)
		h.db.QueryRow(ctx, `SELECT COUNT(*) FROM group_buys WHERE status = 'succeeded'`).Scan(&succeeded)
		content += fmt.Sprintf("Total: %d\nActive: %d\nSucceeded: %d\n", total, active, succeeded)
	}

	streamBody := fmt.Sprintf("BT /F1 16 Tf 50 750 Td (%s) Tj ET\nBT /F1 12 Tf 50 720 Td (%s) Tj ET",
		escapePDF(title), escapePDF(content))

	// Build each object as a string so we can compute exact byte offsets.
	header := "%PDF-1.4\n"
	obj1 := "1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n"
	obj2 := "2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n"
	obj3 := "3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]/Contents 4 0 R/Resources<</Font<</F1 5 0 R>>>>>>endobj\n"
	obj4 := fmt.Sprintf("4 0 obj<</Length %d>>\nstream\n%s\nendstream\nendobj\n", len(streamBody), streamBody)
	obj5 := "5 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>endobj\n"

	// Compute byte offsets for each object.
	off1 := len(header)
	off2 := off1 + len(obj1)
	off3 := off2 + len(obj2)
	off4 := off3 + len(obj3)
	off5 := off4 + len(obj4)
	xrefOffset := off5 + len(obj5)

	xref := fmt.Sprintf(
		"xref\n0 6\n%010d 65535 f \n%010d 00000 n \n%010d 00000 n \n%010d 00000 n \n%010d 00000 n \n%010d 00000 n \n",
		0, off1, off2, off3, off4, off5,
	)
	trailer := fmt.Sprintf("trailer<</Size 6/Root 1 0 R>>\nstartxref\n%d\n%%%%EOF\n", xrefOffset)

	_, err = f.WriteString(header + obj1 + obj2 + obj3 + obj4 + obj5 + xref + trailer)
	return err
}

func escapePDF(s string) string {
	// Basic PDF string escaping
	result := ""
	for _, c := range s {
		switch c {
		case '(', ')':
			result += "\\" + string(c)
		case '\\':
			result += "\\\\"
		case '\n':
			result += ") Tj ET\nBT /F1 12 Tf 50 700 Td ("
		default:
			result += string(c)
		}
	}
	return result
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

// ProcessQueuedExports is called by the worker to drain export jobs.
func ProcessQueuedExports(ctx context.Context, db *pgxpool.Pool, rdb *redis.Client, exportDir string) (int, error) {
	rows, err := db.Query(ctx, `
		UPDATE export_jobs SET status = 'processing', updated_at = NOW()
		WHERE status = 'queued'
		RETURNING id
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	h := &Handler{db: db, rdb: rdb, exportDir: exportDir}
	processed := 0
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			h.processClaimedExport(ctx, id)
			processed++
		}
	}
	return processed, nil
}
