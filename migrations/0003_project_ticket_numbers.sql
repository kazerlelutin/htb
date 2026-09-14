ALTER TABLE projects ADD COLUMN next_ticket_number BIGINT NOT NULL DEFAULT 1;

ALTER TABLE tickets ADD COLUMN number BIGINT;

WITH numbered AS (
  SELECT id, row_number() OVER (PARTITION BY project_id ORDER BY id) AS number
  FROM tickets
)
UPDATE tickets AS ticket
SET number = numbered.number
FROM numbered
WHERE ticket.id = numbered.id;

ALTER TABLE tickets ALTER COLUMN number SET NOT NULL;
ALTER TABLE tickets ADD CONSTRAINT tickets_project_number_key UNIQUE (project_id, number);

UPDATE projects AS project
SET next_ticket_number = COALESCE(
  (SELECT MAX(ticket.number) + 1 FROM tickets AS ticket WHERE ticket.project_id = project.id),
  1
);
