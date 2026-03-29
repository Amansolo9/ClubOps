package services

import (
	"database/sql"
	"hash/fnv"
	"strconv"
	"strings"

	"clubops_portal/internal/models"
	"clubops_portal/internal/store"
)

type FlagService struct {
	store *store.SQLiteStore
}

func NewFlagService(st *store.SQLiteStore) *FlagService {
	return &FlagService{store: st}
}

func (s *FlagService) IsEnabledForUser(flagKey string, user *models.User) bool {
	flag, err := s.store.GetFeatureFlagByKey(flagKey)
	if err != nil {
		if err == sql.ErrNoRows {
			return false
		}
		return false
	}
	if !flag.Enabled {
		return false
	}
	target := strings.TrimSpace(strings.ToLower(flag.TargetScope))
	if target == "global" || target == "" {
		return inRollout(flag.RolloutPct, user)
	}
	if strings.HasPrefix(target, "role:") {
		role := strings.TrimPrefix(target, "role:")
		if strings.ToLower(user.Role) != role {
			return false
		}
		return inRollout(flag.RolloutPct, user)
	}
	if strings.HasPrefix(target, "club:") {
		if user.ClubID == nil {
			return false
		}
		club := strings.TrimPrefix(target, "club:")
		return club == strconv.FormatInt(*user.ClubID, 10) && inRollout(flag.RolloutPct, user)
	}
	return inRollout(flag.RolloutPct, user)
}

func inRollout(pct int, user *models.User) bool {
	if pct <= 0 {
		return false
	}
	if pct >= 100 {
		return true
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(user.Username))
	bucket := int(h.Sum32() % 100)
	return bucket < pct
}
