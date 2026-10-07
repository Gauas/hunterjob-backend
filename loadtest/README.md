# Authentication load test

`authn.js` is a k6 scenario for authenticated dashboard traffic with user think time. It does not equate 10,000 concurrent users with 10,000 requests/second. Use a dedicated test environment and tokens belonging to seeded users.

```sh
k6 run -e API_URL=https://api.example.test -e ACCESS_TOKEN="$ACCESS_TOKEN" \
  -e VUS=100 -e DURATION=5m -e THINK_SECONDS=3 loadtest/authn.js
```

Increase VUs gradually while observing HunterJob and Identity Service CPU/memory and application p50/p95/p99, RPS, and error rate. To reach 10k VUs, distribute the load generators and use many seeded users/tokens; a single shared token is only useful for an initial authentication-path smoke test.

For a separate refresh/logout check, pass a refresh token for a disposable session with `-e REFRESH_TOKEN="$REFRESH_TOKEN"`. That scenario rotates the refresh token once, verifies the old token is rejected, logs out, and verifies the new refresh token is rejected. Do not reuse that token after the test.

The unit tests in `pkg/authn` cover JWKS cache hit, concurrent first miss, unknown `kid` after rotation, wrong audience and expiration. For a deployment load test, count calls to `/.well-known/jwks.json` at Identity Service or ingress and confirm they scale with application replicas/cache refreshes, not with authenticated RPS. Normal dashboard traffic should not produce `/ValidateToken` calls, Identity Service requests or Redis auth lookups. Record those measurements explicitly; this repository does not contain a benchmark result.
