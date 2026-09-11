package server

type rawOpt struct {
	Raw bool `json:"raw,omitempty" jsonschema:"Return Reddit's raw JSON instead of the cleaned form"`
}

type pageOpts struct {
	Limit int    `json:"limit,omitempty" jsonschema:"1-100, default 25"`
	After string `json:"after,omitempty" jsonschema:"Pagination cursor from a previous response"`
}

type sortOpts struct {
	Sort string `json:"sort,omitempty" jsonschema:"Listing sort order"`
	Time string `json:"time,omitempty" jsonschema:"For top/controversial: hour, day, week, month, year, all"`
}

type emptyIn struct{}

type userIn struct {
	Username string `json:"username" jsonschema:"Reddit username without the u/ prefix"`
	rawOpt
}

type postIn struct {
	Post string `json:"post" jsonschema:"Post URL, t3_ fullname, or bare post id"`
	rawOpt
}

type commentsIn struct {
	Post    string `json:"post" jsonschema:"Post URL, t3_ fullname, or bare post id"`
	Sort    string `json:"sort,omitempty" jsonschema:"confidence (best), top, new, controversial, old, qa"`
	Limit   int    `json:"limit,omitempty" jsonschema:"Max top-level comments to return"`
	Depth   int    `json:"depth,omitempty" jsonschema:"Max reply depth"`
	Comment string `json:"comment,omitempty" jsonschema:"Focus on this comment id and its subtree"`
	Context int    `json:"context,omitempty" jsonschema:"With comment: number of parent levels to include (0-8)"`
	rawOpt
}

type moreIn struct {
	Post string   `json:"post" jsonschema:"Post URL, t3_ fullname, or bare post id"`
	IDs  []string `json:"ids" jsonschema:"Comment ids from a more_ids field (max 100)"`
	Sort string   `json:"sort,omitempty" jsonschema:"confidence, top, new, controversial, old, qa"`
	rawOpt
}

type userListIn struct {
	Username string `json:"username,omitempty" jsonschema:"Username; omit or use 'me' for the logged-in account"`
	sortOpts
	pageOpts
	rawOpt
}

type savedIn struct {
	pageOpts
	rawOpt
}

type subListIn struct {
	Subreddit string `json:"subreddit,omitempty" jsonschema:"Subreddit name without r/; omit for the logged-in home feed"`
	sortOpts
	pageOpts
	rawOpt
}

type subIn struct {
	Subreddit string `json:"subreddit" jsonschema:"Subreddit name without r/"`
	rawOpt
}

type searchIn struct {
	Query     string `json:"query" jsonschema:"Search query (Reddit search syntax supported)"`
	Subreddit string `json:"subreddit,omitempty" jsonschema:"Restrict to this subreddit"`
	NSFW      bool   `json:"nsfw,omitempty" jsonschema:"Include over-18 results"`
	sortOpts
	pageOpts
	rawOpt
}

type submitIn struct {
	Subreddit string `json:"subreddit" jsonschema:"Subreddit to post in, without the r/ prefix"`
	Title     string `json:"title" jsonschema:"Post title"`
	Text      string `json:"text,omitempty" jsonschema:"Body text for a self post. Markdown. Provide either text or url"`
	URL       string `json:"url,omitempty" jsonschema:"Link target for a link post. Provide either text or url"`
	NSFW      bool   `json:"nsfw,omitempty" jsonschema:"Mark the post as over 18"`
	Spoiler   bool   `json:"spoiler,omitempty" jsonschema:"Mark the post as a spoiler"`
	FlairID   string `json:"flair_id,omitempty" jsonschema:"Flair template id, if the subreddit requires flair"`
	FlairText string `json:"flair_text,omitempty" jsonschema:"Flair text, when the template allows editing it"`
	NoReplies bool   `json:"no_reply_notifications,omitempty" jsonschema:"Turn off inbox notifications for replies"`
	rawOpt
}

type replyIn struct {
	Parent string `json:"parent" jsonschema:"What to reply to: a post URL, a comment URL, a post id, or a t1_/t3_ fullname"`
	Text   string `json:"text" jsonschema:"Comment body. Markdown"`
	rawOpt
}

type editIn struct {
	Target string `json:"target" jsonschema:"What to edit: a post URL, a comment URL, a post id, or a t1_/t3_ fullname"`
	Text   string `json:"text" jsonschema:"Replacement body. Markdown"`
	rawOpt
}

type deleteIn struct {
	Target string `json:"target" jsonschema:"What to delete: a post URL, a comment URL, a post id, or a t1_/t3_ fullname"`
	rawOpt
}

type loginIn struct {
	TimeoutSeconds int  `json:"timeout_seconds,omitempty" jsonschema:"Seconds to wait for the login to finish (default 300)"`
	Force          bool `json:"force,omitempty" jsonschema:"Open the browser even when the saved session still works"`
}
