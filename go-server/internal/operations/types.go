package operations

type SubscriptionPlan struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Description        *string  `json:"description"`
	Amount             float64  `json:"amount"`
	DurationDays       int      `json:"durationDays"`
	QuotaImages        int      `json:"quotaImages"`
	BonusCredits       float64  `json:"-"`
	DiscountPercent    float64  `json:"discountPercent"`
	AllowedProviderIDs []string `json:"allowedProviderIds"`
	AllowedModelIDs    []string `json:"allowedModelIds"`
	Badge              *string  `json:"badge"`
	SortOrder          int      `json:"sortOrder"`
	Status             string   `json:"status"`
	CreatedAt          string   `json:"createdAt"`
	UpdatedAt          string   `json:"updatedAt"`
}

type FreeQuotaLimits struct {
	Hourly  int
	Daily   int
	Monthly int
}

type SubscriptionQuotaWindow struct {
	Key             string `json:"key"`
	Label           string `json:"label"`
	QuotaLimit      int    `json:"quotaLimit"`
	QuotaUsed       int    `json:"quotaUsed"`
	QuotaRemaining  int    `json:"quotaRemaining"`
	PeriodStartedAt string `json:"periodStartedAt"`
	PeriodEndsAt    string `json:"periodEndsAt"`
}

type SubscriptionEntitlement struct {
	ID                 string                    `json:"id,omitempty"`
	Status             string                    `json:"status"`
	Tier               string                    `json:"tier"`
	IsPaid             bool                      `json:"isPaid"`
	Source             string                    `json:"source,omitempty"`
	StartedAt          string                    `json:"startedAt,omitempty"`
	ExpiresAt          string                    `json:"expiresAt,omitempty"`
	PeriodStartedAt    string                    `json:"periodStartedAt"`
	PeriodEndsAt       string                    `json:"periodEndsAt"`
	PlanID             string                    `json:"planId,omitempty"`
	PlanName           string                    `json:"planName"`
	DiscountPercent    float64                   `json:"discountPercent"`
	AllowedProviderIDs []string                  `json:"allowedProviderIds"`
	AllowedModelIDs    []string                  `json:"allowedModelIds"`
	QuotaImages        int                       `json:"quotaImages"`
	QuotaLimit         int                       `json:"quotaLimit"`
	QuotaUsed          int                       `json:"quotaUsed"`
	QuotaRemaining     int                       `json:"quotaRemaining"`
	EffectiveRemaining int                       `json:"effectiveQuotaRemaining"`
	QuotaUnlimited     bool                      `json:"quotaUnlimited"`
	QuotaWindows       []SubscriptionQuotaWindow `json:"quotaWindows,omitempty"`
	Plan               *SubscriptionPlan         `json:"plan,omitempty"`
}

type CustomSubscriptionGrant struct {
	Name         string `json:"name"`
	DurationDays int    `json:"durationDays"`
	QuotaImages  int    `json:"quotaImages"`
}

type Invite struct {
	ID                    string  `json:"id"`
	InviterID             string  `json:"inviterId"`
	InviterEmail          *string `json:"inviterEmail,omitempty"`
	InviteeID             string  `json:"inviteeId"`
	InviteeEmail          *string `json:"inviteeEmail,omitempty"`
	RewardCredits         float64 `json:"rewardCredits"`
	RewardType            string  `json:"rewardType"`
	RewardPlanID          *string `json:"rewardPlanId,omitempty"`
	RewardLabel           *string `json:"rewardLabel,omitempty"`
	InviteeRewardCredits  float64 `json:"inviteeRewardCredits"`
	InviteeRewardType     string  `json:"inviteeRewardType"`
	InviteeRewardPlanID   *string `json:"inviteeRewardPlanId,omitempty"`
	InviteeRewardLabel    *string `json:"inviteeRewardLabel,omitempty"`
	Status                string  `json:"status"`
	RiskReason            *string `json:"riskReason,omitempty"`
	InviteeIP             *string `json:"inviteeIp"`
	VerifiedAt            *string `json:"verifiedAt,omitempty"`
	RewardedAt            *string `json:"rewardedAt,omitempty"`
	RechargeRebateCount   int     `json:"rechargeRebateCount"`
	RechargeRebateCredits float64 `json:"rechargeRebateCredits"`
	CreatedAt             string  `json:"createdAt"`
}

type InviteRewardSpec struct {
	Type    string
	Credits float64
	PlanID  string
}

type InviteRiskLimits struct {
	Enabled          bool
	BlockSameIP      bool
	BlockSameDevice  bool
	MaxPerIP24h      int
	MaxPerDevice24h  int
	MaxPerInviter24h int
}

type InviteBindingInput struct {
	InviterID     string
	InviteeID     string
	InviteeIP     string
	IPHash        string
	DeviceHash    string
	InviterReward InviteRewardSpec
	InviteeReward InviteRewardSpec
	Risk          InviteRiskLimits
}

