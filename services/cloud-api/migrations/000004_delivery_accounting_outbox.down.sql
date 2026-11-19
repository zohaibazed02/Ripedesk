-- 000004_delivery_accounting_outbox.down.sql

DROP TABLE IF EXISTS sync_review_queue CASCADE;
DROP TABLE IF EXISTS processed_commands CASCADE;
DROP TABLE IF EXISTS outbox_events CASCADE;
DROP TABLE IF EXISTS journal_lines CASCADE;
DROP TABLE IF EXISTS journal_entries CASCADE;
DROP TABLE IF EXISTS chart_of_accounts CASCADE;
DROP FUNCTION IF EXISTS enforce_balanced_journal_entry() CASCADE;
DROP TABLE IF EXISTS attendance_logs CASCADE;
DROP TABLE IF EXISTS staff_rosters CASCADE;
DROP TABLE IF EXISTS rider_gps_pings CASCADE;
DROP TABLE IF EXISTS delivery_trips CASCADE;
