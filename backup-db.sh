#!/bin/bash
source /var/www/dawhub/.env
docker exec dawhub_db_1 pg_dump -U "$DB_USER" "$DB_NAME" > /var/www/dawhub/backups/db-$(date +%Y%m%d).sql