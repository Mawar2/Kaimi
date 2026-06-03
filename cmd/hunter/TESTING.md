# Hunter Agent Testing Guide

## Overview

The Hunter agent has two modes of operation:
- **Cached mode**: Uses pre-recorded fixture data for fast, deterministic testing (runs in CI)
- **Live mode**: Makes real API calls to SAM.gov (manual testing only, not in CI)

## Automated Tests (CI)

All automated tests run in cached mode and are executed as part of the CI pipeline:

```bash
# Run all tests
go test ./...

# Run only Hunter tests
go test ./cmd/hunter/... -v

# Run only SAM.gov client tests
go test ./internal/samgov/... -v

# Run only Store tests
go test ./internal/store/... -v
```

### Test Coverage

1. **Unit Tests**
   - SAM.gov client configuration and validation
   - Opportunity data transformation
   - NAICS code parsing
   - Configuration validation
   - Store operations (Save, Get, List, Delete)

2. **Contract Tests**
   - SAM.gov client interface compliance
   - Store interface compliance
   - Opportunity schema serialization

3. **Integration Tests**
   - End-to-end Hunter workflow in cached mode
   - Fetching opportunities from fixtures
   - Saving to JSON store
   - Retrieving saved opportunities

## Manual Testing - Cached Mode

Test the Hunter agent locally using cached fixtures:

```bash
# Run with default settings (cached mode, NAICS 541512,541519)
go run cmd/hunter/main.go --mode=cached

# Run with custom NAICS codes
go run cmd/hunter/main.go --mode=cached --naics=541512,541519,541330

# Run with custom store path
go run cmd/hunter/main.go --mode=cached --store-path=/tmp/test-queue
```

**Expected Results:**
- Hunter should fetch 3 opportunities from cached fixtures
- All 3 opportunities should be saved to `./queue/queue/` directory
- Each opportunity saved as `{notice-id}.json`
- No errors should be reported

**Verification:**
```bash
# Check saved opportunities
ls ./queue/queue/
# Should show: a1b2c3d4e5f6.json, f6e5d4c3b2a1.json, 9z8y7x6w5v4u.json

# Inspect an opportunity
cat ./queue/queue/a1b2c3d4e5f6.json | jq .
```

## Manual Testing - Live SAM.gov API

**IMPORTANT:** Live API testing requires a valid SAM.gov API key and should NOT be run in CI.

### Prerequisites

1. **Obtain SAM.gov API Key**
   - Register at https://open.gsa.gov/api/
   - Request access to the Opportunities API
   - Save your API key securely

2. **Set Environment Variable**
   ```bash
   export SAM_API_KEY="your-api-key-here"
   ```

### Live API Test Plan

#### Test 1: Basic Fetch with Single NAICS Code

```bash
go run cmd/hunter/main.go \
  --mode=live \
  --naics=541512 \
  --store-path=./test-live-queue
```

**Expected Results:**
- Hunter connects to live SAM.gov API
- Fetches all active opportunities with NAICS code 541512
- Number of opportunities varies based on current listings
- All opportunities saved successfully
- No API errors (check for rate limiting)

**Verification:**
- Check `./test-live-queue/queue/` for saved opportunities
- Verify opportunities have valid data:
  - Non-empty ID, Title, Agency
  - Valid dates (PostedDate, ResponseDeadline)
  - Correct NAICS code (541512)
  - Valid URL to SAM.gov

#### Test 2: Multiple NAICS Codes

```bash
go run cmd/hunter/main.go \
  --mode=live \
  --naics=541512,541519 \
  --store-path=./test-multi-naics
```

**Expected Results:**
- Fetches opportunities for both NAICS codes
- Opportunities are deduplicated (if same opportunity matches multiple codes)
- All unique opportunities saved

**Verification:**
- Check for opportunities with NAICS 541512 OR 541519
- Verify no duplicate opportunity IDs

#### Test 3: Pagination Handling

