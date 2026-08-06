package content

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestClaimAnnouncementRewardGrantsOnlyOnce(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	db := database.Wrap(rawDB)
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT title, COALESCE\(reward_credits, 0\), target_type, status`).
		WithArgs("announcement-1").
		WillReturnRows(sqlmock.NewRows([]string{"title", "reward_credits", "target_type", "status"}).AddRow("维护补偿", 2.5, "all", "active"))
	mock.ExpectQuery(`SELECT reward_claimed_at`).
		WithArgs("announcement-1", "user-1").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT credits`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"credits"}).AddRow(10.0))
	mock.ExpectExec(`UPDATE users`).
		WithArgs(12.5, "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO credit_logs`).
		WithArgs(sqlmock.AnyArg(), "user-1", 2.5, 12.5, "公告奖励：维护补偿").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO announcement_receipts`).
		WithArgs("announcement-1", "user-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := NewRepository(db).ClaimAnnouncementReward(context.Background(), "announcement-1", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Granted || result.RewardCredits != 2.5 || result.BalanceAfter != 12.5 {
		t.Fatalf("unexpected first claim: %+v", result)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT title, COALESCE\(reward_credits, 0\), target_type, status`).
		WithArgs("announcement-1").
		WillReturnRows(sqlmock.NewRows([]string{"title", "reward_credits", "target_type", "status"}).AddRow("维护补偿", 2.5, "all", "active"))
	mock.ExpectQuery(`SELECT reward_claimed_at`).
		WithArgs("announcement-1", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"reward_claimed_at"}).AddRow(now))
	mock.ExpectCommit()

	result, err = NewRepository(db).ClaimAnnouncementReward(context.Background(), "announcement-1", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Granted || result.RewardCredits != 2.5 {
		t.Fatalf("unexpected duplicate claim: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
