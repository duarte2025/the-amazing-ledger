package postgres

import (
	"context"

	"github.com/jackc/pgx/v4"

	"github.com/stone-co/the-amazing-ledger/app/domain/vos"
	"github.com/stone-co/the-amazing-ledger/app/shared/instrumentation/newrelic"
)

func (r *LedgerRepository) GetBatchAccountBalance(ctx context.Context, accountsName []vos.AccountName) ([]vos.AccountBalance, error) {
	operation := "Repository.GetBatchAccountBalanceAggregated"
	query := `
		SELECT
			account_class,
			account_group,
			account_subgroup,
			account_id,
			MAX(version) as current_version,
			SUM(CASE operation
				WHEN $1 THEN amount
				ELSE 0
				END) AS total_credit,
			SUM(CASE operation
				WHEN $2 THEN amount
				ELSE 0
				END) AS total_debit
		FROM entries
		WHERE account_class = $3 AND account_group = $4 AND account_subgroup = $5 AND account_id = $6
		GROUP BY account_class, account_group, account_subgroup, account_id
	`

	defer newrelic.NewDatastoreSegment(ctx, collection, operation, query).End()

	creditOperation := vos.CreditOperation.String()
	debitOperation := vos.DebitOperation.String()

	var batch pgx.Batch
	for _, accountName := range accountsName {
		batch.Queue(
			query,
			query,
			creditOperation,
			debitOperation,
			accountName.Class.String(),
			accountName.Group,
			accountName.Subgroup,
			accountName.ID,
		)
	}

	br := r.db.SendBatch(ctx, &batch)
	rows, err := br.Query()
	if err != nil {
		return nil, err
	}

	var currentVersion uint64
	var totalCredit int
	var totalDebit int

	var accountsBalance []vos.AccountBalance
	for  i := 0; rows.Next(); i++ {
		if err := rows.Scan(
			nil,
			nil,
			nil,
			nil,
			&currentVersion,
			&totalCredit,
			&totalDebit,
		); err != nil {
			return nil, err
		}
		accountBalance := *vos.NewAccountBalance(accountsName[i], vos.Version(currentVersion), totalCredit, totalDebit)
		accountsBalance = append(accountsBalance, accountBalance)
	}

	return accountsBalance, nil

}