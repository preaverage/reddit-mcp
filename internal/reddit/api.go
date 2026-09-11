package reddit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const (
	pathMe           = "/api/v1/me"
	pathInfo         = "/api/info"
	pathMoreChildren = "/api/morechildren"
	pathSearch       = "/search"

	sortHot    = "hot"
	selfUser   = "me"
	frontpage  = "frontpage"
	savedLinks = "links"

	paramID       = "id"
	paramDepth    = "depth"
	paramComment  = "comment"
	paramContext  = "context"
	paramAPIType  = "api_type"
	paramLinkID   = "link_id"
	paramChildren = "children"
	paramQuery    = "q"
	paramType     = "type"
	paramRestrict = "restrict_sr"
	paramOver18   = "include_over_18"

	maxMoreChildren = 100
)

var postIDPatterns = []*regexp.Regexp{
	regexp.MustCompile(`/comments/([a-z0-9]+)`),
	regexp.MustCompile(`redd\.it/([a-z0-9]+)`),
	regexp.MustCompile(`^t3_([a-z0-9]+)$`),
	regexp.MustCompile(`^([a-z0-9]+)$`),
}

func ParsePostID(s string) (string, error) {
	s = strings.TrimSpace(s)

	for _, re := range postIDPatterns {
		if m := re.FindStringSubmatch(s); m != nil {
			return m[1], nil
		}
	}

	return "", fmt.Errorf("cannot parse post id from %q", s)
}

func trimUser(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "/u/")

	return strings.TrimPrefix(name, "u/")
}

func trimSubreddit(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "/r/")

	return strings.TrimPrefix(name, "r/")
}

func userPath(name, section string) string {
	return "/user/" + url.PathEscape(name) + "/" + section
}

func subredditPath(name, section string) string {
	return "/r/" + url.PathEscape(name) + "/" + section
}

func (c *Client) Me(ctx context.Context) (Result[User], error) {
	var r rawUser
	raw, err := c.GetJSON(ctx, pathMe, nil, &r)
	if err != nil {
		return Result[User]{}, err
	}

	c.rememberUsername(r.Name)

	return Result[User]{r.clean(), redact(raw, sensitiveMeFields)}, nil
}

func (c *Client) rememberUsername(name string) {
	if name == "" || c.session().Username == name {
		return
	}

	next := c.session().Clone()
	next.Username = name
	c.sess.Store(next)

	c.persist()
}

func (c *Client) Username(ctx context.Context) (string, error) {
	if name := c.session().Username; name != "" {
		return name, nil
	}

	me, err := c.Me(ctx)
	if err != nil {
		return "", err
	}

	return me.Data.Name, nil
}

func (c *Client) resolveUser(ctx context.Context, name string) (string, error) {
	name = trimUser(name)
	if name != "" && name != selfUser {
		return name, nil
	}

	return c.Username(ctx)
}

func (c *Client) User(ctx context.Context, name string) (Result[User], error) {
	r, raw, err := getThing[rawUser](ctx, c, userPath(trimUser(name), "about"), nil)
	if err != nil {
		return Result[User]{}, err
	}

	return Result[User]{r.clean(), raw}, nil
}

func (c *Client) Post(ctx context.Context, idOrURL string) (Result[Post], error) {
	id, err := ParsePostID(idOrURL)
	if err != nil {
		return Result[Post]{}, err
	}

	l, raw, err := getThing[listing](ctx, c, pathInfo, url.Values{paramID: {prefixPost + id}})
	if err != nil {
		return Result[Post]{}, err
	}

	posts := decodeChildren(l, kindPost, rawPost.clean)
	if len(posts) == 0 {
		return Result[Post]{}, fmt.Errorf("post %s not found", id)
	}

	return Result[Post]{posts[0], raw}, nil
}

func (c *Client) Comments(ctx context.Context, idOrURL string, o CommentsOptions) (Result[Thread], error) {
	id, err := ParsePostID(idOrURL)
	if err != nil {
		return Result[Thread]{}, err
	}

	q := url.Values{}
	setIf(q, paramSort, o.Sort)
	setIfPositive(q, paramLimit, o.Limit)
	setIfPositive(q, paramDepth, o.Depth)
	if o.Comment != "" {
		q.Set(paramComment, strings.TrimPrefix(o.Comment, prefixComment))
		setIfPositive(q, paramContext, o.Context)
	}

	var arr []thing
	raw, err := c.GetJSON(ctx, "/comments/"+id, q, &arr)
	if err != nil {
		return Result[Thread]{}, err
	}
	if len(arr) < 2 {
		return Result[Thread]{}, fmt.Errorf("unexpected comments payload for %s", id)
	}

	thread, err := buildThread(id, arr[0], arr[1])
	if err != nil {
		return Result[Thread]{}, err
	}

	return Result[Thread]{thread, raw}, nil
}

