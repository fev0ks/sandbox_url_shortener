export function envString(name, fallback) {
  const value = __ENV[name];
  if (value === undefined || value === '') {
    return fallback;
  }
  return value;
}

export function envInt(name, fallback) {
  const raw = envString(name, String(fallback));
  const value = Number.parseInt(raw, 10);
  if (Number.isNaN(value) || value <= 0) {
    return fallback;
  }
  return value;
}

export function envFloat(name, fallback) {
  const raw = envString(name, String(fallback));
  const value = Number.parseFloat(raw);
  if (Number.isNaN(value) || value < 0) {
    return fallback;
  }
  return value;
}

export const config = {
  baseUrl: envString('BASE_URL', 'http://localhost:8080').replace(/\/$/, ''),
  duration: envString('DURATION', '30s'),
  vus: envInt('VUS', 5),
  linkPoolSize: envInt('LINK_POOL_SIZE', 5),
  createWeight: envFloat('CREATE_WEIGHT', 0.02),
  requestDelaySeconds: envFloat('REQUEST_DELAY_SECONDS', 1),
  createClientIPPool: envInt('CREATE_CLIENT_IP_POOL', 20),
  spikePeakVus: envInt('SPIKE_PEAK_VUS', 20),
  saturationVus: envInt('SATURATION_VUS', 50),
  saturationDuration: envString('SATURATION_DURATION', '2m'),
  systemTags: ['status', 'method', 'name', 'scenario', 'expected_response', 'check'],
};
