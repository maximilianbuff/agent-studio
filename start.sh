#!/usr/bin/env bash
set -e
ROOT="$(cd "$(dirname "$0")" && pwd)"

echo "Starting Agent Studio..."

# Backend
cd "$ROOT/backend"
go build -o agent-studio-server . &
wait
./agent-studio-server &
BACKEND_PID=$!
echo "Backend running (PID $BACKEND_PID) on :8000"

# Frontend
cd "$ROOT/frontend"
if [ ! -d node_modules ]; then
  echo "Installing frontend deps..."
  npm install
fi
npm run dev &
FRONTEND_PID=$!
echo "Frontend running (PID $FRONTEND_PID) on :5173"

trap "kill $BACKEND_PID $FRONTEND_PID 2>/dev/null" EXIT
wait
