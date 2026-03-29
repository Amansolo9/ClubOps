package handlers

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) importRegions(c *fiber.Ctx) error {
	user := currentUser(c)
	fh, err := c.FormFile("file")
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "file required")
	}
	f, err := fh.Open()
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Request could not be processed.")
	}
	defer f.Close()
	id, err := h.mdm.ImportRegionCSV(f, c.FormValue("version_label"), user.ID)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	return c.JSON(fiber.Map{"version_id": id})
}

func (h *Handler) getRegionVersion(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid region version id")
	}
	version, rows, err := h.mdm.GetRegionVersion(id)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	return c.JSON(fiber.Map{"version": version, "rows": rows})
}

func (h *Handler) updateRegionVersion(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid region version id")
	}
	rows, err := parseRegionRowsCSV(c.FormValue("rows_csv"))
	if err != nil {
		return h.writeServiceError(c, err)
	}
	newID, err := h.mdm.UpdateRegionVersion(id, c.FormValue("version_label"), rows, currentUser(c).ID)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	return c.SendString("region version updated as snapshot #" + strconv.FormatInt(newID, 10))
}

func (h *Handler) importDimension(c *fiber.Ctx) error {
	user := currentUser(c)
	fh, err := c.FormFile("file")
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "file required")
	}
	f, err := fh.Open()
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Request could not be processed.")
	}
	defer f.Close()
	dimensionName := c.FormValue("dimension_name")
	if strings.TrimSpace(dimensionName) == "" {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "dimension_name required")
	}
	id, err := h.mdm.ImportDimensionCSV(f, dimensionName, c.FormValue("version_label"), user.ID)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	return c.JSON(fiber.Map{"version_id": id})
}

func (h *Handler) importSalesFacts(c *fiber.Ctx) error {
	fh, err := c.FormFile("file")
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "file required")
	}
	f, err := fh.Open()
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Request could not be processed.")
	}
	defer f.Close()
	count, err := h.mdm.ImportSalesFactCSV(f)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	return c.JSON(fiber.Map{"rows_imported": count})
}
