#!/bin/bash
set -e

echo "Starting script"

# Restore the database if it does not already exist.
if [ -f /data/data.db ]; then
	echo "Database already exists, skipping restore"
else
	echo "No database found, restoring from replica if exists"
	litestream restore -if-replica-exists /data/data.db
fi

# Run litestream with your app as the subprocess.
exec litestream replicate -exec "/space"
