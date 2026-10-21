package reddit

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	http "github.com/bogdanfinn/fhttp"
)

// Reddit chat runs on a Matrix homeserver that accepts the web app's bearer
// token. Direct chats are ordinary Matrix rooms marked with a
// com.reddit.chat.type state event naming both participants.
const (
	matrixBase   = "https://matrix.redditspace.com/_matrix/client/v3"
	matrixServer = ":reddit.com"
	prefixMatrix = "@"
	prefixUser   = "t2_"

	evMessage  = "m.room.message"
	evChatType = "com.reddit.chat.type"
	msgText    = "m.text"
	chatDirect = "direct"

	pathUsersByID = "/api/user_data_by_account_ids"

	syncFilter = `{"room":{"timeline":{"limit":1,"types":["m.room.message"]},"state":{"types":["com.reddit.chat.type"]}},"presence":{"types":[]},"account_data":{"types":[]}}`
)

// Chat is one direct conversation, with its latest message.
type Chat struct {
	Room     string `json:"room"`
	With     string `json:"with"`
	Request  bool   `json:"request,omitempty"`
	LastFrom string `json:"last_from,omitempty"`
	LastText string `json:"last_text,omitempty"`
	LastAt   string `json:"last_at,omitempty"`
}

type ChatMessage struct {
	ID   string `json:"id"`
	From string `json:"from"`
	Text string `json:"text"`
	At   string `json:"at"`
}

type ChatHistory struct {
	Room     string        `json:"room"`
	With     string        `json:"with,omitempty"`
	Messages []ChatMessage `json:"messages"`
	Before   string        `json:"before,omitempty"`
}

type SentChat struct {
	Room    string `json:"room"`
	Event   string `json:"event"`
	To      string `json:"to"`
	Created bool   `json:"created_room,omitempty"`
}

type matrixEvent struct {
	Type     string          `json:"type"`
	Sender   string          `json:"sender"`
	StateKey *string         `json:"state_key"`
	EventID  string          `json:"event_id"`
	TS       int64           `json:"origin_server_ts"`
	Content  json.RawMessage `json:"content"`
}

type messageContent struct {
	MsgType string `json:"msgtype"`
	Body    string `json:"body"`
}

type chatTypeContent struct {
	Type         string   `json:"type"`
	Participants []string `json:"participants"`
}

type syncRoom struct {
	Summary struct {
		Heroes []string `json:"m.heroes"`
	} `json:"summary"`
	State struct {
		Events []matrixEvent `json:"events"`
	} `json:"state"`
	InviteState struct {
		Events []matrixEvent `json:"events"`
	} `json:"invite_state"`
	Timeline struct {
		Events []matrixEvent `json:"events"`
	} `json:"timeline"`
}

func (c *Client) matrix(ctx context.Context, method, path string, body any) ([]byte, error) {
	tok, err := c.validToken(ctx)
	if err != nil {
		return nil, err
	}

	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}

	full := matrixBase + path

	var req *http.Request
	if reader != nil {
		req, err = http.NewRequestWithContext(ctx, method, full, reader)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, full, nil)
	}
	if err != nil {
		return nil, err
	}

	req.Header = c.apiHeaders(tok, false)
	if body != nil {
		req.Header.Set(hContentType, "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", path, err)
	}
	defer resp.Body.Close()

	out, err := readBody(resp)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{resp.StatusCode, string(out)}
	}

	return out, nil
}

func (c *Client) matrixJSON(ctx context.Context, method, path string, body, v any) error {
	out, err := c.matrix(ctx, method, path, body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(out, v); err != nil {
		return fmt.Errorf("decode %s: %w (body starts: %.120s)", path, err, out)
	}

	return nil
}

func matrixID(accountID string) string {
	return prefixMatrix + prefixUser + strings.TrimPrefix(accountID, prefixUser) + matrixServer
}

func accountID(mxid string) string {
	return strings.TrimSuffix(strings.TrimPrefix(mxid, prefixMatrix), matrixServer)
}

func (c *Client) myMatrixID(ctx context.Context) (string, error) {
	var who struct {
		UserID string `json:"user_id"`
	}

	if err := c.matrixJSON(ctx, http.MethodGet, "/account/whoami", nil, &who); err != nil {
		return "", err
	}

	return who.UserID, nil
}

// usernames maps t2_ account ids to usernames in one call.
func (c *Client) usernames(ctx context.Context, ids []string) map[string]string {
	names := map[string]string{}
	if len(ids) == 0 {
		return names
	}

	var data map[string]struct {
		Name string `json:"name"`
	}

	for start := 0; start < len(ids); start += 100 {
		end := min(start+100, len(ids))

		out, err := c.Get(ctx, pathUsersByID, url.Values{"ids": {strings.Join(ids[start:end], ",")}})
		if err != nil || json.Unmarshal(out, &data) != nil {
			continue
		}

		for id, u := range data {
			names[id] = u.Name
		}
	}

	return names
}

func (c *Client) userMatrixID(ctx context.Context, username string) (string, string, error) {
	u, err := c.User(ctx, username)
	if err != nil {
		return "", "", fmt.Errorf("look up u/%s: %w", trimUser(username), err)
	}

	return matrixID(u.Data.ID), u.Data.Name, nil
}

func (c *Client) directRooms(ctx context.Context, me string) (map[string][]string, error) {
	var direct map[string][]string

	err := c.matrixJSON(ctx, http.MethodGet, "/user/"+url.PathEscape(me)+"/account_data/m.direct", nil, &direct)

	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return map[string][]string{}, nil
	}

	return direct, err
}

