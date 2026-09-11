package reddit

import "encoding/json"

const (
	kindComment   = "t1"
	kindPost      = "t3"
	kindMore      = "more"
	prefixComment = "t1_"
	prefixPost    = "t3_"
)

type thing struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}

type listing struct {
	Children []thing `json:"children"`
	After    string  `json:"after"`
	Before   string  `json:"before"`
}

type rawPost struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Title           string  `json:"title"`
	Author          string  `json:"author"`
	Subreddit       string  `json:"subreddit"`
	Score           int     `json:"score"`
	UpvoteRatio     float64 `json:"upvote_ratio"`
	NumComments     int     `json:"num_comments"`
	CreatedUTC      float64 `json:"created_utc"`
	URL             string  `json:"url"`
	Permalink       string  `json:"permalink"`
	Selftext        string  `json:"selftext"`
	IsSelf          bool    `json:"is_self"`
	Over18          bool    `json:"over_18"`
	Spoiler         bool    `json:"spoiler"`
	Stickied        bool    `json:"stickied"`
	Locked          bool    `json:"locked"`
	Archived        bool    `json:"archived"`
	Domain          string  `json:"domain"`
	LinkFlairText   string  `json:"link_flair_text"`
	AuthorFlairText string  `json:"author_flair_text"`
	Distinguished   string  `json:"distinguished"`
	PostHint        string  `json:"post_hint"`
	Edited          any     `json:"edited"`
	Likes           *bool   `json:"likes"`
	Saved           bool    `json:"saved"`
	CrosspostParent string  `json:"crosspost_parent"`
}

type rawComment struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Author          string          `json:"author"`
	Body            string          `json:"body"`
	Score           int             `json:"score"`
	ScoreHidden     bool            `json:"score_hidden"`
	CreatedUTC      float64         `json:"created_utc"`
	ParentID        string          `json:"parent_id"`
	LinkID          string          `json:"link_id"`
	Depth           int             `json:"depth"`
	Permalink       string          `json:"permalink"`
	IsSubmitter     bool            `json:"is_submitter"`
	Stickied        bool            `json:"stickied"`
	Distinguished   string          `json:"distinguished"`
	Edited          any             `json:"edited"`
	AuthorFlairText string          `json:"author_flair_text"`
	Subreddit       string          `json:"subreddit"`
	LinkTitle       string          `json:"link_title"`
	Likes           *bool           `json:"likes"`
	Saved           bool            `json:"saved"`
	Replies         json.RawMessage `json:"replies"`
}

type rawMore struct {
	Count    int      `json:"count"`
	Name     string   `json:"name"`
	ID       string   `json:"id"`
	ParentID string   `json:"parent_id"`
	Depth    int      `json:"depth"`
	Children []string `json:"children"`
}

type rawUserSubreddit struct {
	Title             string `json:"title"`
	PublicDescription string `json:"public_description"`
	Subscribers       int    `json:"subscribers"`
	Over18            bool   `json:"over_18"`
}

type rawUser struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	CreatedUTC       float64           `json:"created_utc"`
	LinkKarma        int               `json:"link_karma"`
	CommentKarma     int               `json:"comment_karma"`
	TotalKarma       int               `json:"total_karma"`
	AwardeeKarma     int               `json:"awardee_karma"`
	AwarderKarma     int               `json:"awarder_karma"`
	IsGold           bool              `json:"is_gold"`
	IsMod            bool              `json:"is_mod"`
	IsEmployee       bool              `json:"is_employee"`
	Verified         bool              `json:"verified"`
	HasVerifiedEmail bool              `json:"has_verified_email"`
	IconImg          string            `json:"icon_img"`
	IsFriend         bool              `json:"is_friend"`
	IsBlocked        bool              `json:"is_blocked"`
	IsSuspended      bool              `json:"is_suspended"`
	InboxCount       int               `json:"inbox_count"`
	HasMail          bool              `json:"has_mail"`
	Subreddit        *rawUserSubreddit `json:"subreddit"`
}

type rawSubreddit struct {
	ID                string  `json:"id"`
	DisplayName       string  `json:"display_name"`
	Title             string  `json:"title"`
	PublicDescription string  `json:"public_description"`
	Description       string  `json:"description"`
	Subscribers       int     `json:"subscribers"`
	ActiveUserCount   int     `json:"active_user_count"`
	CreatedUTC        float64 `json:"created_utc"`
	Over18            bool    `json:"over18"`
	SubredditType     string  `json:"subreddit_type"`
	URL               string  `json:"url"`
	UserIsSubscriber  bool    `json:"user_is_subscriber"`
	UserIsModerator   bool    `json:"user_is_moderator"`
	UserIsBanned      bool    `json:"user_is_banned"`
	IconImg           string  `json:"icon_img"`
	CommunityIcon     string  `json:"community_icon"`
	Lang              string  `json:"lang"`
}

type rawMoreChildren struct {
	JSON struct {
		Errors [][]string `json:"errors"`
		Data   struct {
			Things []thing `json:"things"`
		} `json:"data"`
	} `json:"json"`
}
