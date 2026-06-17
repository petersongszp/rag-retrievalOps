#!/usr/bin/env sh
set -eu

IMAGE="${DOCLING_SERVE_IMAGE:-quay.io/docling-project/docling-serve-cpu:latest}"
NAME="${DOCLING_SERVE_CONTAINER:-docling-serve}"
PORT="${DOCLING_SERVE_PORT:-5001}"
ENABLE_UI="${DOCLING_SERVE_ENABLE_UI:-1}"

if docker ps --format '{{.Names}}' | grep -Fxq "$NAME"; then
  echo "Docling Serve is already running: $NAME"
elif docker ps -a --format '{{.Names}}' | grep -Fxq "$NAME"; then
  echo "Starting existing Docling Serve container: $NAME"
  docker start "$NAME" >/dev/null
else
  echo "Pulling $IMAGE"
  docker pull "$IMAGE"
  echo "Creating Docling Serve container: $NAME"
  docker run -d \
    --name "$NAME" \
    -p "$PORT:5001" \
    -e DOCLING_SERVE_ENABLE_UI="$ENABLE_UI" \
    "$IMAGE" >/dev/null
fi

echo "Docling Serve: http://localhost:$PORT"
echo "Docling UI:    http://localhost:$PORT/ui"
echo "Docling docs:  http://localhost:$PORT/docs"
echo
echo "Raw Docling endpoint is /v1/convert/file."
echo "The RAG backend should use the parser-provider adapter at /parse."
