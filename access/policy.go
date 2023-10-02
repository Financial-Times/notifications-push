package access

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const advancedNotificationsXPolicy = "ADVANCED_NOTIFICATIONS"

var xPoliciesPattern = regexp.MustCompile(`['"]x-policy['"]\s*:\s*['"](.*)?['"]`)

type NotificationSubscriptionOptions struct {
	ReceiveAdvancedNotifications bool
}

type PolicyProcessor struct {
	client *http.Client
	url    *url.URL
}

func NewPolicyProcessor(u *url.URL, c *http.Client) *PolicyProcessor {
	return &PolicyProcessor{
		url:    u,
		client: c,
	}
}

func (pp *PolicyProcessor) GetNotificationSubscriptionOptions(ctx context.Context, k string) (*NotificationSubscriptionOptions, error) {
	opts := &NotificationSubscriptionOptions{
		ReceiveAdvancedNotifications: false,
	}

	policies, err := pp.getXPolicies(ctx, k)
	if err != nil {
		return nil, err
	}

	for _, p := range policies {
		if p == advancedNotificationsXPolicy {
			opts.ReceiveAdvancedNotifications = true
			break
		}
	}

	return opts, nil
}

func (pp *PolicyProcessor) getXPolicies(ctx context.Context, k string) ([]string, error) {
	if k == "" {
		return nil, NewPolicyErr("Empty api key used to get X-Policies", http.StatusUnauthorized, "", "")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", pp.url.String(), nil)
	if err != nil {
		return nil, NewPolicyErr("Invalid X-Policies URL", http.StatusInternalServerError, "", err.Error())
	}

	req.Header.Set(apiKeyHeaderField, k)

	keySuffix := keySuffixLogging(k)

	resp, err := pp.client.Do(req)
	if err != nil {
		return nil, NewPolicyErr("Request to get X-Policies assigned to API key failed", http.StatusInternalServerError, keySuffix, err.Error())
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewPolicyErr("Reading X-Policies assigned to API Key failed", http.StatusInternalServerError, "", err.Error())
	}

	if resp.StatusCode != http.StatusOK {
		errMsg := "Request to get X-Policies assigned to API key returned an unexpected response"
		if resp.StatusCode == http.StatusNotFound {
			errMsg = "X-Policies assigned to API key not found"
		}

		return nil, NewPolicyErr(errMsg, resp.StatusCode, keySuffix, string(data))
	}

	matches := xPoliciesPattern.FindStringSubmatch(string(data))
	if matches == nil {
		return nil, NewPolicyErr("Decoding X-Policies assigned to API key failed", http.StatusInternalServerError, keySuffix, "")
	}

	policies := make([]string, 0)

	for _, xPolicy := range strings.Split(matches[1], ",") {
		xPolicy = strings.TrimSpace(xPolicy)
		if xPolicy != "" {
			policies = append(policies, xPolicy)
		}
	}

	return policies, nil
}

type PolicyErr struct {
	Msg         string
	Status      int
	KeySuffix   string
	Description string
}

func (e *PolicyErr) Error() string {
	return e.Msg
}

func NewPolicyErr(m string, s int, k string, d string) *PolicyErr {
	return &PolicyErr{
		Msg:         m,
		Status:      s,
		KeySuffix:   k,
		Description: d,
	}
}
