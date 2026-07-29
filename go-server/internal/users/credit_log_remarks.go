package users

import (
	"context"
	"sort"
	"strings"
)

type creditLogAdminReference struct {
	index  int
	detail string
}

func (r *Repository) resolveCreditLogAdminRemarks(ctx context.Context, items []CreditLog) {
	references := make(map[string][]creditLogAdminReference)
	for index := range items {
		if items[index].Type != "manual_adjust" {
			continue
		}
		actorID, detail, ok := splitLegacyAdminCreditRemark(items[index].Remark)
		if !ok {
			continue
		}

		// Replace first so a failed lookup never sends an internal identifier.
		items[index].Remark = formatAdminCreditRemark("系统管理员", detail)
		references[actorID] = append(references[actorID], creditLogAdminReference{index: index, detail: detail})
	}
	if len(references) == 0 || r == nil || r.db == nil {
		return
	}

	actorIDs := make([]string, 0, len(references))
	for actorID := range references {
		actorIDs = append(actorIDs, actorID)
	}
	sort.Strings(actorIDs)

	placeholders := make([]string, len(actorIDs))
	args := make([]any, len(actorIDs))
	for index, actorID := range actorIDs {
		placeholders[index] = "?"
		args[index] = actorID
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, email
		FROM users
		WHERE id IN (`+strings.Join(placeholders, ", ")+`)
	`, args...)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var actorID string
		var email string
		if err := rows.Scan(&actorID, &email); err != nil {
			continue
		}
		email = strings.TrimSpace(email)
		if email == "" || !strings.Contains(email, "@") {
			continue
		}
		for _, reference := range references[actorID] {
			items[reference.index].Remark = formatAdminCreditRemark("管理员 "+email, reference.detail)
		}
	}
}

func splitLegacyAdminCreditRemark(value string) (string, string, bool) {
	const prefix = "管理员 "
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, prefix) {
		return "", "", false
	}
	remainder := strings.TrimSpace(strings.TrimPrefix(value, prefix))
	actorID, detail, found := strings.Cut(remainder, "：")
	if !found {
		actorID, detail, found = strings.Cut(remainder, ":")
	}
	actorID = strings.TrimSpace(actorID)
	if !found || actorID == "" || strings.Contains(actorID, "@") {
		return "", "", false
	}
	return actorID, strings.TrimSpace(detail), true
}

func formatAdminCreditRemark(actor string, detail string) string {
	actor = strings.TrimSpace(actor)
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return actor
	}
	return actor + "：" + detail
}
