package otx

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cast"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type getURLsResponse struct {
	URLList []struct {
		URL      string `json:"url"`
		Domain   string `json:"domain"`
		Hostname string `json:"hostname"`
		Result   struct {
			URLWorker struct {
				IP       string `json:"ip"`
				HTTPCode int    `json:"http_code"`
			} `json:"urlworker"`
		} `json:"result"`
		HTTPCode int    `json:"httpcode"`
		Encoded  string `json:"encoded"`
	} `json:"url_list"`
	PageNum    int  `json:"page_num"`
	Limit      int  `json:"limit"`
	Paged      bool `json:"paged"`
	HasNext    bool `json:"has_next"`
	FullSize   int  `json:"full_size"`
	ActualSize int  `json:"actual_size"`
}

type Source struct {
	keys sources.Keys
}

func (s *Source) Name() (name string) {
	name = sources.OpenThreatExchange

	return
}

func (s *Source) NeedsKeys() (needs bool) {
	needs = true

	return
}

func (s *Source) UseKeys(keys ...string) {
	s.keys = append(s.keys, keys...)
}

func (s *Source) Run(ctx context.Context, cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		key := ""

		if picked, err := s.keys.PickRandom(); err == nil {
			key = picked
		}

		previousSignature := ""

		for page := 1; ; page++ {
			if page > sources.MaxPages {
				result := sources.WarningResult(s.Name(), fmt.Sprintf("pagination capped at %d pages", sources.MaxPages))

				results <- result

				break
			}

			getURLsReqURL := fmt.Sprintf("https://otx.alienvault.com/api/v1/indicators/domain/%s/url_list", domain)

			getURLsReq := cfg.Client.NewRequest().
				URL(getURLsReqURL).
				Param("limit", "100").
				Param("page", cast.ToString(page))

			if key != "" {
				getURLsReq = getURLsReq.SetHeader("X-OTX-API-KEY", key)
			}

			getURLsRes, err := getURLsReq.Do(ctx)
			if err != nil {
				result := sources.ErrorResult(s.Name(), fmt.Errorf("request failed: %w", err))

				results <- result

				return
			}

			if err = sources.CheckResponse(getURLsRes, key); err != nil {
				_ = getURLsRes.Body.Close()

				result := sources.ErrorResult(s.Name(), err)

				results <- result

				return
			}

			var getURLsResData getURLsResponse

			if err = json.NewDecoder(getURLsRes.Body).Decode(&getURLsResData); err != nil {
				result := sources.ErrorResult(s.Name(), fmt.Errorf("failed to parse JSON response: %w", err))

				results <- result

				_ = getURLsRes.Body.Close()

				return
			}

			_ = getURLsRes.Body.Close()

			signature := fmt.Sprintf("%d|%d", len(getURLsResData.URLList), getURLsResData.ActualSize)

			if len(getURLsResData.URLList) > 0 {
				signature += "|" + getURLsResData.URLList[0].URL + "|" + getURLsResData.URLList[len(getURLsResData.URLList)-1].URL
			}

			if signature == previousSignature {
				result := sources.WarningResult(s.Name(), "pagination stopped: page watermark did not advance")

				results <- result

				break
			}

			previousSignature = signature

			for _, item := range getURLsResData.URLList {
				URL, valid := cfg.Validator(item.URL)
				if !valid {
					continue
				}

				result := sources.URLResult(s.Name(), URL)

				results <- result
			}

			if !getURLsResData.HasNext {
				break
			}
		}
	}()

	return results
}

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{
		keys: make(sources.Keys, 0),
	}

	return
}
