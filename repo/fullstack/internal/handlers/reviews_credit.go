package handlers

import (
	"encoding/json"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	"clubops_portal/fullstack/internal/services"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) createReview(c *fiber.Ctx) error {
	user := currentUser(c)
	if user != nil && (user.Role == "organizer" || user.Role == "team_lead") && user.ClubID == nil {
		return apiError(c, fiber.StatusForbidden, "club_scope_required", "club scope required")
	}
	orderID, err := strconv.ParseInt(c.FormValue("fulfilled_order_id"), 10, 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid fulfilled_order_id")
	}
	order, err := h.store.GetFulfilledOrderByID(orderID)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	if user.Role == "member" && order.OwnerUserID != user.ID {
		return apiError(c, fiber.StatusForbidden, "forbidden", "You are not allowed to perform this action.")
	}
	if user.ClubID != nil && *user.ClubID != order.ClubID {
		return apiError(c, fiber.StatusForbidden, "forbidden", "You are not allowed to perform this action.")
	}
	stars, err := strconv.Atoi(c.FormValue("stars"))
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid stars")
	}
	form, err := c.MultipartForm()
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Request could not be processed.")
	}
	var files []*multipart.FileHeader
	if form != nil && form.File != nil {
		files = form.File["images"]
	}
	tags := strings.Split(c.FormValue("tags"), ",")
	_, err = h.review.CreateReviewForOrder(orderID, user.ID, stars, tags, c.FormValue("comment"), files)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	c.Set("HX-Trigger", `{"reviewsUpdated":true}`)
	return c.SendString("review submitted")
}

func (h *Handler) appealReview(c *fiber.Ctx) error {
	user := currentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid review id")
	}
	if err := h.review.AppealReview(id, user.ID, user.ClubID); err != nil {
		return h.writeServiceError(c, err)
	}
	c.Set("HX-Trigger", `{"reviewsUpdated":true}`)
	return c.SendString("appeal queued")
}

func (h *Handler) moderateReview(c *fiber.Ctx) error {
	user := currentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid review id")
	}
	review, err := h.store.GetReviewByID(id)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	if user.Role == "organizer" && (user.ClubID == nil || *user.ClubID != review.ClubID) {
		return apiError(c, fiber.StatusForbidden, "forbidden", "You are not allowed to perform this action.")
	}
	hide := c.FormValue("decision") == "hide"
	if err := h.review.ModerateReview(id, hide, c.FormValue("reason")); err != nil {
		return h.writeServiceError(c, err)
	}
	c.Set("HX-Trigger", `{"reviewsUpdated":true}`)
	return c.SendString("moderation saved")
}

func (h *Handler) createCreditRule(c *fiber.Ctx) error {
	user := currentUser(c)
	formula := services.CreditFormula{}
	var err error
	formula.Weight, err = strconv.ParseFloat(c.FormValue("weight", "1"), 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid weight")
	}
	formula.MakeupBonus, err = strconv.ParseFloat(c.FormValue("makeup_bonus", "0"), 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid makeup_bonus")
	}
	formula.RetakeFactor, err = strconv.ParseFloat(c.FormValue("retake_factor", "1"), 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid retake_factor")
	}
	if raw := strings.TrimSpace(c.FormValue("thresholds_json")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &formula.Thresholds); err != nil {
			return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid thresholds_json")
		}
	}
	if raw := strings.TrimSpace(c.FormValue("deductions_json")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &formula.Deductions); err != nil {
			return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid deductions_json")
		}
	}
	var effectiveTo *string
	if c.FormValue("effective_to") != "" {
		v := c.FormValue("effective_to")
		effectiveTo = &v
	}
	_, err = h.credit.CreateRule(c.FormValue("version"), formula, c.FormValue("makeup_enabled") == "true", c.FormValue("retake_enabled") == "true", c.FormValue("effective_from", time.Now().Format("2006-01-02")), effectiveTo, user.ID, true)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	return c.SendString("credit rule created")
}

func (h *Handler) issueCredit(c *fiber.Ctx) error {
	user := currentUser(c)
	memberID, err := strconv.ParseInt(c.FormValue("member_id"), 10, 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid member_id")
	}
	member, err := h.store.GetMemberByID(memberID)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	if user.Role != "admin" && (user.ClubID == nil || *user.ClubID != member.ClubID) {
		return apiError(c, fiber.StatusForbidden, "forbidden", "You are not allowed to perform this action.")
	}
	base, err := strconv.ParseFloat(c.FormValue("base_score"), 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid base_score")
	}
	if !h.flags.IsEnabledForUser("credit_engine_v2", currentUser(c)) {
		return apiError(c, fiber.StatusForbidden, "forbidden", "credit engine feature disabled for your scope")
	}
	txnDate := c.FormValue("txn_date", time.Now().Format("2006-01-02"))
	id, credit, err := h.credit.IssueCredit(memberID, base, c.FormValue("makeup") == "true", c.FormValue("retake") == "true", txnDate)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	if c.Get("HX-Request") == "true" {
		return c.SendString("<div class='text-emerald-700'>Issued #" + strconv.FormatInt(id, 10) + " with credit " + strconv.FormatFloat(credit, 'f', 2, 64) + "</div>")
	}
	return c.JSON(fiber.Map{"id": id, "credit": credit})
}
