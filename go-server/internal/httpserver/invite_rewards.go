package httpserver

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"aipi-go/internal/operations"
	"aipi-go/internal/settings"
	"aipi-go/internal/users"
)

type inviteProgramConfig struct {
	Enabled       bool
	InviterReward operations.InviteRewardSpec
	InviteeReward operations.InviteRewardSpec
	Risk          operations.InviteRiskLimits
}

func inviteProgramConfigFromSettings(values settings.Settings) inviteProgramConfig {
	inviterPlanID := strings.TrimSpace(anyString(values["inviteInviterRewardPlanId"]))
	if inviterPlanID == "" {
		inviterPlanID = strings.TrimSpace(anyString(values["inviteRewardPlanId"]))
	}
	inviterType := strings.ToLower(strings.TrimSpace(anyString(values["inviteInviterRewardType"])))
	if inviterType == "" {
		inviterType = strings.ToLower(strings.TrimSpace(anyString(values["inviteRewardType"])))
	}
	return inviteProgramConfig{
		Enabled: anyBool(values["inviteEnabled"]),
		InviterReward: operations.InviteRewardSpec{
			Type:    inviterType,
			Credits: anyFloat(values["inviteInviterRewardCredits"], 0),
			PlanID:  inviterPlanID,
		},
		InviteeReward: operations.InviteRewardSpec{
			Type:    strings.ToLower(strings.TrimSpace(anyString(values["inviteInviteeRewardType"]))),
			Credits: anyFloat(values["inviteInviteeRewardCredits"], 0),
			PlanID:  strings.TrimSpace(anyString(values["inviteInviteeRewardPlanId"])),
		},
		Risk: operations.InviteRiskLimits{
			Enabled:          anyBool(values["inviteRiskEnabled"]),
			BlockSameIP:      anyBool(values["inviteRiskBlockSameIP"]),
			BlockSameDevice:  anyBool(values["inviteRiskBlockSameDevice"]),
			MaxPerIP24h:      positiveSettingInt(values["inviteRiskMaxPerIP24h"], 2),
			MaxPerDevice24h:  positiveSettingInt(values["inviteRiskMaxPerDevice24h"], 1),
			MaxPerInviter24h: positiveSettingInt(values["inviteRiskMaxPerInviter24h"], 10),
		},
	}
}

func inviteRechargeRebateConfigFromSettings(values settings.Settings) operations.InviteRechargeRebateConfig {
	return operations.InviteRechargeRebateConfig{
		Enabled:              anyBool(values["inviteRechargeRebateEnabled"]),
		Percent:              anyFloat(values["inviteRechargeRebatePercent"], 5),
		RechargeRate:         anyFloat(values["rechargeRate"], 10),
		IncludeSubscriptions: anyBool(values["inviteRebateIncludeSubscriptions"]),
	}
}

func (r *Router) finalizeInviteRewards(ctx context.Context, userID string) (*operations.InviteRewardResult, error) {
	values, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		return nil, err
	}
	result, err := operations.NewRepository(r.db).FinalizeInviteRewards(ctx, userID, inviteProgramConfigFromSettings(values).Risk)
	if err != nil || result == nil {
		return result, err
	}
	if result.Status == "rewarded" {
		r.publishCurrentUser(context.Background(), result.InviterID)
		r.publishCurrentUser(context.Background(), result.InviteeID)
	}
	return result, nil
}

func (r *Router) settleInviteRewards(ctx context.Context, user *users.User) *users.User {
	if user == nil || user.EmailVerifiedAt == nil || strings.TrimSpace(user.InvitedBy) == "" {
		return user
	}
	if _, err := r.finalizeInviteRewards(ctx, user.ID); err != nil {
		if r.logger != nil {
			r.logger.Warn("invite reward settlement failed", "userId", user.ID, "error", err)
		}
		return user
	}
	updated, err := users.NewRepository(r.db).FindByID(ctx, user.ID)
	if err == nil && updated != nil {
		return updated
	}
	return user
}

func inviteRewardDescription(spec operations.InviteRewardSpec, planName string) string {
	switch strings.ToLower(strings.TrimSpace(spec.Type)) {
	case "balance":
		return strconv.FormatFloat(spec.Credits, 'f', -1, 64) + " 余额"
	case "subscription":
		if strings.TrimSpace(planName) != "" {
			return planName
		}
		return "订阅权益"
	default:
		return "暂未配置"
	}
}

func inviteRewardSpecConfigured(spec operations.InviteRewardSpec) bool {
	switch strings.ToLower(strings.TrimSpace(spec.Type)) {
	case "balance":
		return spec.Credits > 0
	case "subscription":
		return strings.TrimSpace(spec.PlanID) != ""
	default:
		return false
	}
}

func inviteRewardLabel(prefix string, spec operations.InviteRewardSpec, planName string) string {
	return fmt.Sprintf("%s：%s", prefix, inviteRewardDescription(spec, planName))
}
