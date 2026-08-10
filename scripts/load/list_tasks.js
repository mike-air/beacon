// Chapter 50 — the load test that tells you where the knee is.
//
// The stages are the point. Holding one steady number tells you whether the
// system survives THAT number. Ramping tells you where it stops surviving, and
// that inflection is what you actually need to know before a launch.
//
// The thresholds turn this into a CI gate: k6 exits non-zero when p95 crosses
// 500ms or the error rate crosses 1%, so a performance regression fails a build
// instead of surprising you in production.
//
//   BEACON_TOKEN=... BEACON_ORG_ID=... BEACON_PROJECT_ID=... \
//     k6 run scripts/load/list_tasks.js
//
// [verbatim ch50] with the URL pointed at the local server and the org path
// segment this repo's routes require.
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 50 },   // ramp to 50 virtual users
    { duration: '2m', target: 50 },    // hold 50 for two minutes
    { duration: '30s', target: 200 },  // ramp to 200
    { duration: '2m', target: 200 },   // hold 200 for two minutes
    { duration: '30s', target: 0 },    // ramp back down
  ],
  thresholds: {
    http_req_failed: ['rate<0.01'],            // <1% errors
    http_req_duration: ['p(95)<500'],          // p95 under 500ms
  },
};

const BASE = __ENV.BEACON_BASE_URL || 'http://localhost:8080';
const TOKEN = __ENV.BEACON_TOKEN;
const ORG_ID = __ENV.BEACON_ORG_ID;
const PROJECT_ID = __ENV.BEACON_PROJECT_ID;

export default function () {
  const res = http.get(
    `${BASE}/v1/orgs/${ORG_ID}/projects/${PROJECT_ID}/tasks?limit=50`,
    { headers: { Authorization: `Bearer ${TOKEN}` } },
  );
  check(res, {
    'status is 200': (r) => r.status === 200,
    'body has items': (r) => r.json('items') !== undefined,
  });
  sleep(1);
}