type InviteRewardResult struct {
	InviteID   string `json:"inviteId"`
	InviterID  string `json:"inviterId"`
	InviteeID  string `json:"inviteeId"`
	Status     string `json:"status"`
	RiskReason string `json:"riskReason,omitempty"`
}

type InviteRechargeRebateConfig struct {
	Enabled              bool
	Percent              float64
	RechargeRate         float64
	IncludeSubscriptions bool
}

type InviteRechargeRebate struct {
	ID            string  `json:"id"`
	InviteID      string  `json:"inviteId"`
	OrderID       string  `json:"orderId"`
	InviterID     string  `json:"inviterId"`
	InviteeID     string  `json:"inviteeId"`
	InviteeEmail  *string `json:"inviteeEmail,omitempty"`
	OrderType     string  `json:"orderType"`
	OrderAmount   float64 `json:"orderAmount"`
	RechargeRate  float64 `json:"rechargeRate"`
	RebatePercent float64 `json:"rebatePercent"`
	RebateCredits float64 `json:"rebateCredits"`
	OutTradeNo    string  `json:"outTradeNo"`
	CreatedAt     string  `json:"createdAt"`
}

type InviteDeleteResult struct {
	Deleted             bool   `json:"deleted"`
	InviterID           string `json:"inviterId,omitempty"`
	SubscriptionRevoked bool   `json:"subscriptionRevoked"`
	RevokedDays         int    `json:"revokedDays,omitempty"`
}

type LotteryPrize struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	PrizeType     string  `json:"prizeType"`
	PlanID        string  `json:"planId"`
	PlanName      *string `json:"planName,omitempty"`
	DurationDays  int     `json:"durationDays,omitempty"`
	QuotaImages   int     `json:"quotaImages,omitempty"`
	Weight        int     `json:"weight"`
	DailyStock    int     `json:"dailyStock"`
	TodayUsed     int     `json:"todayUsed"`
	MonthlyStock  int     `json:"monthlyStock"`
	MonthUsed     int     `json:"monthUsed"`
	RemainingText string  `json:"remainingText,omitempty"`
	MonthlyText   string  `json:"monthlyText,omitempty"`
	SortOrder     int     `json:"sortOrder"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"createdAt"`
	UpdatedAt     string  `json:"updatedAt"`
}

type LotteryRecord struct {
	ID           string  `json:"id"`
	UserID       string  `json:"userId"`
	UserEmail    *string `json:"userEmail,omitempty"`
	PrizeID      string  `json:"prizeId"`
	PrizeName    *string `json:"prizeName,omitempty"`
	PrizeType    string  `json:"prizeType"`
	PlanID       string  `json:"planId"`
	PlanName     *string `json:"planName,omitempty"`
	DurationDays int     `json:"durationDays,omitempty"`
	DrawDate     string  `json:"drawDate"`
	UserIP       *string `json:"userIp,omitempty"`
	CreatedAt    string  `json:"createdAt"`
}

type LotteryDrawResult struct {
	DrawnToday bool          `json:"drawnToday"`
	Record     LotteryRecord `json:"record"`
	Prize      LotteryPrize  `json:"prize"`
	Won        bool          `json:"won"`
	Message    string        `json:"message"`
}

type RechargeOrder struct {
	ID                 string                `json:"id"`
	UserID             string                `json:"userId"`
	UserEmail          *string               `json:"userEmail,omitempty"`
	OutTradeNo         string                `json:"outTradeNo"`
	TradeNo            *string               `json:"tradeNo"`
	OrderType          string                `json:"orderType"`
	SubscriptionPlanID *string               `json:"subscriptionPlanId"`
	Amount             float64               `json:"amount"`
	Credits            float64               `json:"-"`
	Status             string                `json:"status"`
	PayURL             *string               `json:"payUrl"`
	QRCode             *string               `json:"qrCode"`
	PaidAt             *string               `json:"paidAt"`
	CreatedAt          string                `json:"createdAt"`
	UpdatedAt          string                `json:"updatedAt"`
	InviteRebate       *InviteRechargeRebate `json:"-"`
}

type DashboardTaskSummary struct {
	ID               string  `json:"id"`
	UserID           string  `json:"userId"`
	UserEmail        *string `json:"userEmail,omitempty"`
	ModelID          string  `json:"modelId"`
	ModelName        *string `json:"modelName,omitempty"`
	ModelDisplayName *string `json:"modelDisplayName,omitempty"`
	Quantity         int     `json:"quantity"`
	CostCredits      float64 `json:"-"`
	Status           string  `json:"status"`
	CreatedAt        string  `json:"createdAt"`
}

type DashboardTaskTrendPoint struct {
	Date       string `json:"date"`
	Total      int    `json:"total"`
	Queued     int    `json:"queued"`
	Pending    int    `json:"pending"`
	Processing int    `json:"processing"`
	Running    int    `json:"running"`
	Success    int    `json:"success"`
	Failed     int    `json:"failed"`
	Canceled   int    `json:"canceled"`
}
