#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# FitCommerce — Test Runner
# Runs the full test suite (backend + frontend) inside Docker.
# Fails if combined coverage drops below 90%.
#
# Usage: ./run_tests.sh [--backend-only] [--frontend-only] [--lint] [--migrate] [--seed] [--reset]
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

RUN_BACKEND=true
RUN_FRONTEND=true
COVERAGE_THRESHOLD=90

while [[ $# -gt 0 ]]; do
  case "$1" in
    --backend-only)  RUN_FRONTEND=false; shift ;;
    --frontend-only) RUN_BACKEND=false; shift ;;
    --lint)
      echo "Running linter..."
      docker compose exec backend go vet ./...
      exit $? ;;
    --migrate)
      echo "Running migrations..."
      docker compose exec backend ./api migrate
      exit $? ;;
    --seed)
      echo "Running seeds (handled on startup)..."
      echo "Seeds run automatically when the backend starts."
      exit 0 ;;
    --reset)
      echo "Resetting database..."
      docker compose down -v
      docker compose up -d
      echo "Database reset. Services starting."
      exit 0 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

BACKEND_COVERAGE=0
FRONTEND_COVERAGE=0

# ─── Backend Tests ────────────────────────────────────────────────────────────
if [ "$RUN_BACKEND" = true ]; then
  echo ""
  echo "══════════════════════════════════════════════════════"
  echo "  Running backend tests"
  echo "══════════════════════════════════════════════════════"

  docker compose -f docker-compose.yml \
    -f docker-compose.test.yml \
    run --rm --build backend-test

  # Parse coverage from the output file written by the test container
  if [ -f ".coverage/backend.txt" ]; then
    BACKEND_COVERAGE=$(grep -oP '[0-9]+\.[0-9]+(?=%)' .coverage/backend.txt | tail -1 || echo "0")
    echo "Backend coverage: ${BACKEND_COVERAGE}%"
  fi
fi

# ─── Frontend Tests ───────────────────────────────────────────────────────────
if [ "$RUN_FRONTEND" = true ]; then
  echo ""
  echo "══════════════════════════════════════════════════════"
  echo "  Running frontend tests"
  echo "══════════════════════════════════════════════════════"

  docker compose -f docker-compose.yml \
    -f docker-compose.test.yml \
    run --rm --build frontend-test

  # Parse coverage from the output file written by the test container
  if [ -f ".coverage/frontend.txt" ]; then
    FRONTEND_COVERAGE=$(grep -oP 'All files\s+\|\s+\K[0-9]+\.[0-9]+' .coverage/frontend.txt | head -1 || echo "0")
    echo "Frontend coverage: ${FRONTEND_COVERAGE}%"
  fi
fi

# ─── Coverage Gate ────────────────────────────────────────────────────────────
echo ""
echo "══════════════════════════════════════════════════════"
echo "  Coverage Summary"
echo "══════════════════════════════════════════════════════"
echo "  Backend  : ${BACKEND_COVERAGE}%"
echo "  Frontend : ${FRONTEND_COVERAGE}%"
echo "  Threshold: ${COVERAGE_THRESHOLD}%"

FAILED=false

check_coverage() {
  local name="$1"
  local actual="$2"
  local threshold="$3"
  if awk "BEGIN {exit !($actual < $threshold)}"; then
    echo "  FAIL: $name coverage ${actual}% is below threshold ${threshold}%"
    FAILED=true
  else
    echo "  PASS: $name coverage ${actual}%"
  fi
}

if [ "$RUN_BACKEND" = true ]; then
  check_coverage "Backend" "$BACKEND_COVERAGE" "$COVERAGE_THRESHOLD"
fi
if [ "$RUN_FRONTEND" = true ]; then
  check_coverage "Frontend" "$FRONTEND_COVERAGE" "$COVERAGE_THRESHOLD"
fi

echo ""
if [ "$FAILED" = true ]; then
  echo "Tests FAILED — coverage below ${COVERAGE_THRESHOLD}%"
  exit 1
fi

echo "All tests PASSED"
exit 0