```bash
# Use a common NAICS code to trigger pagination
go run cmd/hunter/main.go \
  --mode=live \
  --naics=541512 \
  --store-path=./test-pagination
```

**Expected Results:**
- If > 100 opportunities exist, Hunter automatically paginates
- All pages fetched (check logs for "Fetched X opportunities")
- Rate limiting respected (200ms delay between requests)

**Verification:**
- Count opportunities saved: `ls ./test-pagination/queue/ | wc -l`
- Verify count matches "Opportunities fetched" in summary

#### Test 4: Error Handling - Invalid API Key

```bash
SAM_API_KEY="invalid-key" go run cmd/hunter/main.go \
  --mode=live \
  --naics=541512
```

**Expected Results:**
- Hunter should fail gracefully with API authentication error
- Error message should be clear and actionable
- No partial data saved

#### Test 5: Error Handling - Invalid NAICS Code

```bash
go run cmd/hunter/main.go \
  --mode=live \
  --naics=999999 \
  --store-path=./test-invalid-naics
```

**Expected Results:**
- Hunter completes successfully
- 0 opportunities fetched (NAICS code doesn't match any opportunities)
- No errors reported

### Live API Testing Checklist

When testing against live SAM.gov API, verify:

- [ ] API authentication works with valid key
- [ ] Opportunities are fetched for specified NAICS codes
- [ ] Pagination works correctly (if > 100 results)
- [ ] Rate limiting is respected (200ms between requests)
- [ ] All opportunity fields are populated correctly
- [ ] Dates are parsed correctly (PostedDate, ResponseDeadline)
- [ ] Attachments URLs are valid
- [ ] Place of performance is formatted correctly
- [ ] Opportunities are saved to store successfully
- [ ] Saved JSON files are valid and readable
- [ ] Duplicate opportunities are handled (if any)
- [ ] Summary statistics are accurate
- [ ] Error handling works for invalid API key
- [ ] Error handling works for network issues

### Performance Benchmarks

Expected performance (varies by network and SAM.gov API response time):

- **Cached mode**: < 100ms for 3 opportunities
- **Live mode**:
  - Single NAICS, < 100 opps: 5-15 seconds
  - Single NAICS, > 100 opps: Add ~200ms per page
  - Multiple NAICS: Multiply by number of codes

### Known Limitations

1. **NAICS Description**: SAM.gov API doesn't always provide NAICS descriptions in search results. The Hunter leaves this field empty. Consider adding a NAICS lookup table in Phase 1.

2. **Contract Type**: Not always included in SAM.gov search results. Field may be empty until full opportunity details are fetched.

3. **Rate Limiting**: SAM.gov has rate limits. Hunter includes 200ms delay between paginated requests. For heavy usage, consider implementing exponential backoff.

4. **API Changes**: SAM.gov API structure may change. If tests fail after API update, check SAM.gov API documentation for schema changes.

## Troubleshooting

### Issue: "failed to read cached fixture"
**Solution**: Ensure you're running from the project root, or the test is finding the fixture file at `test/fixtures/samgov_response.json`.

### Issue: "SAM_API_KEY environment variable is required"
**Solution**: Set the SAM_API_KEY environment variable before running in live mode.

### Issue: "failed to create queue directory"
**Solution**: Ensure the store path is writable. Default is `./queue` in current directory.

### Issue: API returns 429 Too Many Requests
**Solution**: SAM.gov rate limiting triggered. Wait a few minutes and retry. Consider increasing delay between requests.

### Issue: Opportunities have empty fields
**Solution**: Some SAM.gov opportunities have incomplete data. This is expected - Hunter saves what's available.

## Continuous Integration

The CI pipeline runs only cached mode tests:

```yaml
# Example GitHub Actions workflow
- name: Run tests
  run: go test ./... -v
```

Live API tests are NOT run in CI because:
1. Require API credentials (security risk)
2. Depend on external service availability
3. Results vary over time (non-deterministic)
4. Rate limiting concerns

Live API testing is performed manually before releases.
