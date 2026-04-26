import { check } from 'k6';

import { config } from '../lib/config.js';
import { createLink, redirect } from '../lib/http.js';

export const options = {
  systemTags: config.systemTags,
  vus: 1,
  iterations: 1,
  thresholds: {
    http_req_failed: ['rate==0'],
    http_req_duration: ['p(95)<500'],
  },
};

export default function () {
  const link = createLink('k6-smoke');
  const response = redirect(link.short_code);

  check(response, {
    'smoke redirect returns 302': (res) => res.status === 302,
  });
}