func buildThread(id string, postThing, commentThing thing) (Thread, error) {
	var pl, cl listing

	if err := json.Unmarshal(postThing.Data, &pl); err != nil {
		return Thread{}, err
	}
	if err := json.Unmarshal(commentThing.Data, &cl); err != nil {
		return Thread{}, err
	}

	posts := decodeChildren(pl, kindPost, rawPost.clean)
	if len(posts) == 0 {
		return Thread{}, fmt.Errorf("post %s not found", id)
	}

	th := Thread{Post: posts[0]}

	var count int
	var ids []string
	th.Comments, count, ids = commentTree(cl)

	if len(ids) > 0 {
		th.More = &More{ParentID: th.Post.Fullname, Count: count, IDs: ids}
	}

	return th, nil
}

func (c *Client) MoreChildren(ctx context.Context, postIDOrURL string, ids []string, sort string) (Result[[]Comment], error) {
	id, err := ParsePostID(postIDOrURL)
	if err != nil {
		return Result[[]Comment]{}, err
	}

	ids = ids[:min(len(ids), maxMoreChildren)]

	q := url.Values{
		paramAPIType:  {"json"},
		paramLinkID:   {prefixPost + id},
		paramChildren: {strings.Join(ids, ",")},
	}
	setIf(q, paramSort, sort)

	var resp rawMoreChildren
	raw, err := c.GetJSON(ctx, pathMoreChildren, q, &resp)
	if err != nil {
		return Result[[]Comment]{}, err
	}
	if len(resp.JSON.Errors) > 0 {
		return Result[[]Comment]{}, fmt.Errorf("reddit: %v", resp.JSON.Errors)
	}

	l := listing{Children: resp.JSON.Data.Things}
	out := decodeChildren(l, kindComment, rawComment.clean)
	out = append(out, decodeChildren(l, kindMore, rawMore.clean)...)

	return Result[[]Comment]{out, raw}, nil
}

func (c *Client) UserPosts(ctx context.Context, name string, o ListOptions) (Result[Page[Post]], error) {
	name, err := c.resolveUser(ctx, name)
	if err != nil {
		return Result[Page[Post]]{}, err
	}

	return c.postListing(ctx, userPath(name, "submitted"), o.values())
}

func (c *Client) UserComments(ctx context.Context, name string, o ListOptions) (Result[Page[Comment]], error) {
	name, err := c.resolveUser(ctx, name)
	if err != nil {
		return Result[Page[Comment]]{}, err
	}

	return c.commentListing(ctx, userPath(name, "comments"), o.values())
}

func (c *Client) Saved(ctx context.Context, o ListOptions) (Result[Page[Post]], error) {
	name, err := c.resolveUser(ctx, "")
	if err != nil {
		return Result[Page[Post]]{}, err
	}

	q := o.values()
	q.Set(paramType, savedLinks)

	return c.postListing(ctx, userPath(name, "saved"), q)
}

func (c *Client) SubredditPosts(ctx context.Context, sub string, o ListOptions) (Result[Page[Post]], error) {
	sort := o.Sort
	if sort == "" {
		sort = sortHot
	}

	q := o.values()
	q.Del(paramSort)

	path := "/" + sort
	if sub = trimSubreddit(sub); sub != "" && sub != frontpage {
		path = subredditPath(sub, sort)
	}

	return c.postListing(ctx, path, q)
}

func (c *Client) SubredditInfo(ctx context.Context, sub string) (Result[Subreddit], error) {
	r, raw, err := getThing[rawSubreddit](ctx, c, subredditPath(trimSubreddit(sub), "about"), nil)
	if err != nil {
		return Result[Subreddit]{}, err
	}

	return Result[Subreddit]{r.clean(), raw}, nil
}

func (c *Client) Search(ctx context.Context, query string, o SearchOptions) (Result[Page[Post]], error) {
	q := url.Values{
		paramQuery: {query},
		paramLimit: {clampLimit(o.Limit)},
		paramType:  {"link"},
	}
	setIf(q, paramSort, o.Sort)
	setIf(q, paramTime, o.Time)
	setIf(q, paramAfter, o.After)
	if o.NSFW {
		q.Set(paramOver18, "on")
	}

	path := pathSearch
	if sub := trimSubreddit(o.Subreddit); sub != "" {
		path = subredditPath(sub, "search")
		q.Set(paramRestrict, "1")
	}

	return c.postListing(ctx, path, q)
}
