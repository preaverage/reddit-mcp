package reddit

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

const (
	defaultLimit = 25
	maxLimit     = 100

	paramLimit = "limit"
	paramSort  = "sort"
	paramTime  = "t"
	paramAfter = "after"
)

func clampLimit(n int) string {
	if n <= 0 {
		n = defaultLimit
	}

	return strconv.Itoa(min(n, maxLimit))
}

func (o ListOptions) values() url.Values {
	q := url.Values{paramLimit: {clampLimit(o.Limit)}}

	setIf(q, paramSort, o.Sort)
	setIf(q, paramTime, o.Time)
	setIf(q, paramAfter, o.After)

	return q
}

func setIf(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}

func setIfPositive(q url.Values, key string, value int) {
	if value > 0 {
		q.Set(key, strconv.Itoa(value))
	}
}

func getThing[T any](ctx context.Context, c *Client, path string, q url.Values) (T, json.RawMessage, error) {
	var v T

	var t thing
	raw, err := c.GetJSON(ctx, path, q, &t)
	if err != nil {
		return v, nil, err
	}

	if err := json.Unmarshal(t.Data, &v); err != nil {
		return v, nil, err
	}

	return v, raw, nil
}

func decodeChildren[R any, T any](l listing, kind string, clean func(R) T) []T {
	out := make([]T, 0, len(l.Children))

	for _, ch := range l.Children {
		if ch.Kind != kind {
			continue
		}

		var r R
		if json.Unmarshal(ch.Data, &r) == nil {
			out = append(out, clean(r))
		}
	}

	return out
}

func (c *Client) postListing(ctx context.Context, path string, q url.Values) (Result[Page[Post]], error) {
	l, raw, err := getThing[listing](ctx, c, path, q)
	if err != nil {
		return Result[Page[Post]]{}, err
	}

	page := Page[Post]{
		Items: decodeChildren(l, kindPost, rawPost.clean),
		After: l.After,
	}

	return Result[Page[Post]]{page, raw}, nil
}

func (c *Client) commentListing(ctx context.Context, path string, q url.Values) (Result[Page[Comment]], error) {
	l, raw, err := getThing[listing](ctx, c, path, q)
	if err != nil {
		return Result[Page[Comment]]{}, err
	}

	page := Page[Comment]{
		Items: decodeChildren(l, kindComment, rawComment.clean),
		After: l.After,
	}

	return Result[Page[Comment]]{page, raw}, nil
}
