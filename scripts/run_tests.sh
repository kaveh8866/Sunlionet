#!/bin/bash
set -e

# ShadowNet Agent - Test Runner
# This script runs all unit tests and integration tests for the project.

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}====================================================${NC}"
echo -e "${GREEN}       ShadowNet Agent: Full Test Suite             ${NC}"
echo -e "${GREEN}====================================================${NC}"

# Check for Go installation
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed or not in PATH.${NC}"
    echo "Please install Go 1.21+ to run the tests."
    exit 1
fi

echo -e "\n${YELLOW}Running Go Unit Tests with Race Detector...${NC}"
# Run all tests recursively with verbose output and data race detection
go test -v -race ./...

if [ $? -eq 0 ]; then
    echo -e "\n${GREEN}✓ All unit tests passed successfully!${NC}"
else
    echo -e "\n${RED}✗ Some unit tests failed. Check the output above.${NC}"
    exit 1
fi

echo -e "\n${YELLOW}Running Go Linter (go vet)...${NC}"
go vet ./...
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ No suspicious constructs found.${NC}"
fi

echo -e "\n${YELLOW}Checking code formatting...${NC}"
UNFORMATTED=$(gofmt -l .)
if [ -z "$UNFORMATTED" ]; then
    echo -e "${GREEN}✓ All files are properly formatted.${NC}"
else
    echo -e "${RED}✗ The following files are not formatted correctly:${NC}"
    echo "$UNFORMATTED"
    echo "Run 'gofmt -w .' to fix them."
fi

echo -e "\n${GREEN}====================================================${NC}"
echo -e "${GREEN}  Test Suite Completed Successfully!                ${NC}"
echo -e "${GREEN}====================================================${NC}"
