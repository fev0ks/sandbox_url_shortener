package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"sandbox_url_shortener/services/mcp-logs/internal/domain"
)

// DiscoveryWindow is how far back the registry looks for a service. It is
// deliberately wider than any tool window: a service that failed silently for a
// day should still be reachable, and a service absent for this long is not
// something an investigator can query anyway.
const DiscoveryWindow = 7 * 24 * time.Hour

// registry answers "which services exist" and "what can I filter on" from the
// index rather than from configuration.
//
// Both answers are cached. A terms aggregation across a whole cluster is not a
// cheap query, the answers change on the timescale of a deployment rather than
// a conversation, and an agent calls list_services at the start of every
// investigation.
type registry struct {
	repo      RegistryRepo
	allowlist []string
	domains   map[string][]string
	ttl       time.Duration
	now       func() time.Time

	mu       sync.Mutex
	services cacheEntry[[]domain.ServiceInfo]
	fields   map[string]cacheEntry[[]domain.FieldInfo]
}

type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
	ok        bool
}

func (e cacheEntry[T]) valid(now time.Time) bool {
	return e.ok && now.Before(e.expiresAt)
}

// RegistryRepo is the discovery half of the log repository.
type RegistryRepo interface {
	DiscoverServices(ctx context.Context, since time.Time, limit int) ([]domain.ServiceInfo, error)
	DescribeFields(ctx context.Context, service string, since time.Time) ([]domain.FieldInfo, int, bool, error)
}

func newRegistry(repo RegistryRepo, allowlist []string, domains map[string][]string, ttl time.Duration, now func() time.Time) *registry {
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &registry{
		repo:      repo,
		allowlist: allowlist,
		domains:   domains,
		ttl:       ttl,
		now:       now,
		fields:    make(map[string]cacheEntry[[]domain.FieldInfo]),
	}
}

// Services returns the discovered registry, filtered by the operator allowlist.
func (r *registry) Services(ctx context.Context) ([]domain.ServiceInfo, error) {
	now := r.now()

	r.mu.Lock()
	if r.services.valid(now) {
		cached := r.services.value
		r.mu.Unlock()
		return cached, nil
	}
	r.mu.Unlock()

	found, err := r.repo.DiscoverServices(ctx, now.Add(-DiscoveryWindow), 0)
	if err != nil {
		// Falling back to the allowlist keeps the server usable when discovery
		// fails, but only to names an operator already vouched for.
		if len(r.allowlist) > 0 {
			return r.fromAllowlist(), nil
		}
		return nil, fmt.Errorf("discover services: %w", err)
	}

	out := make([]domain.ServiceInfo, 0, len(found))
	for _, svc := range found {
		if !r.allowed(svc.Service) {
			continue
		}
		svc.Domain = r.domainOf(svc.Service)
		out = append(out, svc)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Service < out[j].Service })

	r.mu.Lock()
	r.services = cacheEntry[[]domain.ServiceInfo]{value: out, expiresAt: now.Add(r.ttl), ok: true}
	r.mu.Unlock()
	return out, nil
}

// Fields returns the filterable fields of one service.
func (r *registry) Fields(ctx context.Context, service string) ([]domain.FieldInfo, error) {
	now := r.now()

	r.mu.Lock()
	if entry, hit := r.fields[service]; hit && entry.valid(now) {
		cached := entry.value
		r.mu.Unlock()
		return cached, nil
	}
	r.mu.Unlock()

	fields, _, _, err := r.repo.DescribeFields(ctx, service, now.Add(-DiscoveryWindow))
	if err != nil {
		return nil, fmt.Errorf("describe fields: %w", err)
	}

	r.mu.Lock()
	r.fields[service] = cacheEntry[[]domain.FieldInfo]{value: fields, expiresAt: now.Add(r.ttl), ok: true}
	r.mu.Unlock()
	return fields, nil
}

func (r *registry) fromAllowlist() []domain.ServiceInfo {
	out := make([]domain.ServiceInfo, 0, len(r.allowlist))
	for _, name := range r.allowlist {
		out = append(out, domain.ServiceInfo{Service: name, Domain: r.domainOf(name)})
	}
	return out
}

func (r *registry) allowed(service string) bool {
	if len(r.allowlist) == 0 {
		return true
	}
	for _, name := range r.allowlist {
		if name == service {
			return true
		}
	}
	return false
}

func (r *registry) domainOf(service string) string {
	for domain, members := range r.domains {
		for _, name := range members {
			if name == service {
				return domain
			}
		}
	}
	return ""
}

// Domains lists the configured domains in a stable order.
func (r *registry) Domains() []string {
	names := make([]string, 0, len(r.domains))
	for name := range r.domains {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// names is the flat list of known service names.
func names(services []domain.ServiceInfo) []string {
	out := make([]string, 0, len(services))
	for _, svc := range services {
		out = append(out, svc.Service)
	}
	return out
}

// nearest returns the known values closest to want, best first.
//
// A bare "unknown service" is a dead end for an agent that guessed a plausible
// name; the near matches usually contain the one it meant, and turn a failed
// call into a corrected one rather than an abandoned line of investigation.
func nearest(want string, known []string, limit int) []string {
	type scored struct {
		value string
		dist  int
	}

	want = strings.ToLower(want)
	candidates := make([]scored, 0, len(known))
	for _, k := range known {
		d := editDistance(want, strings.ToLower(k))
		// Anything more than a third of the length away is a different word,
		// not a typo, and suggesting it would be noise.
		if d <= 2 || d*3 <= len(k) || strings.Contains(strings.ToLower(k), want) {
			candidates = append(candidates, scored{value: k, dist: d})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].dist < candidates[j].dist })

	out := make([]string, 0, limit)
	for _, c := range candidates {
		if len(out) == limit {
			break
		}
		out = append(out, c.value)
	}
	return out
}

// editDistance is the Levenshtein distance between two short strings.
func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
