// k6 run -e API_URL=https://api.gauas.com -e ACCESS_TOKEN=... loadtest/authn.js
// Set VUS=10000 only with distinct test users/tokens and a sized load generator.
import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

const base = (__ENV.API_URL || "http://localhost:8080").replace(/\/$/, "");
const thinkSeconds = Number(__ENV.THINK_SECONDS || "3");
const failures = new Rate("authn_failures");
const normalLatency = new Trend("authenticated_api_ms", true);

export const options = {
  scenarios: {
    normal_api: {
      executor: "constant-vus",
      vus: Number(__ENV.VUS || "100"),
      duration: __ENV.DURATION || "5m",
      exec: "normalAPI",
    },
    ...(__ENV.REFRESH_TOKEN ? {
      session_flow: { executor: "shared-iterations", vus: 1, iterations: 1, exec: "sessionFlow" },
    } : {}),
  },
  thresholds: {
    authn_failures: ["rate<0.01"],
    authenticated_api_ms: ["p(95)<1000", "p(99)<2000"],
  },
};

export function normalAPI() {
  if (!__ENV.ACCESS_TOKEN) throw new Error("ACCESS_TOKEN is required");
  const response = http.get(`${base}/v1/hunterjob/dashboard`, {
    headers: { Authorization: `Bearer ${__ENV.ACCESS_TOKEN}` },
  });
  const ok = check(response, { "authenticated dashboard succeeds": (r) => r.status === 200 });
  failures.add(!ok);
  normalLatency.add(response.timings.duration);
  sleep(thinkSeconds);
}

export function sessionFlow() {
  const refresh = http.post(`${base}/v1/auth/refresh`, JSON.stringify({ refresh_token: __ENV.REFRESH_TOKEN }), {
    headers: { "Content-Type": "application/json" },
  });
  const refreshed = check(refresh, { "refresh succeeds": (r) => r.status === 200 });
  failures.add(!refreshed);
  if (!refreshed) return;
  const tokens = refresh.json();
  const oldRefresh = http.post(`${base}/v1/auth/refresh`, JSON.stringify({ refresh_token: __ENV.REFRESH_TOKEN }), {
    headers: { "Content-Type": "application/json" },
  });
  failures.add(!check(oldRefresh, { "rotated refresh token rejected": (r) => r.status === 401 }));
  const logout = http.post(`${base}/v1/auth/logout`, null, {
    headers: { Authorization: `Bearer ${tokens.access_token}` },
  });
  failures.add(!check(logout, { "logout succeeds": (r) => r.status === 204 }));
  const revokedRefresh = http.post(`${base}/v1/auth/refresh`, JSON.stringify({ refresh_token: tokens.refresh_token }), {
    headers: { "Content-Type": "application/json" },
  });
  failures.add(!check(revokedRefresh, { "revoked refresh token rejected": (r) => r.status === 401 }));
}
