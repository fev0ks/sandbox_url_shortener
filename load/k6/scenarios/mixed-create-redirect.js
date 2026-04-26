import { sleep } from 'k6';

import { config } from '../lib/config.js';
import { checkRedirect, createLink, randomLink, redirect, seedLinks } from '../lib/http.js';

export const options = {
  systemTags: config.systemTags,
  vus: config.vus,
  duration: config.duration,
  thresholds: {
    http_req_failed: ['rate<0.03'],
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
  },
};

export function setup() {
  return {
    links: seedLinks(config.linkPoolSize, 'k6-mixed-seed'),
  };
}

export default function (data) {
  if (Math.random() < config.createWeight) {
    createLink('k6-mixed-create', __VU + __ITER);
    sleep(config.requestDelaySeconds);
    return;
  }

  const link = randomLink(data.links);
  const response = redirect(link.short_code);
  checkRedirect(response);
  sleep(config.requestDelaySeconds);
}
