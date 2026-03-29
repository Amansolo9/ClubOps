package models

import "time"

type User struct {
	ID             int64
	Username       string
	PasswordHash   string
	Role           string
	ClubID         *int64
	FailedAttempts int
	LockedUntil    *time.Time
	MustChangePass bool
	PasswordSetAt  time.Time
	CreatedAt      time.Time
}

type Session struct {
	Token     string
	UserID    int64
	ExpiresAt time.Time
	CreatedAt time.Time
}

type Club struct {
	ID              int64
	Name            string
	Tags            string
	AvatarPath      string
	RecruitmentOpen bool
	Description     string
}

type Member struct {
	ID             int64
	ClubID         int64
	FullName       string
	EmailEncrypted string
	PhoneEncrypted string
	JoinDate       string
	PositionTitle  string
	IsActive       bool
	GroupName      string
	CustomFields   string
	CreatedAt      time.Time
}

type DimensionVersion struct {
	ID            int64
	DimensionName string
	Label         string
	CreatedBy     int64
	CreatedAt     time.Time
}

type DimensionValue struct {
	ID        int64
	VersionID int64
	Code      string
	Value     string
}

type Budget struct {
	ID             int64
	ClubID         int64
	AccountCode    string
	CampusCode     string
	ProjectCode    string
	PeriodType     string
	PeriodStart    string
	Amount         float64
	Spent          float64
	ThresholdAlert bool
	CreatedBy      int64
	Status         string
	CreatedAt      time.Time
}

type BudgetChangeRequest struct {
	ID             int64
	BudgetID       int64
	RequestedBy    int64
	ProposedAmount float64
	ChangePercent  float64
	Reason         string
	Status         string
	ReviewedBy     *int64
	CreatedAt      time.Time
}

type RegionVersion struct {
	ID        int64
	Label     string
	CreatedBy int64
	CreatedAt time.Time
}

type RegionNode struct {
	ID        int64
	VersionID int64
	State     string
	County    string
	City      string
}

type FulfilledOrder struct {
	ID           int64
	ClubID       int64
	SiteID       int64
	MemberID     int64
	OwnerUserID  int64
	ServiceLabel string
	Status       string
	FulfilledAt  time.Time
	CreatedAt    time.Time
}

type CreditRuleVersion struct {
	ID            int64
	VersionLabel  string
	FormulaJSON   string
	MakeupEnabled bool
	RetakeEnabled bool
	EffectiveFrom string
	EffectiveTo   *string
	CreatedBy     int64
	CreatedAt     time.Time
	IsActive      bool
}

type CreditIssued struct {
	ID               int64
	MemberID         int64
	RuleVersionID    int64
	BaseScore        float64
	MakeupUsed       bool
	RetakeUsed       bool
	CalculatedCredit float64
	ImmutableHash    string
	IssuedAt         time.Time
}

type Review struct {
	ID               int64
	ClubID           int64
	FulfilledOrderID *int64
	SiteID           int64
	MemberID         int64
	ReviewerID       int64
	Stars            int
	Tags             string
	Comment          string
	ImagePaths       string
	AppealStatus     string
	HiddenReason     *string
	CreatedAt        time.Time
}

type FeatureFlag struct {
	ID          int64
	FlagKey     string
	Enabled     bool
	TargetScope string
	RolloutPct  int
	UpdatedBy   int64
	CreatedAt   time.Time
}

type SalesFact struct {
	ID              int64
	ProductCode     string
	CustomerCode    string
	ChannelCode     string
	RegionCode      string
	TimeCode        string
	Amount          float64
	TransactionDate string
	CreatedAt       time.Time
}
