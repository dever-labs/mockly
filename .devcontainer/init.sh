#!/usr/bin/env bash
set -euo pipefail

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${CYAN}==> Installing Go tools...${NC}"
GOTOOLCHAIN=auto go install github.com/air-verse/air@v1.61.7
GOTOOLCHAIN=auto go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

echo -e "${CYAN}==> Installing UI dependencies...${NC}"
npm --prefix ui ci

echo -e ""
echo -e "${GREEN}✔ Devcontainer ready!${NC}"
echo -e ""
echo -e "Quick-start commands:"
echo -e "  ${YELLOW}air${NC}                   — run the server with hot reload"
echo -e "  ${YELLOW}go test ./internal/... -race${NC}   — run unit/integration tests"
echo -e "  ${YELLOW}golangci-lint run${NC}     — lint the code"
echo -e "  ${YELLOW}npm --prefix ui run dev${NC}        — run the UI in dev mode"
echo -e ""
echo -e "Ports:"
echo -e "  HTTP Mocks       → http://localhost:8080"
echo -e "  WebSocket Mocks  → ws://localhost:8081"
echo -e "  Management API + UI → http://localhost:9091"
echo -e "  gRPC Mocks       → localhost:50051"
echo -e ""

if ls ~/.copilot/*.json &>/dev/null 2>&1; then
  echo -e "${GREEN}✔ GitHub Copilot CLI authenticated.${NC}"
else
  echo -e "Run ${YELLOW}copilot login${NC} to authenticate GitHub Copilot CLI."
  echo -e "Credentials persist in a Docker volume — no re-login needed after rebuild."
fi
