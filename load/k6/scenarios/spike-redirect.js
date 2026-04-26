import { sleep } from 'k6';

import { config } from '../lib/config.js';
import { checkRedirect, randomLink, redirect, seedLinks } from '../lib/http.js';

export const options = {
  systemTags: config.systemTags,
  stages: [
    { duration: '15s', target: config.vus },
    { duration: '15s', target: config.spikePeakVus },
    { duration: '30s', target: config.spikePeakVus },
    { duration: '15s', target: config.vus },
    { duration: '15s', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.03'],
    http_req_duration: ['p(95)<500', 'p(99)<1500'],
  },
};

export function setup() {
  return {
    links: seedLinks(config.linkPoolSize, 'k6-spike'),
  };
}

export default function (data) {
  const link = randomLink(data.links);
  const response = redirect(link.short_code);
  checkRedirect(response);
  sleep(config.requestDelaySeconds);
}
