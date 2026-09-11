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
	pathSubmit  = "/api/submit"
	pathComment = "/api/comment"

	kindSelf = "self"
	kindLink = "link"

	paramKind        = "kind"
	paramSubreddit   = "sr"
	paramTitle       = "title"
	paramText        = "text"
	paramURL         = "url"
	paramNSFW        = "nsfw"
	paramSpoiler     = "spoiler"
	paramFlairID     = "flair_id"
	paramFlairText   = "flair_text"
	paramSendReplies = "sendreplies"
	paramThingID     = "thing_id"
	paramResubmit    = "resubmit"
)

// WriteError is Reddit rejecting a write on its own terms, such as a rate
// limit or a missing subreddit. The HTTP status is still 200 in that case.
type WriteError struct {
	Errors [][]string
}

func (e *WriteError) Error() string {
	parts := make([]string, 0, len(e.Errors))

	for _, entry := range e.Errors {
		switch len(entry) {
		case 0:
			continue
		case 1:
			parts = append(parts, entry[0])
		default:
			parts = append(parts, fmt.Sprintf("%s: %s", entry[0], entry[1]))
		}
	}

	return "reddit rejected the request: " + strings.Join(parts, "; ")
}

// Code returns the first error code Reddit gave, such as RATELIMIT.
func (e *WriteError) Code() string {
	if len(e.Errors) > 0 && len(e.Errors[0]) > 0 {
		return e.Errors[0][0]
	}

	return ""
}

type SubmitOptions struct {
	Subreddit string
	Title     string
	Text      string
	URL       string
	NSFW      bool
	Spoiler   bool
	FlairID   string
	FlairText string
	// NoReplyNotifications turns off inbox replies for the new post.
	NoReplyNotifications bool
}

// Submission is a newly created post.
type Submission struct {
	ID        string `json:"id"`
	Fullname  string `json:"fullname"`
	URL       string `json:"url"`
	Subreddit string `json:"subreddit"`
	Title     string `json:"title"`
}

