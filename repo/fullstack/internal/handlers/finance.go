package handlers

import (
	"strconv"

	"clubops_portal/fullstack/internal/models"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) createBudget(c *fiber.Ctx) error {
	user := currentUser(c)
	clubID := int64(1)
	if user.Role == "admin" {
		var err error
		clubID, err = parseInt64WithDefault(c.FormValue("club_id"), 1)
		if err != nil {
			return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid club_id")
		}
	} else {
		if user.ClubID == nil {
			return apiError(c, fiber.StatusForbidden, "club_scope_required", "club scope required")
		}
		clubID = *user.ClubID
	}

	amount, err := strconv.ParseFloat(c.FormValue("amount"), 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid amount")
	}
	id, err := h.finance.CreateBudget(clubID, c.FormValue("account_code"), c.FormValue("campus_code"), c.FormValue("project_code"), c.FormValue("period_type"), c.FormValue("period_start"), amount, user.ID)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	c.Set("HX-Trigger", `{"budgetsUpdated":true}`)
	if c.Get("HX-Request") == "true" {
		return c.SendString("<div class='text-emerald-700'>Budget created #" + strconv.FormatInt(id, 10) + "</div>")
	}
	return c.JSON(fiber.Map{"id": id})
}

func (h *Handler) requestBudgetChange(c *fiber.Ctx) error {
	user := currentUser(c)
	budgetID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid budget id")
	}
	ok, err := h.store.CanAccessBudget(user, budgetID)
	if err != nil {
		return apiError(c, fiber.StatusNotFound, "not_found", "Resource not found.")
	}
	if !ok {
		return apiError(c, fiber.StatusForbidden, "forbidden", "You are not allowed to perform this action.")
	}
	proposed, err := strconv.ParseFloat(c.FormValue("proposed_amount"), 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid proposed_amount")
	}
	changeID, err := h.finance.RequestBudgetChange(budgetID, user.ID, proposed, c.FormValue("reason"), user.Role, user.ClubID)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	c.Set("HX-Trigger", `{"budgetsUpdated":true}`)
	if changeID == 0 {
		return c.SendString("Budget updated directly (<10% change)")
	}
	return c.SendString("Budget change request submitted")
}

func (h *Handler) reviewBudgetChange(c *fiber.Ctx) error {
	user := currentUser(c)
	changeID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid change id")
	}
	approve := c.FormValue("decision") == "approve"
	if err := h.finance.ApproveChange(changeID, user.ID, approve); err != nil {
		return h.writeServiceError(c, err)
	}
	c.Set("HX-Trigger", `{"budgetsUpdated":true}`)
	return c.SendString("review saved")
}

func (h *Handler) budgetProjection(c *fiber.Ctx) error {
	user := currentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid id")
	}
	ok, err := h.store.CanAccessBudget(user, id)
	if err != nil {
		return apiError(c, fiber.StatusNotFound, "not_found", "Resource not found.")
	}
	if !ok {
		return apiError(c, fiber.StatusForbidden, "forbidden", "You are not allowed to perform this action.")
	}
	b, err := h.store.GetBudgetByID(id)
	if err != nil {
		return apiError(c, fiber.StatusNotFound, "not_found", "Resource not found.")
	}
	remainingSpend, err := strconv.ParseFloat(c.Query("expected_remaining_spend", "0"), 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid expected_remaining_spend")
	}
	return c.JSON(fiber.Map{"budget_id": b.ID, "projected_end_balance": h.finance.Projection(*b, remainingSpend)})
}

func (h *Handler) recordBudgetSpend(c *fiber.Ctx) error {
	user := currentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid budget id")
	}
	ok, err := h.store.CanAccessBudget(user, id)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	if !ok {
		return apiError(c, fiber.StatusForbidden, "forbidden", "You are not allowed to perform this action.")
	}
	spent, err := strconv.ParseFloat(c.FormValue("spent"), 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid spent")
	}
	if err := h.finance.RecordSpend(id, spent); err != nil {
		return h.writeServiceError(c, err)
	}
	c.Set("HX-Trigger", `{"budgetsUpdated":true}`)
	return c.SendString("budget spend updated")
}

func (h *Handler) budgetsListPartial(c *fiber.Ctx) error {
	if err := requireManagedClub(currentUser(c)); err != nil {
		return c.Status(403).SendString(err.Error())
	}
	budgets, err := h.finance.ListBudgets(scopedClubIDForUser(currentUser(c), c))
	if err != nil {
		return h.writeServiceError(c, err)
	}
	rows := make([]budgetListRow, 0, len(budgets))
	for _, budget := range budgets {
		executionPct := 0.0
		if budget.Amount > 0 {
			executionPct = (budget.Spent / budget.Amount) * 100
		}
		rows = append(rows, budgetListRow{Budget: budget, ExecutionPct: executionPct, Remaining: budget.Amount - budget.Spent})
	}
	return c.Render("partials/budgets_list", fiber.Map{"Budgets": rows, "User": currentUser(c)})
}

func (h *Handler) budgetChangesPartial(c *fiber.Ctx) error {
	if err := requireManagedClub(currentUser(c)); err != nil {
		return c.Status(403).SendString(err.Error())
	}
	changes, err := h.finance.ListPendingChanges(scopedClubIDForUser(currentUser(c), c))
	if err != nil {
		return h.writeServiceError(c, err)
	}
	return c.Render("partials/budget_changes", fiber.Map{"Changes": changes, "User": currentUser(c)})
}

func (h *Handler) reviewsListPartial(c *fiber.Ctx) error {
	user := currentUser(c)
	var reviews []models.Review
	if user != nil && user.Role == "member" {
		var err error
		reviews, err = h.store.ListReviewsByReviewer(user.ID)
		if err != nil {
			return h.writeServiceError(c, err)
		}
	} else {
		if err := requireManagedClub(user); err != nil {
			return c.Status(403).SendString(err.Error())
		}
		var err error
		reviews, err = h.review.ListReviews(scopedClubIDForUser(user, c))
		if err != nil {
			return h.writeServiceError(c, err)
		}
	}
	return c.Render("partials/reviews_list", fiber.Map{"Reviews": reviews, "User": currentUser(c)})
}
