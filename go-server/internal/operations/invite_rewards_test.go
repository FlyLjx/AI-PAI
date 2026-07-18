package operations

import (
	"context"
	"testing"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestFinalizeInviteRewardsGrantsBothBalancesOnce(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectBegin()
	expectPendingBalanceInvite(mock, "pending")
	mock.ExpectQuery(`SELECT id FROM users`).WithArgs("invitee-1").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("invitee-1"))
	mock.ExpectQuery(`SELECT credits FROM users`).WithArgs("inviter-1").WillReturnRows(sqlmock.NewRows([]string{"credits"}).AddRow(20.0))
	mock.ExpectExec(`UPDATE users SET credits=\?, updated_at=CURRENT_TIMESTAMP WHERE id=\?`).WithArgs(30.0, "inviter-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO credit_logs`).WithArgs(sqlmock.AnyArg(), "inviter-1", 10.0, 30.0, "邀请奖励：被邀请人已完成邮箱验证").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`SELECT credits FROM users`).WithArgs("invitee-1").WillReturnRows(sqlmock.NewRows([]string{"credits"}).AddRow(2.0))
	mock.ExpectExec(`UPDATE users SET credits=\?, updated_at=CURRENT_TIMESTAMP WHERE id=\?`).WithArgs(7.0, "invitee-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO credit_logs`).WithArgs(sqlmock.AnyArg(), "invitee-1", 5.0, 7.0, "新人奖励：已完成邀请注册和邮箱验证").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE user_invites`).WithArgs("invite-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := NewRepository(database.Wrap(rawDB))
	result, err := repo.FinalizeInviteRewards(context.Background(), "invitee-1", InviteRiskLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Status != "rewarded" {
		t.Fatalf("result = %#v, want rewarded", result)
	}

	mock.ExpectBegin()
	expectPendingBalanceInvite(mock, "rewarded")
	mock.ExpectCommit()
	result, err = repo.FinalizeInviteRewards(context.Background(), "invitee-1", InviteRiskLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Status != "rewarded" {
		t.Fatalf("second result = %#v, want rewarded", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFinalizeInviteRewardsBlocksSameIP(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectBegin()
	expectPendingBalanceInvite(mock, "pending")
	mock.ExpectQuery(`SELECT id FROM users`).WithArgs("invitee-1").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("invitee-1"))
	mock.ExpectQuery(`SELECT COUNT\(\*\)`).WithArgs("inviter-1", "ip-hash", "203.0.113.8").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(`UPDATE user_invites SET status='blocked'`).WithArgs("邀请人与被邀请人使用相同网络地址", "invite-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := NewRepository(database.Wrap(rawDB)).FinalizeInviteRewards(context.Background(), "invitee-1", InviteRiskLimits{
		Enabled: true, BlockSameIP: true, MaxPerIP24h: 2, MaxPerDevice24h: 1, MaxPerInviter24h: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Status != "blocked" || result.RiskReason == "" {
		t.Fatalf("result = %#v, want blocked with reason", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func expectPendingBalanceInvite(mock sqlmock.Sqlmock, status string) {
	mock.ExpectQuery(`SELECT id, inviter_id, invitee_id`).WithArgs("invitee-1").WillReturnRows(sqlmock.NewRows([]string{
		"id", "inviter_id", "invitee_id", "status",
		"reward_type", "reward_credits", "reward_plan_id", "reward_plan_snapshot",
		"invitee_reward_type", "invitee_reward_credits", "invitee_reward_plan_id", "invitee_reward_plan_snapshot",
		"invitee_ip", "device_hash", "ip_hash",
	}).AddRow(
		"invite-1", "inviter-1", "invitee-1", status,
		"balance", 10.0, nil, nil,
		"balance", 5.0, nil, nil,
		"203.0.113.8", "device-hash", "ip-hash",
	))
}
