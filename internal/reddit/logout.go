package reddit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	http "github.com/bogdanfinn/fhttp"
)

const (
	oldBase    = "https://old.reddit.com"
	pathOldMe  = "/api/me.json"
	pathLogout = "/logout"
	paramUH    = "uh"
	paramTop   = "top"
)

var errNoModhash = errors.New("no modhash returned; session is not logged in")

func (c *Client) Logout(ctx context.Context) error {
	modhash, err := c.modhash(ctx)
	if err != nil {
		return err
	}

	form := url.Values{paramUH: {modhash}, paramTop: {"off"}}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oldBase+pathLogout, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}

	req.Header = c.buildHeaders(c.docSource(), map[string]string{
		hContentType: formContentType,
		hOrigin:      oldBase,
		hReferer:     oldBase + "/",
		hFetchDest:   "document",
		hFetchMode:   "navigate",
		hFetchSite:   "same-origin",
		hFetchUser:   "?1",
	})

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("logout request: %w", err)
	}
	defer resp.Body.Close()

	io.Copy(io.Discard, io.LimitReader(resp.Body, maxDiscard))

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusOK {
		return &APIError{resp.StatusCode, ""}
	}

	return nil
}

func (c *Client) modhash(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, oldBase+pathOldMe, nil)
	if err != nil {
		return "", err
	}

	req.Header = c.buildHeaders(c.docSource(), map[string]string{
		hFetchDest: "document",
		hFetchMode: "navigate",
		hFetchSite: "none",
		hFetchUser: "?1",
	})

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := readBody(resp)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", &APIError{resp.StatusCode, string(body)}
	}

	var me struct {
		Data struct {
			Modhash string `json:"modhash"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &me); err != nil {
		return "", fmt.Errorf("decode %s: %w", pathOldMe, err)
	}
	if me.Data.Modhash == "" {
		return "", errNoModhash
	}

	return me.Data.Modhash, nil
}
