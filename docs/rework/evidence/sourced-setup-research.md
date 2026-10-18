# sourced-setup-research evidence

## Commands

    rtk go test ./server/internal/setup

## Coverage

- Tavily POST carries the configured bearer key and maps results into sources plus leads.
- SearXNG requests format=json and reports an explicit error when HTML comes back.
- 429 and 403 responses produce named provider errors.
- Malformed JSON is reported as malformed, not as an empty result set.
- Direct HTTP(S) inspection works while search is disabled.
- Inspection retains page text, marks a source inspected, hashes the content, and leaves an unknown game build unknown. Embedded instructions in the page are retained as data.
- Non-HTTP URLs and failed fetches leave the source uninspected.

## External blocker

No configured search provider or key is available, so no real provider search or live source fetch was executed. Recorded evidence is limited to local httptest fixtures and unit tests.
