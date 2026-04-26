import { sleep } from 'k6';

import { config } from '../lib/config.js';
import { checkRedirect, randomLink, redirect, seedLinks } from '../lib/http.js';

export const options = {
  systemTags: config.systemTags,
  vus: config.vus,
  duration: config.duration,
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<250', 'p(99)<750'],
  },
};

export function setup() {
  return {
    links: seedLinks(config.linkPoolSize, 'k6-steady'),
  };
}

export default function (data) {
  const link = randomLink(data.links);
  const response = redirect(link.short_code);
  checkRedirect(response);
  sleep(config.requestDelaySeconds);
}
