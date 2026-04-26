import { sleep } from 'k6';

import { config } from '../lib/config.js';
import { checkRedirect, randomLink, redirect, seedLinks } from '../lib/http.js';

if (__ENV.ALLOW_SATURATION !== '1') {
  throw new Error('saturation scenario requires ALLOW_SATURATION=1');
}

export const options = {
  systemTags: config.systemTags,
  vus: config.saturationVus,
  duration: config.saturationDuration,
  thresholds: {
    http_req_failed: ['rate<0.10'],
    http_req_duration: ['p(95)<1000', 'p(99)<3000'],
  },
};

export function setup() {
  return {
    links: seedLinks(config.linkPoolSize, 'k6-saturation'),
  };
}

export default function (data) {
  const link = randomLink(data.links);
  const response = redirect(link.short_code);
  checkRedirect(response);
  sleep(config.requestDelaySeconds);
}
