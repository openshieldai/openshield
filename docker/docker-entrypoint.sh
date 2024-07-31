#!/bin/sh

# Exit immediately if a command exits with a non-zero status
set -e

# Print each command before executing it
set -x

# Check for DEMO_MODE and run appropriate commands
if [ -n "$DEMO_MODE" ]; then
  echo "Running in DEMO_MODE"
  ./openshield db create-tables
  ./openshield db create-mock-data
fi

# Run the command passed to the script using dumb-init
exec /usr/bin/dumb-init -- "$@"