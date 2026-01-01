#!/bin/bash
# HumanMark API Test Script
# Run this after starting the server to test all endpoints

set -e

BASE_URL="${1:-http://localhost:8080}"
PASS=0
FAIL=0

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo ""
echo "=========================================="
echo -e "${BLUE}  HumanMark API Test Suite${NC}"
echo "=========================================="
echo "Testing: $BASE_URL"
echo ""

# Function to test endpoint
test_endpoint() {
    local name="$1"
    local method="$2"
    local endpoint="$3"
    local data="$4"
    local expected_status="$5"
    local check_field="$6"
    
    echo -n "Testing: $name... "
    
    if [ "$method" == "GET" ]; then
        response=$(curl -s -w "\n%{http_code}" "$BASE_URL$endpoint" 2>/dev/null)
    else
        response=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL$endpoint" \
            -H "Content-Type: application/json" \
            -d "$data" 2>/dev/null)
    fi
    
    status=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')
    
    if [ "$status" == "$expected_status" ]; then
        if [ -n "$check_field" ]; then
            if echo "$body" | grep -q "$check_field"; then
                echo -e "${GREEN}✓ PASS${NC} (status: $status)"
                ((PASS++))
            else
                echo -e "${RED}✗ FAIL${NC} (missing: $check_field)"
                echo "  Response: $body"
                ((FAIL++))
            fi
        else
            echo -e "${GREEN}✓ PASS${NC} (status: $status)"
            ((PASS++))
        fi
    else
        echo -e "${RED}✗ FAIL${NC} (expected: $expected_status, got: $status)"
        echo "  Response: $body"
        ((FAIL++))
    fi
}

echo "----------------------------------------"
echo -e "${YELLOW}1. Basic Endpoints${NC}"
echo "----------------------------------------"

# Health check
test_endpoint "Health Check" "GET" "/health" "" "200" "healthy"

# Index
test_endpoint "Index Page" "GET" "/" "" "200" "HumanMark"

echo ""
echo "----------------------------------------"
echo -e "${YELLOW}2. Text Detection${NC}"
echo "----------------------------------------"

# Human-like text
test_endpoint "Human Text (casual)" "POST" "/verify" \
    '{"text": "Hey! So I was thinking about this yesterday... you know what really bugs me? When people dont use their turn signals lol. Anyway, what are you up to this weekend?"}' \
    "200" "human"

# AI-like text  
test_endpoint "AI Text (formal)" "POST" "/verify" \
    '{"text": "As an AI language model, I cannot provide personal opinions. However, it is important to note that this topic has many facets. Furthermore, we should consider multiple perspectives. In conclusion, I hope this helps."}' \
    "200" "human"

# Mixed text
test_endpoint "Mixed Text" "POST" "/verify" \
    '{"text": "The quarterly results exceeded expectations by 15%. Revenue grew to $4.2M while maintaining healthy margins. The team successfully launched three new features."}' \
    "200" "human"

# Short text
test_endpoint "Short Text" "POST" "/verify" \
    '{"text": "Hello world, this is a test message."}' \
    "200" "human"

# Long text
LONG_TEXT="This is a longer piece of text that contains multiple paragraphs and varied sentence structures. Some sentences are short. Others are considerably longer and contain more complex vocabulary and grammatical structures that require more careful analysis. The goal here is to test how the analyzer handles longer content with natural variation in style and tone. I personally think this kind of writing feels more human because it has personality and quirks. You know what I mean? Anyway, lets see how it scores."
test_endpoint "Long Text" "POST" "/verify" \
    "{\"text\": \"$LONG_TEXT\"}" \
    "200" "human"

echo ""
echo "----------------------------------------"
echo -e "${YELLOW}3. URL Detection${NC}"
echo "----------------------------------------"

# Image URL
test_endpoint "Image URL (jpg)" "POST" "/verify" \
    '{"url": "https://example.com/photo.jpg"}' \
    "200" "content_type"

# Text URL
test_endpoint "Text URL" "POST" "/verify" \
    '{"url": "https://example.com/article.txt"}' \
    "200" "content_type"

echo ""
echo "----------------------------------------"
echo -e "${YELLOW}4. Detailed Response${NC}"
echo "----------------------------------------"

# Detailed response
test_endpoint "Detailed Mode" "POST" "/verify?detailed=true" \
    '{"text": "This is a test for detailed response mode with extra analysis information."}' \
    "200" "details"

echo ""
echo "----------------------------------------"
echo -e "${YELLOW}5. Error Handling${NC}"
echo "----------------------------------------"

# Empty request
test_endpoint "Empty Request" "POST" "/verify" \
    '{}' \
    "400" "error"

# Invalid JSON
test_endpoint "Invalid JSON" "POST" "/verify" \
    '{invalid}' \
    "400" "error"

# Text too short
test_endpoint "Text Too Short" "POST" "/verify" \
    '{"text": "Hi"}' \
    "400" "error"

# Invalid URL
test_endpoint "Invalid URL" "POST" "/verify" \
    '{"url": "not-a-valid-url"}' \
    "400" "error"

echo ""
echo "----------------------------------------"
echo -e "${YELLOW}6. Result Retrieval${NC}"
echo "----------------------------------------"

# First create a job and get its ID
echo -n "Creating job for retrieval test... "
CREATE_RESPONSE=$(curl -s -X POST "$BASE_URL/verify" \
    -H "Content-Type: application/json" \
    -d '{"text": "This is a test to create a job for retrieval testing purposes."}')
JOB_ID=$(echo "$CREATE_RESPONSE" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

if [ -n "$JOB_ID" ]; then
    echo -e "${GREEN}✓${NC} (id: $JOB_ID)"
    
    # Retrieve the job
    test_endpoint "Get Result by ID" "GET" "/verify/$JOB_ID" "" "200" "$JOB_ID"
else
    echo -e "${RED}✗ Failed to create job${NC}"
    ((FAIL++))
fi

# Non-existent ID
test_endpoint "Non-existent ID" "GET" "/verify/non-existent-id-12345" "" "404" "not_found"

echo ""
echo "=========================================="
echo -e "${BLUE}  Test Results${NC}"
echo "=========================================="
echo -e "Passed: ${GREEN}$PASS${NC}"
echo -e "Failed: ${RED}$FAIL${NC}"
echo ""

if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}All tests passed! ✓${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed. Check output above.${NC}"
    exit 1
fi
