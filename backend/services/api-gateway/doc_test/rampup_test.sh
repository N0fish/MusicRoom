#!/bin/bash

# Music Room - Ramp-up Automation Script
# This script automates Apache Benchmark (ab) runs to measure server capacity.

# Configuration
GATEWAY_URL=${GATEWAY_URL:-"http://localhost:8080"}
CONCURRENCY=${1:-50}
REQUESTS=${2:-500}

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}==============================================${NC}"
echo -e "${BLUE}   Music Room - Ramp-up Automation Script     ${NC}"
echo -e "${BLUE}==============================================${NC}"
echo -e "Target: $GATEWAY_URL"
echo -e "Concurrency: $CONCURRENCY"
echo -e "Total Requests: $REQUESTS"
echo ""

# Function to run ab and extract key metrics
run_test() {
    local name=$1
    local cmd=$2
    
    echo -e "${BLUE}[TEST] Running $name...${NC}"
    
    # Run ab and capture output
    output=$(eval "$cmd" 2>&1)
    
    if [[ $? -ne 0 ]]; then
        echo -e "${RED}Error running test for $name${NC}"
        echo "$output"
        return 1
    fi
    
    # Extract metrics
    rps=$(echo "$output" | grep "Requests per second" | awk '{print $4}')
    latency=$(echo "$output" | grep "Time per request:" | head -n 1 | awk '{print $4}')
    p95=$(echo "$output" | grep -A 10 "Percentage of the requests served within a certain time" | grep "95%" | awk '{print $2}')
    failed=$(echo "$output" | grep "Failed requests" | awk '{print $3}')
    
    echo -e "${GREEN}  RPS: $rps req/s${NC}"
    echo -e "${GREEN}  Avg Latency: $latency ms${NC}"
    echo -e "${GREEN}  95th Percentile: $p95 ms${NC}"
    if [[ "$failed" != "0" && ! -z "$failed" ]]; then
        echo -e "${RED}  Failed Requests: $failed${NC}"
    fi
    echo ""
}

# 1. Healthcheck (Baseline)
run_test "Healthcheck (Baseline)" "ab -n $REQUESTS -c $CONCURRENCY $GATEWAY_URL/health"

# 2. Login Load
if [[ -f "login.json" ]]; then
    run_test "Login Load" "ab -n $REQUESTS -c $CONCURRENCY -T 'application/json' -p login.json $GATEWAY_URL/auth/login"
else
    echo -e "${RED}login.json not found, skipping.${NC}"
fi

# 3. Playlist Browsing
run_test "Playlist Browsing" "ab -n $REQUESTS -c $CONCURRENCY $GATEWAY_URL/playlists"

# 4. Event Voting
if [[ -f "vote.json" ]]; then
    # Note: We use a dummy ID here as it's for performance measurement of the path
    run_test "Event Voting" "ab -n $REQUESTS -c $CONCURRENCY -T 'application/json' -p vote.json $GATEWAY_URL/events/test-event-id/vote"
else
    echo -e "${RED}vote.json not found, skipping.${NC}"
fi

echo -e "${BLUE}==============================================${NC}"
echo -e "${BLUE}              Ramp-up Completed               ${NC}"
echo -e "${BLUE}==============================================${NC}"
