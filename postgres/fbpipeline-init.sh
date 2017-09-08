#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
    CREATE USER fbpipeline PASSWORD 'fbpipeline';
    CREATE DATABASE fbpipeline;
EOSQL

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" -d fbpipeline -f /fbpipeline-schema.sql

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
    GRANT ALL PRIVILEGES ON DATABASE fbpipeline TO fbpipeline;
EOSQL
