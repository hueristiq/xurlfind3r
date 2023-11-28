package wayback

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/hueristiq/hqgohttp/headers"
	"github.com/hueristiq/hqgolimit"
	"github.com/hueristiq/xurlfind3r/pkg/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/scraper/sources"
	"github.com/spf13/cast"
)

type Source struct{}

var limiter = hqgolimit.New(&hqgolimit.Options{
	RequestsPerMinute: 40,
})

func (source *Source) Run(config *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		var err error

		getPagesReqURL := formatURL(domain, config.IncludeSubdomains) + "&showNumPages=true"

		limiter.Wait()

		var getPagesRes *http.Response

		getPagesRes, err = httpclient.SimpleGet(getPagesReqURL)
		if err != nil {
			result := sources.Result{
				Type:   sources.Error,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			httpclient.DiscardResponse(getPagesRes)

			return
		}

		var pages uint

		if err = json.NewDecoder(getPagesRes.Body).Decode(&pages); err != nil {
			result := sources.Result{
				Type:   sources.Error,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			getPagesRes.Body.Close()

			return
		}

		getPagesRes.Body.Close()

		waybackURLs := [][]string{}

		for page := uint(0); page < pages; page++ {
			getURLsReqURL := fmt.Sprintf("%s&page=%d", formatURL(domain, config.IncludeSubdomains), page)

			limiter.Wait()

			var getURLsRes *http.Response

			getURLsRes, err = httpclient.SimpleGet(getURLsReqURL)
			if err != nil {
				result := sources.Result{
					Type:   sources.Error,
					Source: source.Name(),
					Error:  err,
				}

				results <- result

				httpclient.DiscardResponse(getURLsRes)

				return
			}

			var getURLsResData [][]string

			if err = json.NewDecoder(getURLsRes.Body).Decode(&getURLsResData); err != nil {
				result := sources.Result{
					Type:   sources.Error,
					Source: source.Name(),
					Error:  err,
				}

				results <- result

				getURLsRes.Body.Close()

				return
			}

			getURLsRes.Body.Close()

			// check if there's results, wayback's pagination response
			// is not always correct when using a filter
			if len(getURLsResData) == 0 {
				break
			}

			// output results
			// Slicing as [1:] to skip first result by default
			waybackURLs = append(waybackURLs, getURLsResData[1:]...)
		}

		mediaURLRegex := regexp.MustCompile(`(?i)\.(apng|bpm|png|bmp|gif|heif|ico|cur|jpg|jpeg|jfif|pjp|pjpeg|psd|raw|svg|tif|tiff|webp|xbm|3gp|aac|flac|mpg|mpeg|mp3|mp4|m4a|m4v|m4p|oga|ogg|ogv|mov|wav|webm|eot|woff|woff2|ttf|otf|pdf)(?:\?|#|$)`)
		robotsURLsRegex := regexp.MustCompile(`^(https?)://[^ "]+/robots.txt$`)

		for _, waybackURL := range waybackURLs {
			URL := waybackURL[1]

			if !sources.IsInScope(URL, domain, config.IncludeSubdomains) {
				return
			}

			result := sources.Result{
				Type:   sources.URL,
				Source: source.Name(),
				Value:  URL,
			}

			results <- result

			if mediaURLRegex.MatchString(URL) {
				return
			}

			if config.ParseWaybackRobots && robotsURLsRegex.MatchString(URL) {
				parseWaybackRobots(config, URL, results)

				return
			}

			if config.ParseWaybackSource {
				parseWaybackSource(config, domain, URL, results)
			}
		}
	}()

	return results
}

func formatURL(domain string, includeSubdomains bool) (URL string) {
	if includeSubdomains {
		domain = "*." + domain
	}

	URL = fmt.Sprintf("http://web.archive.org/cdx/search/cdx?url=%s/*&output=json&collapse=urlkey&fl=timestamp,original,mimetype,statuscode,digest", domain)

	return
}

func getSnapshots(URL string) (snapshots [][2]string, err error) {
	getSnapshotsReqURL := fmt.Sprintf("https://web.archive.org/cdx/search/cdx?url=%s&output=json&fl=timestamp,original&collapse=digest", URL)

	var getSnapshotsRes *http.Response

	limiter.Wait()

	getSnapshotsRes, err = httpclient.SimpleGet(getSnapshotsReqURL)
	if err != nil {
		return
	}

	if cast.ToInt(getSnapshotsRes.Header.Get(headers.ContentLength)) == 0 {
		return
	}

	if err = json.NewDecoder(getSnapshotsRes.Body).Decode(&snapshots); err != nil {
		getSnapshotsRes.Body.Close()

		return
	}

	getSnapshotsRes.Body.Close()

	if len(snapshots) < 2 {
		return
	}

	snapshots = snapshots[1:]

	return
}

func getSnapshotContent(snapshot [2]string) (content string, err error) {
	var (
		timestamp = snapshot[0]
		URL       = snapshot[1]
	)

	getSnapshotContentReqURL := fmt.Sprintf("https://web.archive.org/web/%sif_/%s", timestamp, URL)

	limiter.Wait()

	var getSnapshotContentRes *http.Response

	getSnapshotContentRes, err = httpclient.SimpleGet(getSnapshotContentReqURL)
	if err != nil {
		return
	}

	content = cast.ToString(getSnapshotContentRes.Body)

	if content == "" {
		getSnapshotContentRes.Body.Close()

		return
	}

	getSnapshotContentRes.Body.Close()

	snapshotNotFoundFingerprint := "This page can't be displayed. Please use the correct URL address to access"

	if strings.Contains(content, snapshotNotFoundFingerprint) {
		err = fmt.Errorf("%s", snapshotNotFoundFingerprint)

		return
	}

	return
}

func (source *Source) Name() string {
	return "wayback"
}