func formatTS(ms int64) string {
	if ms == 0 {
		return ""
	}

	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

func (e matrixEvent) text() (string, bool) {
	if e.Type != evMessage {
		return "", false
	}

	var m messageContent
	if json.Unmarshal(e.Content, &m) != nil || m.Body == "" {
		return "", false
	}

	return m.Body, true
}

// lastMessage fetches a room's newest text message.
func (c *Client) lastMessage(ctx context.Context, room string) (matrixEvent, bool) {
	q := url.Values{"dir": {"b"}, "limit": {"1"}, "filter": {`{"types":["m.room.message"]}`}}

	var page struct {
		Chunk []matrixEvent `json:"chunk"`
	}

	if c.matrixJSON(ctx, http.MethodGet, "/rooms/"+url.PathEscape(room)+"/messages?"+q.Encode(), nil, &page) != nil {
		return matrixEvent{}, false
	}

	for _, e := range page.Chunk {
		if _, ok := e.text(); ok {
			return e, true
		}
	}

	return matrixEvent{}, false
}

// Chats lists direct chats, newest first, including chat requests waiting
// to be accepted. Sync only returns the most active rooms, so the full list
// comes from m.direct and any room sync left out is read on its own.
func (c *Client) Chats(ctx context.Context, limit int) (Result[[]Chat], error) {
	me, err := c.myMatrixID(ctx)
	if err != nil {
		return Result[[]Chat]{}, err
	}

	direct, err := c.directRooms(ctx, me)
	if err != nil {
		return Result[[]Chat]{}, err
	}

	var s struct {
		Rooms struct {
			Join   map[string]syncRoom `json:"join"`
			Invite map[string]syncRoom `json:"invite"`
		} `json:"rooms"`
	}

	q := url.Values{"timeout": {"0"}, "filter": {syncFilter}}
	if err := c.matrixJSON(ctx, http.MethodGet, "/sync?"+q.Encode(), nil, &s); err != nil {
		return Result[[]Chat]{}, err
	}

	type row struct {
		Chat
		with string
		last matrixEvent
		seen bool
	}

	rows := map[string]*row{}
	get := func(id string) *row {
		if rows[id] == nil {
			rows[id] = &row{Chat: Chat{Room: id}}
		}
		return rows[id]
	}

	for other, rooms := range direct {
		for _, id := range rooms {
			get(id).with = other
		}
	}

	scan := func(id string, r syncRoom, request bool) {
		x := get(id)
		x.Request = request

		events := append(append(r.State.Events, r.InviteState.Events...), r.Timeline.Events...)
		for _, e := range events {
			if e.Type == evChatType && x.with == "" {
				var ct chatTypeContent
				if json.Unmarshal(e.Content, &ct) == nil {
					for _, p := range ct.Participants {
						if p != me {
							x.with = p
						}
					}
				}
			}

			if _, ok := e.text(); ok && e.TS >= x.last.TS {
				x.last, x.seen = e, true
			}

			if request && x.with == "" && e.Sender != me {
				x.with = e.Sender
			}
		}

		for _, h := range r.Summary.Heroes {
			if x.with == "" && h != me {
				x.with = h
			}
		}
	}

	for id, r := range s.Rooms.Join {
		scan(id, r, false)
	}
	for id, r := range s.Rooms.Invite {
		scan(id, r, true)
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	for _, x := range rows {
		if x.seen || x.Request {
			continue
		}

		wg.Add(1)
		go func(x *row) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			x.last, x.seen = c.lastMessage(ctx, x.Room)
		}(x)
	}
	wg.Wait()

	list := make([]*row, 0, len(rows))
	ids := map[string]bool{}

	for _, x := range rows {
		list = append(list, x)

		for _, m := range []string{x.with, x.last.Sender} {
			if m != "" {
				ids[accountID(m)] = true
			}
		}
	}

	sort.Slice(list, func(i, j int) bool { return list[i].last.TS > list[j].last.TS })

	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}

	idList := make([]string, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	names := c.usernames(ctx, idList)

	name := func(mxid string) string {
		if mxid == me {
			return "me"
		}
		if n := names[accountID(mxid)]; n != "" {
			return n
		}
		return accountID(mxid)
	}

	chats := make([]Chat, 0, len(list))
	for _, x := range list {
		ch := x.Chat
		if x.with != "" {
			ch.With = name(x.with)
		}
		if x.seen {
			ch.LastFrom = name(x.last.Sender)
			ch.LastText, _ = x.last.text()
			ch.LastAt = formatTS(x.last.TS)
		}
		chats = append(chats, ch)
	}

	return Result[[]Chat]{Data: chats}, nil
}

// resolveRoom accepts a room id (!...) or a username and returns the room,
// plus the other person's matrix id when it was found by name.
func (c *Client) resolveRoom(ctx context.Context, me, target string) (string, string, error) {
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "!") {
		return target, "", nil
	}

	other, _, err := c.userMatrixID(ctx, target)
	if err != nil {
		return "", "", err
	}

	direct, err := c.directRooms(ctx, me)
	if err != nil {
		return "", "", err
	}

	if rooms := direct[other]; len(rooms) > 0 {
		return rooms[len(rooms)-1], other, nil
	}

	return "", other, nil
}

