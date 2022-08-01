package cf

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type queryBuilder struct {
	filters  map[string]string
	pageSize int
}

func (q *queryBuilder) set(key string, elements []string) *queryBuilder {
	if q.filters == nil {
		q.filters = make(map[string]string)
	}
	searchParameters := strings.Join(elements, ",")
	q.filters[key] = searchParameters
	return q
}

func (q *queryBuilder) build() url.Values {
	query := url.Values{}
	for key, params := range q.filters {
		query.Add("q", fmt.Sprintf("%s IN %s", key, params))
	}
	if q.pageSize > 0 {
		query.Set("results-per-page", strconv.Itoa(q.pageSize))
	}
	return query
}

func (pc *PlatformClient) buildQuery(key string, values ...string) url.Values {
	query := queryBuilder{
		pageSize: pc.settings.CF.PageSize,
	}
	query.set(key, values)
	return query.build()
}
