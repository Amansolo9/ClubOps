package handlers

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) budgetsPage(c *fiber.Ctx) error {
	if err := requireManagedClub(currentUser(c)); err != nil {
		return c.Status(403).SendString(err.Error())
	}
	return h.render(c, "budgets", fiber.Map{"User": currentUser(c)}, "layouts/main")
}

func (h *Handler) reviewsPage(c *fiber.Ctx) error {
	user := currentUser(c)
	if user != nil && user.Role == "organizer" && user.ClubID == nil {
		return c.Status(403).SendString("club scope required")
	}
	return h.render(c, "reviews", fiber.Map{"User": currentUser(c)}, "layouts/main")
}

func (h *Handler) creditsPage(c *fiber.Ctx) error {
	return h.render(c, "credits", fiber.Map{"User": currentUser(c)}, "layouts/main")
}

func (h *Handler) membersPage(c *fiber.Ctx) error {
	user := currentUser(c)
	if err := requireManagedClub(user); err != nil {
		return c.Status(403).SendString("club scope required")
	}
	var clubID *int64
	selectedClub := ""
	clubViews := []clubView{}
	if user != nil && user.Role == "admin" {
		clubs, err := h.store.ListClubs()
		if err != nil {
			return h.writeServiceError(c, err)
		}
		clubViews = buildClubViews(clubs)
		requested := strings.TrimSpace(c.Query("club_id"))
		switch requested {
		case "", "all":
			clubID = nil
			selectedClub = "all"
		default:
			parsed, err := strconv.ParseInt(requested, 10, 64)
			if err != nil {
				return c.Status(400).SendString("invalid club_id")
			}
			clubID = &parsed
			selectedClub = requested
		}
	} else {
		clubID = userClubID(user)
		if clubID == nil {
			def := int64(1)
			clubID = &def
		}
	}
	limit, err := strconv.Atoi(c.Query("limit", "50"))
	if err != nil {
		return c.Status(400).SendString("invalid limit")
	}
	offset, err := strconv.Atoi(c.Query("offset", "0"))
	if err != nil {
		return c.Status(400).SendString("invalid offset")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	members, err := h.store.ListMembersPagedScoped(clubID, c.Query("group"), c.Query("search"), c.Query("sort", "created_at"), limit, offset)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	for i := range members {
		email, _ := h.crypto.Decrypt(members[i].EmailEncrypted)
		phone, _ := h.crypto.Decrypt(members[i].PhoneEncrypted)
		members[i].EmailEncrypted = email
		members[i].PhoneEncrypted = phone
		members[i].CustomFields = h.decryptAndMaybeMigrateCustomFields(&members[i])
	}
	return h.render(c, "members", fiber.Map{"Members": members, "User": user, "Clubs": clubViews, "SelectedClub": selectedClub}, "layouts/main")
}

func (h *Handler) clubsPage(c *fiber.Ctx) error {
	user := currentUser(c)
	if err := requireManagedClub(user); err != nil {
		return c.Status(403).SendString(err.Error())
	}
	if user != nil && user.Role == "admin" {
		clubs, err := h.store.ListClubs()
		if err != nil {
			return h.writeServiceError(c, err)
		}
		clubViews := buildClubViews(clubs)
		var selected *clubView
		if requested := c.Query("club_id"); requested != "" {
			if id, err := strconv.ParseInt(requested, 10, 64); err == nil {
				for i := range clubViews {
					if clubViews[i].ID == id {
						selected = &clubViews[i]
						break
					}
				}
			}
		}
		if selected == nil && len(clubViews) > 0 {
			selected = &clubViews[0]
		}
		return h.render(c, "clubs", fiber.Map{"Club": selected, "Clubs": clubViews, "User": user}, "layouts/main")
	}
	clubID := int64(1)
	if id := userClubID(user); id != nil {
		clubID = *id
	}
	club, err := h.store.GetClubByID(clubID)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	return h.render(c, "clubs", fiber.Map{"Club": buildClubView(club), "User": user}, "layouts/main")
}

func (h *Handler) regionsPage(c *fiber.Ctx) error {
	versions, err := h.mdm.ListRegionVersions()
	if err != nil {
		return h.writeServiceError(c, err)
	}
	var selected *regionEditorView
	if len(versions) > 0 {
		version, rows, err := h.mdm.GetRegionVersion(versions[0].ID)
		if err != nil {
			return h.writeServiceError(c, err)
		}
		selected = &regionEditorView{Version: version, Rows: rows, CSVRows: regionRowsToCSV(rows)}
	}
	return h.render(c, "regions", fiber.Map{"Versions": versions, "Selected": selected, "User": currentUser(c)}, "layouts/main")
}

func (h *Handler) mdmPage(c *fiber.Ctx) error {
	dimensions, err := h.mdm.ListDimensionVersions("")
	if err != nil {
		return h.writeServiceError(c, err)
	}
	salesFacts, err := h.mdm.ListRecentSalesFacts(20)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	return h.render(c, "mdm", fiber.Map{"MDM": mdmPageView{Dimensions: dimensions, SalesFacts: salesFacts}, "User": currentUser(c)}, "layouts/main")
}

func (h *Handler) flagsPage(c *fiber.Ctx) error {
	flags, err := h.store.ListFeatureFlags()
	if err != nil {
		return h.writeServiceError(c, err)
	}
	return h.render(c, "flags", fiber.Map{"Flags": flags, "User": currentUser(c)}, "layouts/main")
}

func (h *Handler) usersPage(c *fiber.Ctx) error {
	users, err := h.store.ListUsers(nil)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	rows := make([]userRowView, 0, len(users))
	for _, user := range users {
		clubLabel := "unassigned"
		if user.ClubID != nil {
			clubLabel = strconv.FormatInt(*user.ClubID, 10)
		}
		rows = append(rows, userRowView{ID: user.ID, Username: user.Username, Role: user.Role, ClubLabel: clubLabel, MustChangePass: user.MustChangePass})
	}
	return h.render(c, "users", fiber.Map{"Users": rows, "User": currentUser(c)}, "layouts/main")
}