// ChatMessages returns a chat's messages, oldest first. before is the cursor
// from a previous call, for older messages.
func (c *Client) ChatMessages(ctx context.Context, target string, limit int, before string) (Result[ChatHistory], error) {
	me, err := c.myMatrixID(ctx)
	if err != nil {
		return Result[ChatHistory]{}, err
	}

	room, other, err := c.resolveRoom(ctx, me, target)
	if err != nil {
		return Result[ChatHistory]{}, err
	}
	if room == "" {
		return Result[ChatHistory]{}, fmt.Errorf("no chat with u/%s yet", trimUser(target))
	}

	q := url.Values{"dir": {"b"}, "limit": {clampLimit(limit)}, "filter": {`{"types":["m.room.message"]}`}}
	setIf(q, "from", before)

	var page struct {
		Chunk []matrixEvent `json:"chunk"`
		End   string        `json:"end"`
	}

	raw, err := c.matrix(ctx, http.MethodGet, "/rooms/"+url.PathEscape(room)+"/messages?"+q.Encode(), nil)
	if err != nil {
		return Result[ChatHistory]{}, err
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		return Result[ChatHistory]{}, fmt.Errorf("decode messages: %w", err)
	}

	ids := map[string]bool{}
	for _, e := range page.Chunk {
		ids[accountID(e.Sender)] = true
	}
	list := make([]string, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	h := ChatHistory{Room: room, Before: page.End}
	if other != "" {
		list = append(list, accountID(other))
	}
	names := c.usernames(ctx, list)
	h.With = names[accountID(other)]

	for i := len(page.Chunk) - 1; i >= 0; i-- {
		e := page.Chunk[i]

		body, ok := e.text()
		if !ok {
			continue
		}

		from := names[accountID(e.Sender)]
		switch {
		case e.Sender == me:
			from = "me"
		case from == "":
			from = accountID(e.Sender)
		default:
			h.With = from
		}

		h.Messages = append(h.Messages, ChatMessage{ID: e.EventID, From: from, Text: body, At: formatTS(e.TS)})
	}

	return Result[ChatHistory]{Data: h, Raw: raw}, nil
}

func txnID() string {
	b := make([]byte, 16)
	rand.Read(b)

	return strings.ToUpper(hex.EncodeToString(b))
}

// createDirect opens a direct chat the way Reddit's own clients set one up:
// invite-only, shared history, and the com.reddit.chat.type marker.
func (c *Client) createDirect(ctx context.Context, me, other string) (string, error) {
	body := map[string]any{
		"preset":    "trusted_private_chat",
		"is_direct": true,
		"invite":    []string{other},
		"initial_state": []map[string]any{{
			"type":      evChatType,
			"state_key": "",
			"content":   chatTypeContent{Type: chatDirect, Participants: []string{me, other}},
		}},
	}

	var resp struct {
		RoomID string `json:"room_id"`
	}

	if err := c.matrixJSON(ctx, http.MethodPost, "/createRoom", body, &resp); err != nil {
		return "", fmt.Errorf("create chat: %w", err)
	}

	return resp.RoomID, nil
}

// SendChat sends a direct chat message to a username or an existing room,
// starting a new chat when there is none with that user yet.
func (c *Client) SendChat(ctx context.Context, target, text string) (Result[SentChat], error) {
	if strings.TrimSpace(text) == "" {
		return Result[SentChat]{}, fmt.Errorf("text is required")
	}

	me, err := c.myMatrixID(ctx)
	if err != nil {
		return Result[SentChat]{}, err
	}

	room, other, err := c.resolveRoom(ctx, me, target)
	if err != nil {
		return Result[SentChat]{}, err
	}

	sent := SentChat{To: trimUser(target)}

	if room == "" {
		if room, err = c.createDirect(ctx, me, other); err != nil {
			return Result[SentChat]{}, err
		}
		sent.Created = true
	}

	var resp struct {
		EventID string `json:"event_id"`
	}

	path := "/rooms/" + url.PathEscape(room) + "/send/" + evMessage + "/" + txnID()
	if err := c.matrixJSON(ctx, http.MethodPut, path, messageContent{MsgType: msgText, Body: text}, &resp); err != nil {
		return Result[SentChat]{}, fmt.Errorf("send chat message: %w", err)
	}

	sent.Room, sent.Event = room, resp.EventID

	return Result[SentChat]{Data: sent}, nil
}