type submitResponse struct {
	JSON struct {
		Errors [][]string `json:"errors"`
		Data   struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"data"`
	} `json:"json"`
}

type commentResponse struct {
	JSON struct {
		Errors [][]string `json:"errors"`
		Data   struct {
			Things []thing `json:"things"`
		} `json:"data"`
	} `json:"json"`
}

// Submit creates a post. Exactly one of Text or URL must be set.
func (c *Client) Submit(ctx context.Context, o SubmitOptions) (Result[Submission], error) {
	sub := trimSubreddit(o.Subreddit)
	if sub == "" {
		return Result[Submission]{}, fmt.Errorf("subreddit is required")
	}

	if strings.TrimSpace(o.Title) == "" {
		return Result[Submission]{}, fmt.Errorf("title is required")
	}

	hasText := strings.TrimSpace(o.Text) != ""
	hasURL := strings.TrimSpace(o.URL) != ""

	if hasText == hasURL {
		return Result[Submission]{}, fmt.Errorf("provide either text or url, not both")
	}

	form := url.Values{
		paramSubreddit:   {sub},
		paramTitle:       {o.Title},
		paramNSFW:        {boolParam(o.NSFW)},
		paramSpoiler:     {boolParam(o.Spoiler)},
		paramSendReplies: {boolParam(!o.NoReplyNotifications)},
	}

	if hasText {
		form.Set(paramKind, kindSelf)
		form.Set(paramText, o.Text)
	} else {
		form.Set(paramKind, kindLink)
		form.Set(paramURL, o.URL)
		form.Set(paramResubmit, "true")
	}

	setIf(form, paramFlairID, o.FlairID)
	setIf(form, paramFlairText, o.FlairText)

	raw, err := c.PostForm(ctx, pathSubmit, form)
	if err != nil {
		return Result[Submission]{}, err
	}

	var resp submitResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return Result[Submission]{}, fmt.Errorf("decode %s: %w (body starts: %.120s)", pathSubmit, err, raw)
	}

	if len(resp.JSON.Errors) > 0 {
		return Result[Submission]{}, &WriteError{resp.JSON.Errors}
	}

	created := Submission{
		ID:        resp.JSON.Data.ID,
		Fullname:  resp.JSON.Data.Name,
		URL:       resp.JSON.Data.URL,
		Subreddit: sub,
		Title:     o.Title,
	}

	return Result[Submission]{created, raw}, nil
}

// Reply comments on a post or replies to another comment. parent accepts a
// fullname, a post id, or a post or comment URL.
func (c *Client) Reply(ctx context.Context, parent, text string) (Result[Comment], error) {
	if strings.TrimSpace(text) == "" {
		return Result[Comment]{}, fmt.Errorf("text is required")
	}

	target, err := ParseThingID(parent)
	if err != nil {
		return Result[Comment]{}, err
	}

	form := url.Values{
		paramThingID: {target},
		paramText:    {text},
	}

	raw, err := c.PostForm(ctx, pathComment, form)
	if err != nil {
		return Result[Comment]{}, err
	}

	var resp commentResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return Result[Comment]{}, fmt.Errorf("decode %s: %w (body starts: %.120s)", pathComment, err, raw)
	}

	if len(resp.JSON.Errors) > 0 {
		return Result[Comment]{}, &WriteError{resp.JSON.Errors}
	}

	created := decodeChildren(listing{Children: resp.JSON.Data.Things}, kindComment, rawComment.clean)
	if len(created) == 0 {
		return Result[Comment]{}, fmt.Errorf("reddit accepted the reply but returned no comment")
	}

	return Result[Comment]{created[0], raw}, nil
}

func boolParam(b bool) string {
	if b {
		return "true"
	}

	return "false"
}

var (
	fullnamePattern        = regexp.MustCompile(`^(t[135]_[a-z0-9]+)$`)
	commentPermalinkRegexp = regexp.MustCompile(`/comments/[a-z0-9]+/[^/]*/([a-z0-9]+)`)
)

// ParseThingID resolves what a reply should attach to. A comment permalink
// wins over the post it sits under, since that is what the link points at.
func ParseThingID(s string) (string, error) {
	s = strings.TrimSpace(s)

	if m := fullnamePattern.FindStringSubmatch(s); m != nil {
		return m[1], nil
	}

	if m := commentPermalinkRegexp.FindStringSubmatch(s); m != nil {
		return prefixComment + m[1], nil
	}

	id, err := ParsePostID(s)
	if err != nil {
		return "", fmt.Errorf("cannot tell what %q refers to; pass a post URL, a comment URL, or a t1_/t3_ fullname", s)
	}

	return prefixPost + id, nil
}

const (
	pathEdit   = "/api/editusertext"
	pathDelete = "/api/del"

	deletedMarker = "[deleted]"
)

// Edited is whatever the edit changed. Reddit returns the updated thing, and
// which field is set depends on whether it was a post or a comment.
type Edited struct {
	Kind    string   `json:"kind"`
	Post    *Post    `json:"post,omitempty"`
	Comment *Comment `json:"comment,omitempty"`
}

// Deletion reports what happened, including whether the item actually went
// away. Reddit answers a delete the same way whether or not the item was
// yours, so this is checked separately.
type Deletion struct {
	Fullname string `json:"fullname"`
	Deleted  bool   `json:"deleted"`
	Note     string `json:"note,omitempty"`
}

// Edit rewrites the body of your own text post or comment. Link posts have no
// body to edit, and Reddit refuses those.
func (c *Client) Edit(ctx context.Context, target, text string) (Result[Edited], error) {
	if strings.TrimSpace(text) == "" {
		return Result[Edited]{}, fmt.Errorf("text is required")
	}

	thing, err := ParseThingID(target)
	if err != nil {
		return Result[Edited]{}, err
	}

	form := url.Values{
		paramThingID: {thing},
		paramText:    {text},
	}

	raw, err := c.PostForm(ctx, pathEdit, form)
	if err != nil {
		return Result[Edited]{}, err
	}

	var resp commentResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return Result[Edited]{}, fmt.Errorf("decode %s: %w (body starts: %.120s)", pathEdit, err, raw)
	}

	if len(resp.JSON.Errors) > 0 {
		return Result[Edited]{}, &WriteError{resp.JSON.Errors}
	}

	edited, err := firstEdited(resp.JSON.Data.Things)
	if err != nil {
		return Result[Edited]{}, err
	}

	return Result[Edited]{edited, raw}, nil
}

func firstEdited(things []thing) (Edited, error) {
	for _, t := range things {
		switch t.Kind {
		case kindComment:
			var rc rawComment
			if err := json.Unmarshal(t.Data, &rc); err != nil {
				return Edited{}, err
			}

			clean := rc.clean()

			return Edited{Kind: "comment", Comment: &clean}, nil
		case kindPost:
			var rp rawPost
			if err := json.Unmarshal(t.Data, &rp); err != nil {
				return Edited{}, err
			}

			clean := rp.clean()

			return Edited{Kind: "post", Post: &clean}, nil
		}
	}

	return Edited{}, fmt.Errorf("reddit accepted the edit but returned nothing to show for it")
}

// Delete removes your own post or comment. Reddit reports success even when
// the item was never yours, so this confirms the outcome before returning.
func (c *Client) Delete(ctx context.Context, target string) (Result[Deletion], error) {
	thing, err := ParseThingID(target)
	if err != nil {
		return Result[Deletion]{}, err
	}

	raw, err := c.PostForm(ctx, pathDelete, url.Values{paramID: {thing}})
	if err != nil {
		return Result[Deletion]{}, err
	}

	result := Deletion{Fullname: thing}
	result.Deleted, result.Note = c.confirmDeleted(ctx, thing)

	return Result[Deletion]{result, raw}, nil
}

func (c *Client) confirmDeleted(ctx context.Context, thing string) (bool, string) {
	l, _, err := getThing[listing](ctx, c, pathInfo, url.Values{paramID: {thing}})
	if err != nil {
		return false, "could not confirm: " + err.Error()
	}

	if len(l.Children) == 0 {
		return true, ""
	}

	var item struct {
		Author   string `json:"author"`
		Selftext string `json:"selftext"`
		Body     string `json:"body"`
	}
	if err := json.Unmarshal(l.Children[0].Data, &item); err != nil {
		return false, "could not confirm: " + err.Error()
	}

	if item.Author == deletedMarker || item.Selftext == deletedMarker || item.Body == deletedMarker {
		return true, ""
	}

	return false, "reddit accepted the request but the item still looks intact, which usually means it is not yours"
}
