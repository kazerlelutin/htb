ALTER TABLE client_requests DROP CONSTRAINT client_requests_status_check;
ALTER TABLE client_requests ADD CONSTRAINT client_requests_status_check CHECK (status IN ('received', 'in_progress', 'needs_info', 'done', 'rejected'));
