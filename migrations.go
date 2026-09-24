package htb

import _ "embed"

// InitialMigration is embedded so an HTB server can migrate an empty database
// without relying on files present in its runtime container.
//
//go:embed migrations/0001_initial.sql
var InitialMigration string

//go:embed migrations/0002_feature_due_date.sql
var FeatureDueDateMigration string

//go:embed migrations/0003_project_ticket_numbers.sql
var ProjectTicketNumbersMigration string

//go:embed migrations/0004_account_project_limits.sql
var AccountProjectLimitsMigration string

//go:embed migrations/0005_web_sessions.sql
var WebSessionsMigration string

//go:embed migrations/0006_client_requests.sql
var ClientRequestsMigration string
