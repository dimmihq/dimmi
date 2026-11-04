-- Creates the owner and application roles and hands the database to the
-- owner. Safe to run any number of times: it creates what is missing and
-- sets every password and grant again.
--
-- psql runs it as the RDS master user with these variables:
--   owner_role, app_role, database  role and database names
--   owner_secret, app_secret        their passwords
--
-- The application role owns nothing and cannot bypass row-level security.
-- Default privileges make every table the owner creates readable and
-- writable by it, and MAINTAIN lets the queue's leader reindex its tables.
-- Grants are never repeated on existing tables: a migration that takes a
-- privilege away, such as updating audit entries, must stay in force.

\set ON_ERROR_STOP on

SELECT format('CREATE ROLE %I LOGIN', :'owner_role')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = :'owner_role') \gexec

SELECT format('CREATE ROLE %I LOGIN', :'app_role')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = :'app_role') \gexec

SELECT format('ALTER ROLE %I WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS ENCRYPTED PASSWORD %L', :'owner_role', :'owner_secret') \gexec
SELECT format('ALTER ROLE %I WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS ENCRYPTED PASSWORD %L', :'app_role', :'app_secret') \gexec

SELECT format('GRANT %I TO CURRENT_USER', :'owner_role') \gexec
SELECT format('ALTER DATABASE %I OWNER TO %I', :'database', :'owner_role') \gexec
SELECT format('REVOKE ALL ON DATABASE %I FROM PUBLIC', :'database') \gexec
SELECT format('GRANT CONNECT, TEMPORARY ON DATABASE %I TO %I', :'database', :'app_role') \gexec

SELECT format('ALTER SCHEMA public OWNER TO %I', :'owner_role') \gexec
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
SELECT format('GRANT USAGE ON SCHEMA public TO %I', :'app_role') \gexec

SELECT format('ALTER DEFAULT PRIVILEGES FOR ROLE %I IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE, MAINTAIN ON TABLES TO %I', :'owner_role', :'app_role') \gexec
SELECT format('ALTER DEFAULT PRIVILEGES FOR ROLE %I IN SCHEMA public GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO %I', :'owner_role', :'app_role') \gexec
SELECT format('ALTER DEFAULT PRIVILEGES FOR ROLE %I IN SCHEMA public GRANT EXECUTE ON FUNCTIONS TO %I', :'owner_role', :'app_role') \gexec
