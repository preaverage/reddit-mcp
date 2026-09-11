package reddit

import (
	"encoding/json"
	"strings"
	"time"
)

const (
	voteUp   = "up"
	voteDown = "down"
)

func timestamp(f float64) string {
	if f == 0 {
		return ""
	}

	return time.Unix(int64(f), 0).UTC().Format(time.RFC3339)
}

func wasEdited(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x > 0
	}

	return false
}

func vote(likes *bool) string {
	switch {
	case likes == nil:
		return ""
	case *likes:
		return voteUp
	default:
		return voteDown
	}
}

func absolute(permalink string) string {
	if permalink == "" || strings.HasPrefix(permalink, "http") {
		return permalink
	}

	return webBase + permalink
}

func (r rawPost) clean() Post {
	return Post{
		ID:            r.ID,
		Fullname:      r.Name,
		Title:         r.Title,
		Author:        r.Author,
		Subreddit:     r.Subreddit,
		Score:         r.Score,
		UpvoteRatio:   r.UpvoteRatio,
		NumComments:   r.NumComments,
		Created:       timestamp(r.CreatedUTC),
		URL:           r.URL,
		Permalink:     absolute(r.Permalink),
		Selftext:      r.Selftext,
		IsSelf:        r.IsSelf,
		NSFW:          r.Over18,
		Spoiler:       r.Spoiler,
		Stickied:      r.Stickied,
		Locked:        r.Locked,
		Archived:      r.Archived,
		Domain:        r.Domain,
		Flair:         r.LinkFlairText,
		AuthorFlair:   r.AuthorFlairText,
		Distinguished: r.Distinguished,
		PostHint:      r.PostHint,
		Edited:        wasEdited(r.Edited),
		MyVote:        vote(r.Likes),
		Saved:         r.Saved,
		Crosspost:     r.CrosspostParent,
	}
}

func (r rawComment) clean() Comment {
	c := Comment{
		ID:            r.ID,
		Fullname:      r.Name,
		Author:        r.Author,
		Body:          r.Body,
		Score:         r.Score,
		ScoreHidden:   r.ScoreHidden,
		Created:       timestamp(r.CreatedUTC),
		Depth:         r.Depth,
		ParentID:      r.ParentID,
		LinkID:        r.LinkID,
		Subreddit:     r.Subreddit,
		LinkTitle:     r.LinkTitle,
		Permalink:     absolute(r.Permalink),
		IsOP:          r.IsSubmitter,
		Stickied:      r.Stickied,
		Distinguished: r.Distinguished,
		Edited:        wasEdited(r.Edited),
		AuthorFlair:   r.AuthorFlairText,
		MyVote:        vote(r.Likes),
		Saved:         r.Saved,
	}

	if l, ok := repliesListing(r.Replies); ok {
		c.Replies, c.MoreReplies, c.MoreIDs = commentTree(l)
	}

	return c
}

func repliesListing(raw json.RawMessage) (listing, bool) {
	if len(raw) == 0 || raw[0] != '{' {
		return listing{}, false
	}

	var t thing
	if json.Unmarshal(raw, &t) != nil {
		return listing{}, false
	}

	var l listing
	if json.Unmarshal(t.Data, &l) != nil {
		return listing{}, false
	}

	return l, true
}

func commentTree(l listing) (out []Comment, moreCount int, moreIDs []string) {
	out = make([]Comment, 0, len(l.Children))

	for _, ch := range l.Children {
		switch ch.Kind {
		case kindComment:
			var rc rawComment
			if json.Unmarshal(ch.Data, &rc) == nil {
				out = append(out, rc.clean())
			}
		case kindMore:
			var m rawMore
			if json.Unmarshal(ch.Data, &m) == nil {
				moreCount += m.Count
				moreIDs = append(moreIDs, m.Children...)
			}
		}
	}

	return out, moreCount, moreIDs
}

func (m rawMore) clean() Comment {
	return Comment{
		ID:          m.ID,
		Fullname:    m.Name,
		ParentID:    m.ParentID,
		Depth:       m.Depth,
		MoreReplies: m.Count,
		MoreIDs:     m.Children,
	}
}

func (r rawUser) clean() User {
	u := User{
		ID:               r.ID,
		Name:             r.Name,
		Created:          timestamp(r.CreatedUTC),
		LinkKarma:        r.LinkKarma,
		CommentKarma:     r.CommentKarma,
		TotalKarma:       r.TotalKarma,
		AwardeeKarma:     r.AwardeeKarma,
		AwarderKarma:     r.AwarderKarma,
		IconImg:          r.IconImg,
		IsGold:           r.IsGold,
		IsMod:            r.IsMod,
		IsEmployee:       r.IsEmployee,
		Verified:         r.Verified,
		HasVerifiedEmail: r.HasVerifiedEmail,
		IsFriend:         r.IsFriend,
		IsBlocked:        r.IsBlocked,
		IsSuspended:      r.IsSuspended,
		InboxCount:       r.InboxCount,
		HasMail:          r.HasMail,
	}

	if r.Subreddit != nil {
		u.Description = r.Subreddit.PublicDescription
		u.DisplayTitle = r.Subreddit.Title
		u.Followers = r.Subreddit.Subscribers
		u.NSFW = r.Subreddit.Over18
	}

	return u
}

func (r rawSubreddit) clean() Subreddit {
	icon := r.CommunityIcon
	if icon == "" {
		icon = r.IconImg
	}

	return Subreddit{
		ID:                r.ID,
		Name:              r.DisplayName,
		Title:             r.Title,
		PublicDescription: r.PublicDescription,
		Description:       r.Description,
		Subscribers:       r.Subscribers,
		ActiveUsers:       r.ActiveUserCount,
		Created:           timestamp(r.CreatedUTC),
		NSFW:              r.Over18,
		Type:              r.SubredditType,
		URL:               absolute(r.URL),
		Lang:              r.Lang,
		Subscribed:        r.UserIsSubscriber,
		Moderator:         r.UserIsModerator,
		Banned:            r.UserIsBanned,
		Icon:              icon,
	}
}
