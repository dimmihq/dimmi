#!/bin/sh
# Runs once, when the database volume is first created. The database belongs
# to dimmi_owner, which runs migrations. The app connects as dimmi_app, which
# owns nothing and cannot bypass row-level security. pgvector needs a
# superuser, so it is installed here for the owner's migrations to find.
set -eu
psql -v ON_ERROR_STOP=1 --username postgres --dbname postgres <<SQL
CREATE ROLE dimmi_owner LOGIN PASSWORD '$POSTGRES_OWNER_PASSWORD';
CREATE ROLE dimmi_app LOGIN PASSWORD '$POSTGRES_APP_PASSWORD';
CREATE DATABASE dimmi OWNER dimmi_owner;
SQL
psql -v ON_ERROR_STOP=1 --username postgres --dbname dimmi <<SQL
CREATE EXTENSION IF NOT EXISTS vector;
ALTER SCHEMA public OWNER TO dimmi_owner;
REVOKE ALL ON DATABASE dimmi FROM PUBLIC;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT CONNECT, TEMPORARY ON DATABASE dimmi TO dimmi_app;
GRANT USAGE ON SCHEMA public TO dimmi_app;
ALTER DEFAULT PRIVILEGES FOR ROLE dimmi_owner IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE, MAINTAIN ON TABLES TO dimmi_app;
ALTER DEFAULT PRIVILEGES FOR ROLE dimmi_owner IN SCHEMA public
  GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO dimmi_app;
ALTER DEFAULT PRIVILEGES FOR ROLE dimmi_owner IN SCHEMA public
  GRANT EXECUTE ON FUNCTIONS TO dimmi_app;
SQL
