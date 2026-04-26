import http from 'k6/http';
import { check, fail } from 'k6';

import { config } from './config.js';

export function createLink(title = 'k6 local load target', clientSlot = 0) {
  const payload = JSON.stringify({
    original_url: `${config.baseUrl}/health/live`,
    title,
    max_clicks: 0,
  });

  const response = http.post(`${config.baseUrl}/api/v1/links`, payload, {
    headers: {
      'Content-Type': 'application/json',
      'Idempotency-Key': `k6-${title}-${Date.now()}-${Math.random()}`,
      'X-Forwarded-For': createClientIP(clientSlot),
    },
    tags: {
      endpoint: 'link_create',
      name: 'POST /api/v1/links',
    },
  });

  const ok = check(response, {
    'create status is 201': (res) => res.status === 201,
    'create has short_code': (res) => {
      const body = parseJSON(res);
      return Boolean(body && body.short_code);
    },
  });

  if (!ok) {
    fail(`failed to create link: status=${response.status} body=${response.body}`);
  }

  return parseJSON(response);
}

export function redirect(shortCode) {
  return http.get(`${config.baseUrl}/${shortCode}`, {
    redirects: 0,
    tags: {
      endpoint: 'redirect',
      name: 'GET /:shortCode',
    },
  });
}

export function checkRedirect(response) {
  return check(response, {
    'redirect status is 302': (res) => res.status === 302,
    'redirect location is local health endpoint': (res) => {
      const location = res.headers.Location || res.headers.location || '';
      return location.endsWith('/health/live');
    },
  });
}

export function seedLinks(count, prefix) {
  const links = [];
  for (let i = 0; i < count; i += 1) {
    links.push(createLink(`${prefix}-${i}`, i));
  }
  return links;
}

export function randomLink(links) {
  return links[Math.floor(Math.random() * links.length)];
}

function parseJSON(response) {
  try {
    return response.json();
  } catch (_) {
    return null;
  }
}

function createClientIP(slot) {
  const poolSize = Math.max(1, Math.min(config.createClientIPPool, 250));
  const host = (Math.abs(slot) % poolSize) + 1;
  return `198.51.100.${host}`;
}
