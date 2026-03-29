package handlers

import (
	"encoding/json"
	"strings"

	"clubops_portal/internal/models"
)

const customFieldsEncPrefix = "enc:v1:"

func (h *Handler) encryptCustomFields(raw string) string {
	return customFieldsEncPrefix + h.crypto.Encrypt(raw)
}

func (h *Handler) decryptAndMaybeMigrateCustomFields(member *models.Member) string {
	stored := strings.TrimSpace(member.CustomFields)
	if stored == "" {
		return "{}"
	}
	if strings.HasPrefix(stored, customFieldsEncPrefix) {
		plain, _ := h.crypto.Decrypt(strings.TrimPrefix(stored, customFieldsEncPrefix))
		if json.Valid([]byte(plain)) {
			return plain
		}
		return "{}"
	}
	if json.Valid([]byte(stored)) {
		member.CustomFields = h.encryptCustomFields(stored)
		_ = h.store.UpdateMember(*member)
		return stored
	}
	decrypted, _ := h.crypto.Decrypt(stored)
	if json.Valid([]byte(decrypted)) {
		member.CustomFields = h.encryptCustomFields(decrypted)
		_ = h.store.UpdateMember(*member)
		return decrypted
	}
	return "{}"
}
