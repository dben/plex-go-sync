package plex

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jrudio/go-plex-client"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	plex.Plex
}

// New creates a new plex instance that is required
// to make requests to your Plex Media Server
func New(baseURL string, token string) (*Server, error) {
	server, err := plex.New(baseURL, token)
	return &Server{Plex: *server}, err
}

// GetPlaylistsByName GetPlaylists returns a list of results from the Plex server
func (p *Server) GetPlaylistsByName(title string) (plex.SearchResults, error) {
	args := make(map[string]string)
	args["title"] = title

	query := fmt.Sprintf("%s/playlists%s", p.URL, joinArgs(args))

	resp, err := p.http("GET", query)

	if err != nil {
		return plex.SearchResults{}, err
	}

	// Unauthorized
	if resp.Status == plex.ErrorInvalidToken {
		return plex.SearchResults{}, errors.New("invalid token")
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return plex.SearchResults{}, errors.New(plex.ErrorNotAuthorized)
	}
	if resp.StatusCode != http.StatusOK {
		return plex.SearchResults{}, fmt.Errorf(plex.ErrorServer, resp.Status)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer resp.Body.Close()

	var results plex.SearchResults

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return plex.SearchResults{}, err
	}

	return results, nil
}

func (p *Server) RefreshLibraries() chan string {
	results := make(chan string)
	query := fmt.Sprintf("%s/library/sections/all/refresh", p.URL)
	if _, err := p.http("GET", query); err != nil {
		close(results)
		return results
	}

	go func() {
		timeout := time.After(time.Hour * 1)
		ticker := time.NewTicker(time.Second * 30)
		defer ticker.Stop()
		defer close(results)
		libMap := make(map[string]bool)

		for {
			select {
			case <-ticker.C:
				refreshing := false
				libraries, err := p.GetLibraries()
				if err != nil {
					return
				}
				for _, library := range libraries.MediaContainer.Directory {
					if libMap[library.Key] && !library.Refreshing {
						log.Println("Library", library.Title, "is done refreshing")
						results <- library.Key
					}
					libMap[library.Key] = library.Refreshing
					refreshing = refreshing || library.Refreshing
				}
				if !refreshing {
					return
				}
			case <-timeout:
				return
			}
		}
	}()

	return results
}

func (p *Server) http(verb string, query string) (*http.Response, error) {
	client := p.HTTPClient

	log.Println(verb, query)
	req, reqErr := http.NewRequest(verb, query, nil)

	if reqErr != nil {
		return &http.Response{}, reqErr
	}

	req.Header.Add("Accept", p.Headers.Accept)
	req.Header.Add("X-Plex-Platform", p.Headers.Platform)
	req.Header.Add("X-Plex-Platform-Version", p.Headers.PlatformVersion)
	req.Header.Add("X-Plex-Provides", p.Headers.Provides)
	req.Header.Add("X-Plex-Client-Identifier", p.ClientIdentifier)
	req.Header.Add("X-Plex-Product", p.Headers.Product)
	req.Header.Add("X-Plex-Version", p.Headers.Version)
	req.Header.Add("X-Plex-Device", p.Headers.Device)
	req.Header.Add("X-Plex-Token", p.Token)

	// optional headers
	if p.Headers.TargetClientIdentifier != "" {
		req.Header.Add("X-Plex-Target-Identifier", p.Headers.TargetClientIdentifier)
	}

	resp, err := client.Do(req)

	//if resp.Body != nil {
	//	//goland:noinspection GoUnhandledErrorResult
	//	defer resp.Body.Close()
	//
	//	bytes, err := io.ReadAll(resp.Body)
	//	if err != nil {
	//		return &http.Response{}, err
	//	}
	//
	//	log.Println(string(bytes))
	//
	//	resp.Body = io.NopCloser(strings.NewReader(string(bytes)))
	//}

	if err != nil {
		return &http.Response{}, err
	}

	return resp, nil
}

// joinArgs Returns a query string (uses for HTTP URLs) where only the value is URL encoded.
// Example return value: '?genre=action&type=1337'.
// Parameters:
// args (dict): Arguments to include in query string.
func joinArgs(args map[string]string) string {
	var argList []string
	for key, value := range args {
		argList = append(argList, fmt.Sprintf("%s=%s", key, url.QueryEscape(value)))
	}
	return "?" + strings.Join(argList, "&")
}

func (p *Server) EditMetadata(librarySectionID string, args map[string]string) error {
	query := fmt.Sprintf("%s/library/sections/%s/all%s", p.URL, librarySectionID, joinArgs(args))
	resp, err := p.http("PUT", query)
	if err != nil {
		return err
	}
	// Unauthorized
	if resp.StatusCode == http.StatusUnauthorized {
		return errors.New(plex.ErrorNotAuthorized)
	} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf(plex.ErrorServer, resp.Status)
	}
	return errors.New(resp.Status)
}

func (p *Server) SyncWatched(src plex.Metadata, dest plex.Metadata) {
	doUpdate := false
	updates := map[string]string{
		"id":   src.RatingKey,
		"type": src.Type,
	}
	viewCount, _ := src.ViewCount.Int64()
	srcViewCount, _ := dest.ViewCount.Int64()
	if (viewCount == 0) && (srcViewCount > 0) {
		updates["viewCount"] = dest.ViewCount.String()
		doUpdate = true
	}
	if dest.ViewOffset > 0 {
		updates["viewOffset"] = strconv.Itoa(dest.ViewOffset)
		doUpdate = true
	}
	if doUpdate {
		_ = p.EditMetadata(src.LibrarySectionKey, updates)
	}
}

func (p *Server) GetLibrarySectionByName(name string, filter string) (plex.SearchResults, string, error) {
	libraries, err := p.GetLibraries()
	if err != nil {
		return plex.SearchResults{}, "", err
	}
	key := ""
	libType := ""
	for _, library := range libraries.MediaContainer.Directory {
		if name == library.Title {
			key = library.Key
			libType = library.Type
		}
	}
	if key == "" {
		return plex.SearchResults{}, "", errors.New("library section not found")
	}
	content, err := p.GetLibraryContent(key, filter)
	return content, libType, err

}
