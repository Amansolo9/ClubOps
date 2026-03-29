package handlers

import (
	"clubops_portal/internal/middleware"
	"clubops_portal/internal/models"
	"clubops_portal/internal/services"
	"clubops_portal/internal/store"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store   *store.SQLiteStore
	auth    *services.AuthService
	finance *services.FinanceService
	credit  *services.CreditService
	review  *services.ReviewService
	mdm     *services.MDMService
	crypto  *services.CryptoService
	flags   *services.FlagService
}

type budgetListRow struct {
	models.Budget
	ExecutionPct float64
	Remaining    float64
}

type clubView struct {
	ID              int64
	Name            string
	TagsRaw         string
	Tags            []string
	AvatarPath      string
	RecruitmentOpen bool
	Description     string
}

type regionEditorView struct {
	Version *models.RegionVersion
	Rows    []models.RegionNode
	CSVRows string
}

type userRowView struct {
	ID             int64
	Username       string
	Role           string
	ClubLabel      string
	MustChangePass bool
}

type mdmPageView struct {
	Dimensions []models.DimensionVersion
	SalesFacts []models.SalesFact
}

func NewHandler(st *store.SQLiteStore, auth *services.AuthService, finance *services.FinanceService, credit *services.CreditService, review *services.ReviewService, mdm *services.MDMService, crypto *services.CryptoService, flags *services.FlagService) *Handler {
	return &Handler{store: st, auth: auth, finance: finance, credit: credit, review: review, mdm: mdm, crypto: crypto, flags: flags}
}

func (h *Handler) RegisterRoutes(app *fiber.App) {
	app.Static("/static", "./static")

	app.Get("/", h.dashboard)
	app.Get("/clubs/recruiting", h.publicRecruiting)
	app.Get("/login", h.loginPage)
	app.Post("/login", h.loginAction)
	app.Post("/register", h.register)

	secured := app.Group("", middleware.RequireAuth())
	secured.Post("/logout", h.logout)
	secured.Get("/change-password", h.changePasswordPage)
	secured.Get("/budgets", middleware.RequireAuth("admin", "organizer", "team_lead"), h.budgetsPage)
	secured.Get("/reviews", h.reviewsPage)
	secured.Get("/credits", middleware.RequireAuth("admin", "organizer", "team_lead"), h.creditsPage)
	secured.Get("/members", middleware.RequireAuth("admin", "organizer", "team_lead"), h.membersPage)
	secured.Get("/clubs", middleware.RequireAuth("admin", "organizer", "team_lead"), h.clubsPage)
	secured.Get("/regions", middleware.RequireAuth("admin", "organizer"), h.regionsPage)
	secured.Get("/mdm", middleware.RequireAuth("admin", "organizer"), h.mdmPage)
	secured.Get("/users", middleware.RequireAuth("admin"), h.usersPage)
	secured.Get("/flags", middleware.RequireAuth("admin"), h.flagsPage)
	secured.Get("/partials/budgets/list", middleware.RequireAuth("admin", "organizer", "team_lead"), h.budgetsListPartial)
	secured.Get("/partials/budgets/changes", middleware.RequireAuth("admin", "organizer", "team_lead"), h.budgetChangesPartial)
	secured.Get("/partials/reviews/list", h.reviewsListPartial)
	secured.Get("/partials/fulfilled-orders/options", h.fulfilledOrdersOptionsPartial)

	api := app.Group("/api", middleware.RequireAuth())
	api.Post("/auth/change-password", h.changePassword)
	api.Post("/auth/admin-reset", middleware.RequireAuth("admin"), h.adminResetPassword)
	api.Post("/budgets", middleware.RequireAuth("admin", "organizer", "team_lead"), h.createBudget)
	api.Post("/budgets/:id/change", middleware.RequireAuth("admin", "organizer", "team_lead"), h.requestBudgetChange)
	api.Post("/budget_change_requests/:id/review", middleware.RequireAuth("admin"), h.reviewBudgetChange)
	api.Get("/budgets/:id/projection", middleware.RequireAuth("admin", "organizer", "team_lead"), h.budgetProjection)
	api.Post("/budgets/:id/spend", middleware.RequireAuth("admin", "organizer", "team_lead"), h.recordBudgetSpend)
	api.Post("/reviews", middleware.RequireAuth("admin", "organizer", "team_lead", "member"), h.createReview)
	api.Post("/fulfilled-orders", middleware.RequireAuth("admin", "organizer", "team_lead"), h.createFulfilledOrder)
	api.Post("/reviews/:id/appeal", middleware.RequireAuth("admin", "organizer", "team_lead", "member"), h.appealReview)
	api.Post("/reviews/:id/moderate", middleware.RequireAuth("admin", "organizer"), h.moderateReview)
	api.Post("/credit_rules", middleware.RequireAuth("admin"), h.createCreditRule)
	api.Post("/credits/issue", middleware.RequireAuth("admin", "organizer", "team_lead"), h.issueCredit)
	api.Post("/regions/import", middleware.RequireAuth("admin", "organizer"), h.importRegions)
	api.Get("/regions/:id", middleware.RequireAuth("admin", "organizer"), h.getRegionVersion)
	api.Post("/regions/:id", middleware.RequireAuth("admin", "organizer"), h.updateRegionVersion)
	api.Post("/mdm/dimensions/import", middleware.RequireAuth("admin", "organizer"), h.importDimension)
	api.Post("/mdm/sales-facts/import", middleware.RequireAuth("admin", "organizer"), h.importSalesFacts)
	api.Post("/members", middleware.RequireAuth("admin", "organizer", "team_lead"), h.createMember)
	api.Get("/members/export", middleware.RequireAuth("admin", "organizer", "team_lead"), h.exportMembersCSV)
	api.Post("/members/import", middleware.RequireAuth("admin", "organizer", "team_lead"), h.importMembersCSV)
	api.Post("/members/:id", middleware.RequireAuth("admin", "organizer", "team_lead"), h.updateMember)
	api.Post("/clubs", middleware.RequireAuth("admin"), h.createClub)
	api.Post("/clubs/:id/profile", middleware.RequireAuth("admin", "organizer", "team_lead"), h.updateClubProfile)
	api.Post("/users/:id", middleware.RequireAuth("admin"), h.updateUser)
	api.Post("/flags", middleware.RequireAuth("admin"), h.upsertFlag)
	api.Get("/flags/evaluate/:key", h.evaluateFlag)
}
