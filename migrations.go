package htb

import _ "embed"

// InitialMigration is embedded so an HTB server can migrate an empty database
// without relying on files present in its runtime container.
//
//go:embed migrations/0001_initial.sql
var InitialMigration string

//go:embed migrations/0002_feature_due_date.sql
var FeatureDueDateMigration string
