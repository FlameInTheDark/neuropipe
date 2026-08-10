package persistence

import squirrel "github.com/Masterminds/squirrel"

// sqliteStatements is the sole builder used for application data queries.
// SQLite accepts question-mark placeholders, so query construction remains
// independent of database/sql call sites and can be tested from the emitted
// statement and arguments.
var sqliteStatements = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question)

func statements(runner squirrel.BaseRunner) squirrel.StatementBuilderType {
	return sqliteStatements.RunWith(runner)
}
