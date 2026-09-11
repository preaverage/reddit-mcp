package reddit

import "encoding/json"

type Result[T any] struct {
	Data T
	Raw  json.RawMessage
}

type Page[T any] struct {
	Items []T    `json:"items"`
	After string `json:"after,omitempty"`
}

type Post struct {
	ID            string  `json:"id"`
	Fullname      string  `json:"fullname"`
	Title         string  `json:"title"`
	Author        string  `json:"author"`
	Subreddit     string  `json:"subreddit"`
	Score         int     `json:"score"`
	UpvoteRatio   float64 `json:"upvote_ratio"`
	NumComments   int     `json:"num_comments"`
	Created       string  `json:"created"`
	URL           string  `json:"url"`
	Permalink     string  `json:"permalink"`
	Selftext      string  `json:"selftext,omitempty"`
	IsSelf        bool    `json:"is_self"`
	NSFW          bool    `json:"nsfw"`
	Spoiler       bool    `json:"spoiler,omitempty"`
	Stickied      bool    `json:"stickied,omitempty"`
	Locked        bool    `json:"locked,omitempty"`
	Archived      bool    `json:"archived,omitempty"`
	Domain        string  `json:"domain,omitempty"`
	Flair         string  `json:"flair,omitempty"`
	AuthorFlair   string  `json:"author_flair,omitempty"`
	Distinguished string  `json:"distinguished,omitempty"`
	PostHint      string  `json:"post_hint,omitempty"`
	Edited        bool    `json:"edited,omitempty"`
	MyVote        string  `json:"my_vote,omitempty"`
	Saved         bool    `json:"saved,omitempty"`
	Crosspost     string  `json:"crosspost_parent,omitempty"`
}

type Comment struct {
	ID            string    `json:"id"`
	Fullname      string    `json:"fullname"`
	Author        string    `json:"author"`
	Body          string    `json:"body"`
	Score         int       `json:"score"`
	ScoreHidden   bool      `json:"score_hidden,omitempty"`
	Created       string    `json:"created"`
	Depth         int       `json:"depth"`
	ParentID      string    `json:"parent_id"`
	LinkID        string    `json:"link_id,omitempty"`
	Subreddit     string    `json:"subreddit,omitempty"`
	LinkTitle     string    `json:"link_title,omitempty"`
	Permalink     string    `json:"permalink"`
	IsOP          bool      `json:"is_op,omitempty"`
	Stickied      bool      `json:"stickied,omitempty"`
	Distinguished string    `json:"distinguished,omitempty"`
	Edited        bool      `json:"edited,omitempty"`
	AuthorFlair   string    `json:"author_flair,omitempty"`
	MyVote        string    `json:"my_vote,omitempty"`
	Saved         bool      `json:"saved,omitempty"`
	Replies       []Comment `json:"replies,omitempty"`
	MoreReplies   int       `json:"more_replies,omitempty"`
	MoreIDs       []string  `json:"more_ids,omitempty"`
}

type More struct {
	ParentID string   `json:"parent_id"`
	Count    int      `json:"count"`
	IDs      []string `json:"ids"`
}

type Thread struct {
	Post     Post      `json:"post"`
	Comments []Comment `json:"comments"`
	More     *More     `json:"more,omitempty"`
}

type User struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Created          string `json:"created"`
	LinkKarma        int    `json:"link_karma"`
	CommentKarma     int    `json:"comment_karma"`
	TotalKarma       int    `json:"total_karma"`
	AwardeeKarma     int    `json:"awardee_karma,omitempty"`
	AwarderKarma     int    `json:"awarder_karma,omitempty"`
	Description      string `json:"description,omitempty"`
	DisplayTitle     string `json:"display_title,omitempty"`
	Followers        int    `json:"followers,omitempty"`
	IconImg          string `json:"icon_img,omitempty"`
	IsGold           bool   `json:"is_gold,omitempty"`
	IsMod            bool   `json:"is_mod,omitempty"`
	IsEmployee       bool   `json:"is_employee,omitempty"`
	Verified         bool   `json:"verified,omitempty"`
	HasVerifiedEmail bool   `json:"has_verified_email,omitempty"`
	NSFW             bool   `json:"nsfw,omitempty"`
	IsFriend         bool   `json:"is_friend,omitempty"`
	IsBlocked        bool   `json:"is_blocked,omitempty"`
	IsSuspended      bool   `json:"is_suspended,omitempty"`
	InboxCount       int    `json:"inbox_count,omitempty"`
	HasMail          bool   `json:"has_mail,omitempty"`
}

type Subreddit struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Title             string `json:"title"`
	PublicDescription string `json:"public_description,omitempty"`
	Description       string `json:"description,omitempty"`
	Subscribers       int    `json:"subscribers"`
	ActiveUsers       int    `json:"active_users"`
	Created           string `json:"created"`
	NSFW              bool   `json:"nsfw"`
	Type              string `json:"type"`
	URL               string `json:"url"`
	Lang              string `json:"lang,omitempty"`
	Subscribed        bool   `json:"subscribed"`
	Moderator         bool   `json:"moderator,omitempty"`
	Banned            bool   `json:"banned,omitempty"`
	Icon              string `json:"icon,omitempty"`
}

type CommentsOptions struct {
	Sort    string
	Limit   int
	Depth   int
	Comment string
	Context int
}

type ListOptions struct {
	Sort  string
	Time  string
	Limit int
	After string
}

type SearchOptions struct {
	Subreddit string
	Sort      string
	Time      string
	Limit     int
	After     string
	NSFW      bool
}
