package ingest

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/time/rate"
)

var (
	rateLimiters map[int]*rate.Limiter // org id -> rate limiter

	ErrRequestExceedsBurst = errors.New("request exceeds limit burst size")
)

func ConfigureRateLimits(limitStr string) error {
	if len(limitStr) == 0 {
		return nil
	}

	rateLimiters = make(map[int]*rate.Limiter)

	limitsWithOrgs := strings.Split(limitStr, ";")
	for _, limitWithOrg := range limitsWithOrgs {
		limitWithOrgParts := strings.SplitN(limitWithOrg, ":", 2)
		if len(limitWithOrgParts) != 2 {
			return fmt.Errorf("Invalid limit configuration string: %q", limitWithOrg)
		}

		orgId, err := strconv.ParseInt(limitWithOrgParts[0], 10, 32)
		if err != nil {
			return fmt.Errorf("Unable to parse orgId from string: %q", limitWithOrgParts[0])
		}

		limit, err := strconv.ParseInt(limitWithOrgParts[1], 10, 32)
		if err != nil {
			return fmt.Errorf("Unable to parse rate limit from string: %q", limitWithOrgParts[1])
		}

		rateLimiters[int(orgId)] = rate.NewLimiter(rate.Limit(limit), int(limit))
	}

	return nil
}

func rateLimit(ctx context.Context, orgId, datapoints int) error {
	limiter, ok := rateLimiters[orgId]
	if !ok {
		return nil
	}

	// if the number of points trying to be added is greater then the limiter
	// burst size, just return an error.
	if datapoints > limiter.Burst() {
		return ErrRequestExceedsBurst
	}
	// wait until we are allowed to publish the given number of datapoints
	return limiter.WaitN(ctx, datapoints)
}

func IsRateBudgetAvailable(ctx context.Context, orgId int) bool {
	limiter, ok := rateLimiters[orgId]
	if !ok {
		return true
	}

	return limiter.Allow()
}

func UseRateLimit() bool {
	return len(rateLimiters) > 0
}
