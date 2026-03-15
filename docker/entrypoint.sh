#!/bin/sh
set -eu

wait_for_db() {
  if [ -z "${DATABASE_URL:-}" ]; then
    return 0
  fi
  until pg_isready -d "$DATABASE_URL" >/dev/null 2>&1; do
    sleep 1
  done
}

ensure_initialized() {
  /app/tenderhack init-db
}

maybe_import() {
  if [ "${AUTO_IMPORT:-1}" != "1" ]; then
    return 0
  fi
  count="$(psql "$DATABASE_URL" -Atc "select count(*) from cte_items;" 2>/dev/null || echo 0)"
  if [ "$count" = "0" ]; then
    /app/tenderhack import
  fi
}

main() {
  cmd="${1:-serve}"
  wait_for_db
  ensure_initialized

  if [ "$cmd" = "serve" ]; then
    maybe_import
  fi

  exec /app/tenderhack "$@"
}

main "$@"
